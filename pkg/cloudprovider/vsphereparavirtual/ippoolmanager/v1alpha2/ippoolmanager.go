package v1alpha2

import (
	"context"
	"fmt"
	"reflect"

	vpcnetworkingapis "github.com/vmware-tanzu/nsx-operator/pkg/apis/nsx.vmware.com/v1alpha2"
	vpcnetworkingclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	vpcnetworkinginformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/helper"
)

// IPPoolManager defines an ippool manager working with v1alpha2 ippool CR
type IPPoolManager struct {
	clients         vpcnetworkingclients.Interface
	informerFactory vpcnetworkinginformers.SharedInformerFactory
	podIPPoolType   string
}

// NewIPPoolManager  initializes a IPPoolManager
func NewIPPoolManager(config *rest.Config, clusterNS string, podIPPoolType string) (*IPPoolManager, error) {
	ippoolclients, err := vpcnetworkingclients.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("error building ippool ippoolclientset: %w", err)
	}

	ippoolInformerFactory := vpcnetworkinginformers.NewSharedInformerFactoryWithOptions(ippoolclients,
		helper.DefaultResyncTime, vpcnetworkinginformers.WithNamespace(clusterNS))

	return &IPPoolManager{
		clients:         ippoolclients,
		informerFactory: ippoolInformerFactory,
		podIPPoolType:   podIPPoolType,
	}, nil
}

// NewIPPoolManagerWithClients  initializes a IPPoolManager with clientset
func NewIPPoolManagerWithClients(clients vpcnetworkingclients.Interface, clusterNS string) (*IPPoolManager, error) {
	ippoolInformerFactory := vpcnetworkinginformers.NewSharedInformerFactoryWithOptions(clients,
		helper.DefaultResyncTime, vpcnetworkinginformers.WithNamespace(clusterNS))

	return &IPPoolManager{
		clients:         clients,
		informerFactory: ippoolInformerFactory,
	}, nil
}

// GetIPPool gets the ippool CR from a namespace, belonged to a workload cluster
func (p *IPPoolManager) GetIPPool(clusterNS, clusterName string) (helper.NSXIPPool, error) {
	return p.clients.NsxV1alpha2().IPPools(clusterNS).Get(context.Background(), helper.IppoolNameFromClusterName(clusterName), metav1.GetOptions{})
}

// GetIPPoolFromIndexer gets an ippool CR from cache store
func (p *IPPoolManager) GetIPPoolFromIndexer(key string) (helper.NSXIPPool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return nil, err
	}

	return p.informerFactory.Nsx().V1alpha2().IPPools().Lister().IPPools(namespace).Get(name)
}

// UpdateIPPool updates a kubernetes ippool
func (p *IPPoolManager) UpdateIPPool(ippool *vpcnetworkingapis.IPPool) (*vpcnetworkingapis.IPPool, error) {
	return p.clients.NsxV1alpha2().IPPools(ippool.Namespace).Update(context.Background(), ippool, metav1.UpdateOptions{})
}

// CreateIPPool creates an ippool CR to a namespace, for a workload cluster
func (p *IPPoolManager) CreateIPPool(clusterNS, clusterName string, ownerRef *metav1.OwnerReference) (helper.NSXIPPool, error) {
	ippool := &vpcnetworkingapis.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.IppoolNameFromClusterName(clusterName),
			Namespace: clusterNS,
			OwnerReferences: []metav1.OwnerReference{
				*ownerRef,
			},
		},
		// Omit to provide IPPool type. nsx operator will update the type afterwards.
		Spec: vpcnetworkingapis.IPPoolSpec{
			Type:    p.podIPPoolType,
			Subnets: []vpcnetworkingapis.SubnetRequest{},
		},
	}

	return p.clients.NsxV1alpha2().IPPools(clusterNS).Create(context.Background(), ippool, metav1.CreateOptions{})
}

