package nsxipmanager

import (
	"context"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const allocationSize = 256

var _ NSXIPManager = &NSXVPCIPManager{}

// NSXVPCIPManager is an implementation of NSXIPManager for NSX-T VPC.
type NSXVPCIPManager struct {
	client          nsxclients.Interface
	informerFactory nsxinformers.SharedInformerFactory
	svNamespace     string
	ownerRef        *metav1.OwnerReference
	podIPPoolType   string
}

func (m *NSXVPCIPManager) createIPAddressAllocation(name string) error {
	klog.V(4).Infof("Creating IPAddressAllocation CR %s/%s", m.svNamespace, name)
	ipAddressAllocation := &vpcapisv1.IPAddressAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.svNamespace,
			OwnerReferences: []metav1.OwnerReference{
				*m.ownerRef,
			},
		},
		Spec: vpcapisv1.IPAddressAllocationSpec{
			IPAddressBlockVisibility: convertToIPAddressVisibility(m.podIPPoolType),
			AllocationSize:           allocationSize,
		},
	}
	_, err := m.client.CrdV1alpha1().IPAddressAllocations(m.svNamespace).Create(context.Background(), ipAddressAllocation, metav1.CreateOptions{})
	return err
}

// ClaimPodCIDR will claim pod cidr for the node by creating IPAddressAllocation CR.
func (m *NSXVPCIPManager) ClaimPodCIDR(node *corev1.Node) error {
	// Keep the same behavior as NSX T1: add Pod CIDR allocation req only when node doesn't contain pod cidr
	if node.Spec.PodCIDR != "" && len(node.Spec.PodCIDRs) > 0 {
		klog.V(4).Infof("Pod CIDR %s is already set to node %s", node.Spec.PodCIDR, node.Name)
		return nil
	}

	if _, err := m.informerFactory.Crd().V1alpha1().IPAddressAllocations().Lister().IPAddressAllocations(m.svNamespace).Get(node.Name); err != nil {
		if apierrors.IsNotFound(err) {
			return m.createIPAddressAllocation(node.Name)
		}
		return err
	}

	klog.V(4).Infof("Node %s already requested IPAddressAllocations", node.Name)
	return nil
}

// ReleasePodCIDR will release pod cidr for the node by deleting IPAddressAllocation CR.
func (m *NSXVPCIPManager) ReleasePodCIDR(node *corev1.Node) error {
	if _, err := m.informerFactory.Crd().V1alpha1().IPAddressAllocations().Lister().IPAddressAllocations(m.svNamespace).Get(node.Name); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("IPAddressAllocations %s not found, no need to delete it", node.Name)
			return nil
		}
		return err
	}

	return m.client.CrdV1alpha1().IPAddressAllocations(m.svNamespace).Delete(context.Background(), node.Name, metav1.DeleteOptions{})
}

// convertToIPAddressVisibility converts the ip pool type to the ip address visibility. This is needed because the nsx
// does not unify names yet. Public equals to External.
func convertToIPAddressVisibility(ipPoolType string) vpcapisv1.IPAddressVisibility {
	if ipPoolType == PublicIPPoolType {
		return "External"
	}
	return "Private"
}

// NewNSXVPCIPManager returns a new NSXVPCIPManager object.
func NewNSXVPCIPManager(client nsxclients.Interface, informerFactory nsxinformers.SharedInformerFactory, svNamespace, podIPPoolType string, ownerRef *metav1.OwnerReference) NSXIPManager {
	return &NSXVPCIPManager{
		client:          client,
		informerFactory: informerFactory,
		svNamespace:     svNamespace,
		ownerRef:        ownerRef,
		podIPPoolType:   podIPPoolType,
	}
}
