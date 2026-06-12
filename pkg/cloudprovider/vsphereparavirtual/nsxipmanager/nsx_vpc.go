/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nsxipmanager

import (
	"context"
	"errors"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
)

const (
	// allocationSize is the number of IPv4 addresses requested per node. NSX
	// requires this to be a power of two; 256 (a /24 worth of addresses) is the
	// default chosen to match the historic single-family behaviour.
	allocationSize = 256
	// ipv6AllocationPrefixLength is the IPv6 prefix length requested per node.
	// /64 is the standard architectural boundary for IPv6: it is the smallest
	// prefix that supports SLAAC and is the universally expected subnet size
	// for a single link or node. Each node receives one /64 carved from the
	// supervisor's IPv6 block.
	ipv6AllocationPrefixLength = 64
)

var _ NSXIPManager = &NSXVPCIPManager{}

// NSXVPCIPManager is an implementation of NSXIPManager for NSX-T VPC.
type NSXVPCIPManager struct {
	client          nsxclients.Interface
	informerFactory nsxinformers.SharedInformerFactory
	svNamespace     string
	ownerRef        *metav1.OwnerReference
	podIPPoolType   string
	ipv4Enabled     bool
	ipv6Enabled     bool
}

// crNameForFamily returns the CR name for a given node and IP family.
// IPv4 keeps the bare node name (no rename for existing clusters).
// IPv6 always carries the helper.SuffixIPv6 ("-ipv6") suffix.
//
// Note: this is a *naming* convention only. The authoritative family for any
// existing CR is its Spec.IPAddressType; the authoritative node binding is
// the helper.LabelKeyNodeName label set in createIPAddressAllocation. Callers
// should not try to invert this function by parsing a CR name to recover
// either the node or the family, because node names that themselves end in
// "-ipv6" would be misclassified.
func crNameForFamily(nodeName string, ipv4 bool) string {
	if ipv4 {
		return nodeName
	}
	return nodeName + helper.SuffixIPv6
}

// createIPAddressAllocation creates one IPAddressAllocation CR for the given
// node and IP family.
//
// For IPv4 it sets IPAddressType=IPv4, IPAddressBlockVisibility derived from
// podIPPoolType (Public→External, anything else→Private), and allocationSize.
//
// For IPv6 it sets IPAddressType=IPv6 and allocationPrefixLength=64 (the
// standard architectural boundary for IPv6). IPAddressBlockVisibility is left
// unset for IPv6: the VPC's IPv6 block visibility is configured out-of-band
// on the VPC itself. Operators who set --pod-ip-pool-type=Public should be
// aware that in dual-stack mode it only influences the IPv4 CR.
//
// All CRs carry helper.LabelKeyNodeName and helper.LabelKeyIPFamily labels
// so that downstream consumers (e.g. ipaddressallocation_controller) can
// recover the bound node and family without having to parse the CR name.
func (m *NSXVPCIPManager) createIPAddressAllocation(ctx context.Context, nodeName string, ipv4 bool) error {
	crName := crNameForFamily(nodeName, ipv4)
	klog.V(4).Infof("Creating IPAddressAllocation %s/%s (ipv4=%v)", m.svNamespace, crName, ipv4)

	spec := vpcapisv1.IPAddressAllocationSpec{}
	familyLabel := helper.LabelValueIPFamilyIPv6
	if ipv4 {
		spec.IPAddressType = vpcapisv1.IPAllocationIPAddressTypeIPv4
		spec.IPAddressBlockVisibility = convertToIPAddressVisibility(m.podIPPoolType)
		spec.AllocationSize = allocationSize
		familyLabel = helper.LabelValueIPFamilyIPv4
	} else {
		spec.IPAddressType = vpcapisv1.IPAllocationIPAddressTypeIPv6
		spec.IPv6AllocationPrefixLength = ipv6AllocationPrefixLength
	}

	ipAddressAllocation := &vpcapisv1.IPAddressAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: m.svNamespace,
			Labels: map[string]string{
				helper.LabelKeyNodeName: nodeName,
				helper.LabelKeyIPFamily: familyLabel,
			},
			OwnerReferences: []metav1.OwnerReference{
				*m.ownerRef,
			},
		},
		Spec: spec,
	}
	_, err := m.client.CrdV1alpha1().IPAddressAllocations(m.svNamespace).Create(ctx, ipAddressAllocation, metav1.CreateOptions{})
	return err
}

// enabledFamilies returns the list of (isIPv4) flags for the families that
// should be reconciled, ordered IPv4 first then IPv6 to keep test fixtures
// stable.
func (m *NSXVPCIPManager) enabledFamilies() []bool {
	families := make([]bool, 0, 2)
	if m.ipv4Enabled {
		families = append(families, true)
	}
	if m.ipv6Enabled {
		families = append(families, false)
	}
	return families
}

