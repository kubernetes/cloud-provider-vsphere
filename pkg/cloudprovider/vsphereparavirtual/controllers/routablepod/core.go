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

package routablepod

import (
	"context"
	"fmt"

	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/ipaddressallocation"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/ippool"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/node"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ipfamily"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/helper"
	ippmv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/v1alpha1"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/nsxipmanager"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

// Options collects the configuration that StartControllers needs. Grouping
// these into a struct keeps the function signature stable as new flags are
// added and prevents the boolean-parameter anti-pattern at the call site.
type Options struct {
	// ClusterName is the guest cluster name; required.
	ClusterName string
	// ClusterNS is the supervisor namespace for the guest cluster; required.
	ClusterNS string
	// OwnerRef is stamped onto every CR the controllers create.
	OwnerRef *metav1.OwnerReference
	// VPCModeEnabled selects between VPC (IPAddressAllocation+StaticRoute) and
	// T1 (IPPool+RouteSet) backing CRs. IPv6 requires VPC mode.
	VPCModeEnabled bool
	// PodIPPoolType is "Public" or "Private" and controls IPv4 visibility.
	PodIPPoolType string
	// IPFamily is the resolved --cluster-ip-family value. Valid values are
	// ipfamily.IPv4, ipfamily.IPv6, ipfamily.IPv4IPv6, and ipfamily.IPv6IPv4.
	// It encodes which address families are active and which is primary, so
	// no separate boolean fields are needed.
	IPFamily ipfamily.IPFamily
}

// StartControllers starts the Routable Pods controllers: in VPC mode it starts
// ipaddressallocation_controller (which patches node PodCIDRs) and
// node_controller (which manages IPAddressAllocation CRs); in T1 mode it
// starts ippool_controller and node_controller.
func StartControllers(scCfg *rest.Config, client kubernetes.Interface,
	informerManager *k8s.InformerManager, opts Options) error {

	if opts.ClusterName == "" {
		return fmt.Errorf("cluster name can't be empty")
	}
	if opts.ClusterNS == "" {
		return fmt.Errorf("cluster namespace can't be empty")
	}

	klog.V(2).Info("Routable pod controllers start with VPC mode enabled: ", opts.VPCModeEnabled)

	ctx := informerManager.GetContext()

	var nsxIPManager nsxipmanager.NSXIPManager
	if opts.VPCModeEnabled {
		nsxClient, nsxInformerFactory, err := getNSXClientAndInformer(scCfg, opts.ClusterNS)
		if err != nil {
			return fmt.Errorf("fail to get NSX client or informer factory: %w", err)
		}

		startIPAddressAllocationController(ctx, client, informerManager, nsxInformerFactory, opts.IPFamily, opts.ClusterName)

		nsxIPManager = nsxipmanager.NewNSXVPCIPManager(nsxClient, nsxInformerFactory, opts.ClusterName, opts.ClusterNS, opts.PodIPPoolType, opts.OwnerRef,
			opts.IPFamily.IPv4Enabled(), opts.IPFamily.IPv6Enabled())
	} else {
		ippManager, err := ippmv1alpha1.NewIPPoolManager(scCfg, opts.ClusterNS)
		if err != nil {
			return fmt.Errorf("fail to get ippool manager or start ippool controller: %w", err)
		}

		startIPPoolController(ctx, client, ippManager)

		nsxIPManager = nsxipmanager.NewNSXT1IPManager(ippManager, opts.ClusterName, opts.ClusterNS, opts.OwnerRef)
	}

	nodeController := node.NewController(client, nsxIPManager, informerManager, opts.ClusterName, opts.ClusterNS, opts.OwnerRef, opts.IPFamily.FamilyCount())
	go nodeController.Run(ctx.Done())

	return nil
}

func startIPAddressAllocationController(ctx context.Context, client kubernetes.Interface, informerManager *k8s.InformerManager, nsxInformerFactory nsxinformers.SharedInformerFactory, f ipfamily.IPFamily, clusterName string) {
	ipAddressAllocationController := ipaddressallocation.NewController(
		ctx,
		client,
		informerManager.GetNodeLister(),
		informerManager.IsNodeInformerSynced(),
		nsxInformerFactory.Crd().V1alpha1().IPAddressAllocations(),
		f,
		clusterName)
	go ipAddressAllocationController.Run(ctx, 1)
	nsxInformerFactory.Start(ctx.Done())
}

func startIPPoolController(ctx context.Context, client kubernetes.Interface, ippManager *ippmv1alpha1.IPPoolManager) {
	ippoolController := ippool.NewController(client, ippManager)
	go ippoolController.Run(ctx.Done())
	ippManager.StartIPPoolInformers(ctx.Done())
}

func getNSXClientAndInformer(svCfg *rest.Config, svNamespace string) (nsxclients.Interface, nsxinformers.SharedInformerFactory, error) {
	client, err := nsxclients.NewForConfig(svCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("error building nsx-operator clientset: %w", err)
	}

	informerFactory := nsxinformers.NewSharedInformerFactoryWithOptions(client,
		helper.DefaultResyncTime, nsxinformers.WithNamespace(svNamespace))

	return client, informerFactory, nil
}
