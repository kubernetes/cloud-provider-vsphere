package nsxipmanager

import (
	"context"

	nsxapisv1alpha1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/nsx.vmware.com/v1alpha1"
	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

const allocationSize = 24

var _ NSXIPManager = &NSXVPCIPManager{}

// NSXVPCIPManager is an implementation of NSXIPManager for NSX-T VPC.
type NSXVPCIPManager struct {
	client          nsxclients.Interface
	informerFactory nsxinformers.SharedInformerFactory
	svNamespace     string
	ownerRef        *metav1.OwnerReference
	podIPPoolType   string
}

// ClaimPodCIDR will claim pod cidr for the node by creating IPAddressAllocation CR.
func (m *NSXVPCIPManager) ClaimPodCIDR(node *corev1.Node) error {
	if _, err := m.informerFactory.Nsx().V1alpha1().IPAddressAllocations().Lister().IPAddressAllocations(m.svNamespace).Get(node.Name); err != nil {
		if apierrors.IsNotFound(err) {
			// Keep the same behavior as NSX T1: add Pod CIDR allocation req only when node doesn't contain pod cidr
			if node.Spec.PodCIDR == "" || len(node.Spec.PodCIDRs) == 0 {
				ipAddressAllocation := &nsxapisv1alpha1.IPAddressAllocation{
					ObjectMeta: metav1.ObjectMeta{
						Name:      node.Name,
						Namespace: m.svNamespace,
						OwnerReferences: []metav1.OwnerReference{
							*m.ownerRef,
						},
					},
					Spec: nsxapisv1alpha1.IPAddressAllocationSpec{
						IPAddressBlockVisibility: convertToIPAddressVisibility(m.podIPPoolType),
						AllocationSize:           allocationSize,
					},
				}
				if _, err := m.client.NsxV1alpha1().IPAddressAllocations(m.svNamespace).Create(context.Background(), ipAddressAllocation, metav1.CreateOptions{}); err != nil {
					return err
				}
			} else {
				klog.V(4).Infof("Pod CIDR %s is already set to node %s", node.Spec.PodCIDR, node.Name)
			}
		} else {
			return err
		}
	} else {
		klog.V(4).Infof("Node %s already requested IPAddressAllocations", node.Name)
	}
	return nil
}

// ReleasePodCIDR will release pod cidr for the node by deleting IPAddressAllocation CR.
func (m *NSXVPCIPManager) ReleasePodCIDR(node *corev1.Node) error {
	if _, err := m.informerFactory.Nsx().V1alpha1().IPAddressAllocations().Lister().IPAddressAllocations(m.svNamespace).Get(node.Name); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Infof("IPAddressAllocations %s not found, no need to delete it", node.Name)
		} else {
			return err
		}
	} else {
		if err := m.client.NsxV1alpha1().IPAddressAllocations(m.svNamespace).Delete(context.Background(), node.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// convertToIPAddressVisibility converts the ip pool type to the ip address visibility. This is needed because the nsx
// does not unify names yet. Public equals to External. TODO: remove this once nsx unified names.
func convertToIPAddressVisibility(ipPoolType string) nsxapisv1alpha1.IPAddressVisibility {
	if ipPoolType == PublicIPPoolType {
		return nsxapisv1alpha1.IPAddressVisibilityExternal
	}
	return nsxapisv1alpha1.IPAddressVisibilityPrivate
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
