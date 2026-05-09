/*
Copyright 2021 The Kubernetes Authors.

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

package vsphereparavirtual

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

type instances struct {
	vmClient        vmop.Interface
	namespace       string
	clusterIPFamily string
}

// Compile-time assertion that instances implements cloudprovider.Instances.
var _ cloudprovider.Instances = &instances{}

const (
	// providerPrefix is the Kubernetes cloud provider prefix for this
	// cloud provider.
	providerPrefix = ProviderName + "://"

	// powerStateOff is the powered-off state constant from the hub types package.
	powerStateOff = vmoptypes.PowerStatePoweredOff
)

// DiscoverNodeBackoff is set to be the same with https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L83
var DiscoverNodeBackoff = wait.Backoff{
	Steps:    20,
	Duration: 50 * time.Millisecond,
	Jitter:   1.0,
}

var (
	errBiosUUIDEmpty = errors.New("discovered Bios UUID is empty")
)

// checkError reports whether err is non-nil. It is used as a predicate by the
// vmoperator informer machinery (see vmoperator.go). Prefer direct err != nil
// checks in all other call sites.
func checkError(err error) bool {
	return err != nil
}

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachineInfo if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (i instances) discoverNodeByProviderID(ctx context.Context, providerID string) (*vmoptypes.VirtualMachineInfo, error) {
	return discoverNodeByProviderID(ctx, providerID, i.namespace, i.vmClient)
}

// discoverNodeByName takes a node name and returns a VirtualMachineInfo if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (i instances) discoverNodeByName(ctx context.Context, name types.NodeName) (*vmoptypes.VirtualMachineInfo, error) {
	return discoverNodeByName(ctx, name, i.namespace, i.vmClient)
}

// NewInstances returns an implementation of cloudprovider.Instances.
// clusterIPFamily must be a canonical value from ParseClusterIPFamily:
// ClusterIPFamilyIPv4, ClusterIPFamilyIPv6, ClusterIPFamilyIPv4IPv6, or
// ClusterIPFamilyIPv6IPv4. It controls the ordering of NodeInternalIP addresses
// so the API server's first address matches the cluster's intended stack.
func NewInstances(clusterNS string, vmClient vmop.Interface, clusterIPFamily string) (cloudprovider.Instances, error) {
	return &instances{
		vmClient:        vmClient,
		namespace:       clusterNS,
		clusterIPFamily: clusterIPFamily,
	}, nil
}

// isLinkLocalIP reports whether the given IP string is a link-local address.
// Uses net.IP.IsLinkLocalUnicast() to correctly handle all RFC-4291 IPv6
// link-local addresses (fe80::/10) and RFC-3927 IPv4 link-local addresses
// (169.254.0.0/16) regardless of case or representation.
func isLinkLocalIP(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLinkLocalUnicast()
}

// createNodeAddresses builds the list of NodeAddresses for a VM.
// clusterIPFamily must be canonical (see ParseClusterIPFamily).
//
// Address family filtering rules:
//   - ipv4:      report IPv4 addresses only
//   - ipv6:      report IPv6 addresses only
//   - ipv4ipv6:  report both, IPv4 first
//   - ipv6ipv4:  report both, IPv6 first
func createNodeAddresses(vm *vmoptypes.VirtualMachineInfo, clusterIPFamily string) []v1.NodeAddress {
	if vm.PrimaryIP4 == "" && vm.PrimaryIP6 == "" && len(vm.NetworkInterfaceAddresses) == 0 {
		klog.V(4).Info("instance found, but no address yet")
		return []v1.NodeAddress{}
	}

	var nodeAddresses []v1.NodeAddress
	// addedIPs tracks normalized IP strings to deduplicate addresses that are
	// semantically equal but have different textual representations
	// (e.g. "2001:db8::1" and "2001:0db8:0000::0001").
	addedIPs := make(map[string]bool)
	normalizeIP := func(ip string) string {
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.String()
		}
		return ip
	}

	// wantIPv4 / wantIPv6 gate which address families are reported.
	// Single-stack families suppress the other family entirely; dual-stack
	// families report both, with ordering determined by the flag value.
	var wantIPv4, wantIPv6, ipv6First bool
	switch clusterIPFamily {
	case ClusterIPFamilyIPv4:
		wantIPv4 = true
	case ClusterIPFamilyIPv6:
		wantIPv6 = true
		ipv6First = true
	case ClusterIPFamilyIPv4IPv6:
		wantIPv4, wantIPv6 = true, true
	case ClusterIPFamilyIPv6IPv4:
		wantIPv4, wantIPv6 = true, true
		ipv6First = true
	default:
		// Invariant: callers must pass a canonical value returned from
		// ParseClusterIPFamily. Reaching this branch is a programming error.
		panic(fmt.Sprintf(
			"createNodeAddresses: invariant violated: clusterIPFamily %q is not canonical "+
				"(must be one of %q/%q/%q/%q from ParseClusterIPFamily)",
			clusterIPFamily,
			ClusterIPFamilyIPv4, ClusterIPFamilyIPv6, ClusterIPFamilyIPv4IPv6, ClusterIPFamilyIPv6IPv4,
		))
	}

	appendIP := func(ip string) {
		nodeAddresses = append(nodeAddresses, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: ip,
		})
		addedIPs[normalizeIP(ip)] = true
	}

	// Append primary IPs in cluster-preference order.
	if ipv6First {
		if wantIPv6 && vm.PrimaryIP6 != "" {
			appendIP(vm.PrimaryIP6)
		}
		if wantIPv4 && vm.PrimaryIP4 != "" {
			appendIP(vm.PrimaryIP4)
		}
	} else {
		if wantIPv4 && vm.PrimaryIP4 != "" {
			appendIP(vm.PrimaryIP4)
		}
		if wantIPv6 && vm.PrimaryIP6 != "" {
			appendIP(vm.PrimaryIP6)
		}
	}

	// Append additional interface addresses, filtered by family and deduped.
	for _, ip := range vm.NetworkInterfaceAddresses {
		if addedIPs[normalizeIP(ip)] {
			continue
		}
		if isLinkLocalIP(ip) {
			klog.V(6).Infof("Skipping link-local address: %s", ip)
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil {
			continue
		}
		isIPv4 := parsed.To4() != nil
		if (isIPv4 && !wantIPv4) || (!isIPv4 && !wantIPv6) {
			continue
		}
		nodeAddresses = append(nodeAddresses, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: ip,
		})
		addedIPs[normalizeIP(ip)] = true
	}

	// NodeHostName is intentionally left empty: in paravirtual mode kubelet sets
	// the node hostname via --hostname-override or the OS hostname. The cloud
	// provider does not know the in-guest hostname and must not override it.
	nodeAddresses = append(nodeAddresses, v1.NodeAddress{
		Type:    v1.NodeHostName,
		Address: "",
	})

	if len(nodeAddresses) > 1 {
		klog.V(2).Infof("VM %s: Reporting %d IP address(es) to API server", vm.Name, len(nodeAddresses)-1)
	} else {
		klog.Warningf("VM %s: No IP addresses found in network status", vm.Name)
	}

	return nodeAddresses
}

// NodeAddresses returns the addresses of the specified instance if one exists, otherwise nil
// If the instance exists but does not yet have an IP address, the function returns a zero length slice
func (i *instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddresses() called with ", name)

	vm, err := i.discoverNodeByName(ctx, name)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return nil, err
	}
	if vm == nil {
		klog.V(4).Info("instances.NodeAddresses() InstanceNotFound ", name)
		return nil, cloudprovider.InstanceNotFound
	}
	return createNodeAddresses(vm, i.clusterIPFamily), err
}

// NodeAddressesByProviderID returns the addresses of the specified instance if one exists, otherwise nil
// If the instance exists but does not yet have an IP address, the function returns a zero length slice
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddressesByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return nil, err
	}
	if vm == nil {
		klog.V(4).Info("instances.NodeAddressesByProviderID() InstanceNotFound ", providerID)
		return nil, cloudprovider.InstanceNotFound
	}
	return createNodeAddresses(vm, i.clusterIPFamily), nil
}

// InstanceID returns the cloud provider ID of the named instance if one exists, otherwise an empty string
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	vm, err := i.discoverNodeByName(ctx, nodeName)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return "", err
	}
	if vm == nil {
		klog.V(4).Info("instances.InstanceID() InstanceNotFound ", nodeName)
		return "", cloudprovider.InstanceNotFound
	}

	if vm.BiosUUID == "" {
		return "", errBiosUUIDEmpty
	}

	klog.V(4).Infof("instances.InstanceID() called to get vm: %v uuid: %v", nodeName, vm.BiosUUID)
	return vm.BiosUUID, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.V(4).Info("instances.InstanceType() called with ", name)
	return "", nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.V(4).Info("instances.InstanceTypeByProviderID() called with ", providerID)
	return "", nil
}

// CurrentNodeName returns the name of the node we are currently running on
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	klog.V(4).Info("instances.CurrentNodeName() called with ", hostname)
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceExistsByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return false, err
	}
	return vm != nil, nil
}

// InstanceShutdownByProviderID returns true if the instance exists and is shut down
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceShutdownByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return false, err
	}
	if vm == nil {
		klog.V(4).Info("instances.InstanceShutdownByProviderID() InstanceNotFound ", providerID)
		return false, cloudprovider.InstanceNotFound
	}
	return vm.PowerState == powerStateOff, nil
}

func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	klog.V(4).Info("instances.AddSSHKeyToAllInstances() called")
	return cloudprovider.NotImplemented
}

// GetUUIDFromProviderID returns a UUID from the supplied cloud provider ID.
func GetUUIDFromProviderID(providerID string) string {
	withoutPrefix := strings.TrimPrefix(providerID, providerPrefix)
	return strings.ToLower(strings.TrimSpace(withoutPrefix))
}