// GetIPPoolSubnets gets the subnets from realized status in ippool CR
func (p *IPPoolManager) GetIPPoolSubnets(ippool helper.NSXIPPool) (map[string]string, error) {
	ipp, ok := ippool.(*vpcnetworkingapis.IPPool)
	if !ok {
		return nil, fmt.Errorf("unknown ippool type")
	}

	subs := make(map[string]string)
	for _, sub := range ipp.Status.Subnets {
		subs[sub.Name] = sub.CIDR
	}

	return subs, nil
}

// DeleteSubnetFromIPPool removes the subnet for specific node from ippool CR
func (p *IPPoolManager) DeleteSubnetFromIPPool(subnetName string, ippool helper.NSXIPPool) error {
	ipp, ok := ippool.(*vpcnetworkingapis.IPPool)
	if !ok {
		return fmt.Errorf("unknown ippool type")
	}

	newSubnets := []vpcnetworkingapis.SubnetRequest{}
	for _, sub := range ipp.Spec.Subnets {
		if sub.Name == subnetName {
			continue
		}
		newSubnets = append(newSubnets, sub)
	}
	ipp.Spec.Subnets = newSubnets

	_, err := p.UpdateIPPool(ipp)
	if err != nil {
		return fmt.Errorf("fail to update ippool %s in namespace %s with err: %w", ipp.Name, ipp.Namespace, err)
	}

	return nil
}

// AddSubnetToIPPool adds the subnet for specific node to ippool CR. The subnet name is the node name
func (p *IPPoolManager) AddSubnetToIPPool(node *corev1.Node, ippool helper.NSXIPPool, ownerRef *metav1.OwnerReference) error {
	ipp, ok := ippool.(*vpcnetworkingapis.IPPool)
	if !ok {
		return fmt.Errorf("unknow ippool struct")
	}

	// skip if the request already added
	for _, sub := range ipp.Spec.Subnets {
		if sub.Name == node.Name {
			klog.V(4).Infof("node %s already requested the subnet", node.Name)
			return nil
		}
	}

	newIpp := ipp.DeepCopy()
	// add node cidr allocation req to the ippool spec only when node doesn't contain pod cidr
	if node.Spec.PodCIDR == "" || len(node.Spec.PodCIDRs) == 0 {
		klog.V(4).Infof("add subnet to ippool for node %s", node.Name)
		newIpp.Spec.Subnets = append(newIpp.Spec.Subnets, vpcnetworkingapis.SubnetRequest{
			Name:         node.Name,
			IPFamily:     helper.IPFamilyDefaultV2,
			PrefixLength: helper.PrefixLengthDefault,
		})
	}

	if newIpp.OwnerReferences == nil {
		newIpp.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}

	_, err := p.UpdateIPPool(newIpp)
	if err != nil {
		return fmt.Errorf("fail to update ippool %s in namespace %s with err: %w", ipp.Name, ipp.Namespace, err)
	}

	return nil
}

// DiffIPPoolSubnets validates if subnets of status in ippool CR changes
func (p *IPPoolManager) DiffIPPoolSubnets(old, cur helper.NSXIPPool) bool {
	oldIPPool, ok := old.(*vpcnetworkingapis.IPPool)
	if !ok {
		return false
	}
	curIPPool, ok := cur.(*vpcnetworkingapis.IPPool)
	if !ok {
		return false
	}
	// If they are equal, then there is no diff (return false), otherwise return true
	return !reflect.DeepEqual(oldIPPool.Status.Subnets, curIPPool.Status.Subnets)
}

// GetIPPoolInformer gets ippool informer
func (p *IPPoolManager) GetIPPoolInformer() cache.SharedIndexInformer {
	return p.informerFactory.Nsx().V1alpha2().IPPools().Informer()
}

// GetIPPoolListerSynced gets HasSynced function
func (p *IPPoolManager) GetIPPoolListerSynced() cache.InformerSynced {
	return p.informerFactory.Nsx().V1alpha2().IPPools().Informer().HasSynced
}

// StartIPPoolInformers starts ippool informer
func (p *IPPoolManager) StartIPPoolInformers() {
	p.informerFactory.Start(wait.NeverStop)
}
