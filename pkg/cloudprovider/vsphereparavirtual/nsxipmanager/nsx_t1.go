package nsxipmanager

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager"
)

var _ NSXIPManager = &NSXT1IPManager{}

// NSXT1IPManager is an implementation of NSXIPManager for NSX-T T1.
type NSXT1IPManager struct {
	ippoolManager ippoolmanager.IPPoolManager
	clusterName   string
	svNamespace   string
	ownerRef      *metav1.OwnerReference
}

// ClaimPodCIDR will claim pod cidr for the node by adding subnet to IPPool.
func (m *NSXT1IPManager) ClaimPodCIDR(node *corev1.Node) error {
	ippool, err := m.ippoolManager.GetIPPool(m.svNamespace, m.clusterName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("fail to get ippool in namespace %s for cluster %s", m.svNamespace, m.clusterName)
		}
		// if ippool does not exist, create one
		klog.V(4).Info("creating ippool")
		if ippool, err = m.ippoolManager.CreateIPPool(m.svNamespace, m.clusterName, m.ownerRef); err != nil {
			klog.Error("error creating ippool")
			return err
		}
	}

	err = m.ippoolManager.AddSubnetToIPPool(node, ippool, m.ownerRef)
	if err != nil {
		return fmt.Errorf("fail to add subnet in IPPool for node %s, err: %v", node.Name, err)
	}

	klog.V(4).Infof("added the subnet in IPPool for node %s", node.Name)
	return nil
}

// ReleasePodCIDR will release pod cidr for the node by deleting subnet from IPPool.
func (m *NSXT1IPManager) ReleasePodCIDR(node *corev1.Node) error {
	ippool, err := m.ippoolManager.GetIPPool(m.svNamespace, m.clusterName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Info("ippool is gone, no need to remove the node request")
			return nil
		}
		return fmt.Errorf("fail to get ippool in namespace %s for cluster %s", m.svNamespace, m.clusterName)
	}

	err = m.ippoolManager.DeleteSubnetFromIPPool(node.Name, ippool)
	if err != nil {
		return fmt.Errorf("fail to delete subnet in IPPool for node %s, err: %v", node.Name, err)
	}

	klog.V(4).Infof("deleted the subnet in IPPool for node %s", node.Name)
	return nil
}

// NewNSXT1IPManager creates a new NSXT1IPManager object.
func NewNSXT1IPManager(ippoolManager ippoolmanager.IPPoolManager, clusterName, svNamespace string, ownerRef *metav1.OwnerReference) NSXIPManager {
	return &NSXT1IPManager{
		ippoolManager: ippoolManager,
		clusterName:   clusterName,
		svNamespace:   svNamespace,
		ownerRef:      ownerRef,
	}
}