// ClaimPodCIDR claims pod CIDR(s) for the node by creating IPAddressAllocation
// CRs (one per enabled family). It is idempotent: creating an already-present
// CR is treated as a no-op via a lister Get prior to Create.
func (m *NSXVPCIPManager) ClaimPodCIDR(node *corev1.Node) error {
	families := m.enabledFamilies()

	// No families enabled — nothing to do.
	if len(families) == 0 {
		return nil
	}

	// Fast-path: if the node already has a CIDR for every expected family we
	// can skip the lister Get + Create calls entirely. This is best-effort;
	// the per-family loop below is idempotent if the count ever disagrees.
	if len(node.Spec.PodCIDRs) >= len(families) {
		klog.V(4).Infof("Node %s already has %d pod CIDR(s), skipping claim", node.Name, len(node.Spec.PodCIDRs))
		return nil
	}

	lister := m.informerFactory.Crd().V1alpha1().IPAddressAllocations().Lister().IPAddressAllocations(m.svNamespace)

	// TODO(danqing.hou): thread context from NSXIPManager.ClaimPodCIDR once the
	// interface is refactored to accept a context parameter.
	ctx := context.TODO()

	for _, ipv4 := range families {
		crName := crNameForFamily(node.Name, ipv4)
		if _, err := lister.Get(crName); err == nil {
			klog.V(4).Infof("Node %s IPAddressAllocation %s (ipv4=%v) already exists", node.Name, crName, ipv4)
			continue
		} else if !apierrors.IsNotFound(err) {
			return err
		}
		if err := m.createIPAddressAllocation(ctx, node.Name, ipv4); err != nil {
			return err
		}
	}
	return nil
}

// ReleasePodCIDR deletes all per-family IPAddressAllocation CRs for the node.
// NotFound is treated as success (idempotent delete). On partial failure the
// other families are still attempted, and all errors are aggregated and
// returned so the caller (workqueue) can retry; this guarantees a partially
// realised dual-stack node does not leave one family's CR orphaned just
// because the other family's delete returned a transient error.
func (m *NSXVPCIPManager) ReleasePodCIDR(node *corev1.Node) error {
	// TODO(danqing.hou): thread context from NSXIPManager.ReleasePodCIDR once the
	// interface is refactored to accept a context parameter.
	ctx := context.TODO()
	var errs []error
	for _, ipv4 := range m.enabledFamilies() {
		if err := m.deleteIPAddressAllocation(ctx, node.Name, ipv4); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// deleteIPAddressAllocation deletes the IPAddressAllocation CR for the given node and
// IP family. NotFound is treated as success (idempotent delete).
func (m *NSXVPCIPManager) deleteIPAddressAllocation(ctx context.Context, nodeName string, ipv4 bool) error {
	crName := crNameForFamily(nodeName, ipv4)
	klog.V(4).Infof("Deleting IPAddressAllocation %s/%s", m.svNamespace, crName)
	if err := m.client.CrdV1alpha1().IPAddressAllocations(m.svNamespace).Delete(ctx, crName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		klog.V(4).Infof("IPAddressAllocation %s/%s not found, treating as already deleted", m.svNamespace, crName)
	}
	return nil
}

// convertToIPAddressVisibility converts the ip pool type to the ip address visibility. This is needed because the nsx
// does not unify names yet. Public equals to External.
func convertToIPAddressVisibility(ipPoolType string) vpcapisv1.IPAddressVisibility {
	if ipPoolType == PublicIPPoolType {
		return vpcapisv1.IPAddressVisibilityExternal
	}
	return vpcapisv1.IPAddressVisibilityPrivate
}

// NewNSXVPCIPManager returns an NSXIPManager that manages IPAddressAllocation CRs in VPC mode.
// ipv4Enabled and ipv6Enabled control which per-family CRs are created or deleted.
func NewNSXVPCIPManager(client nsxclients.Interface, informerFactory nsxinformers.SharedInformerFactory, svNamespace, podIPPoolType string, ownerRef *metav1.OwnerReference, ipv4Enabled, ipv6Enabled bool) NSXIPManager {
	return &NSXVPCIPManager{
		client:          client,
		informerFactory: informerFactory,
		svNamespace:     svNamespace,
		ownerRef:        ownerRef,
		podIPPoolType:   podIPPoolType,
		ipv4Enabled:     ipv4Enabled,
		ipv6Enabled:     ipv6Enabled,
	}
}
