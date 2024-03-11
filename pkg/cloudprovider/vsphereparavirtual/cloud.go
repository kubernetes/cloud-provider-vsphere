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
	"errors"
	"flag"
	"fmt"
	"io"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
	cpcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

const (
	// RegisteredProviderName is the name of the cloud provider registered with
	// Kubernetes.
	RegisteredProviderName string = "vsphere-paravirtual"

	// ProviderName is the name used for constructing Provider ID
	ProviderName string = "vsphere"

	clientName string = "vsphere-paravirtual-cloud-controller-manager"

	// CloudControllerManagerNS is the namespace for vsphere paravirtual cluster cloud provider
	CloudControllerManagerNS = "vmware-system-cloud-provider"

	// PublicIPPoolType allows Pod IP address routable outside of Tier 0 router.
	PublicIPPoolType = "Public"

	// PrivateIPPoolType allows Pod IP address routable within VPC router.
	PrivateIPPoolType = "Private"
)

var (
	// SupervisorClusterSecret is the name of vsphere paravirtual supervisor cluster cloud provider secret
	SupervisorClusterSecret = "cloud-provider-creds"

	// ClusterName contains the cluster-name flag injected from main, needed for cleanup
	ClusterName string

	// RouteEnabled if set to true, will start ippool and node controller.
	RouteEnabled bool

	// vpcModeEnabled if set to true, ippool and node controller will process v1alpha1 StaticRoute and v1alpha2 IPPool, otherwise v1alpha1 RouteSet and v1alpha1 IPPool
	vpcModeEnabled bool

	// podIPPoolType specify if Pod IP addresses is public or private.
	podIPPoolType string
)

func init() {
	cloudprovider.RegisterCloudProvider(RegisteredProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		if config == nil {
			return nil, errors.New("no vSphere paravirtual cloud provider config file given")
		}

		// read the config file
		data, err := io.ReadAll(config)
		if err != nil {
			return nil, fmt.Errorf("unable to read cloud configuration from %q [%v]", config, err)
		}

		var cfg cpcfg.Config
		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			// we got an error where the decode wasn't related to a missing type
			return nil, err
		}

		return newVSphereParavirtual(&cfg)
	})

	flag.BoolVar(&vmservice.IsLegacy, "is-legacy-paravirtual", false, "If true, machine label selector will start with capw.vmware.com. By default, it's false, machine label selector will start with capv.vmware.com.")
	flag.BoolVar(&vpcModeEnabled, "enable-vpc-mode", false, "If true, routable pod controller will start with VPC mode. It is useful only when route controller is enabled in vsphereparavirtual mode")
	flag.StringVar(&podIPPoolType, "pod-ip-pool-type", "", "Specify if Pod IP address is Public or Private routable in VPC network. Valid values are Public and Private")
}

// Creates new Controller node interface and returns
func newVSphereParavirtual(cfg *cpcfg.Config) (*VSphereParavirtual, error) {
	cp := &VSphereParavirtual{
		cfg: cfg,
	}

	return cp, nil
}

// Initialize initializes the vSphere paravirtual cloud provider.
func (cp *VSphereParavirtual) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(0).Info("Initing vSphere Paravirtual Cloud Provider")

	if vpcModeEnabled {
		if podIPPoolType != PublicIPPoolType && podIPPoolType != PrivateIPPoolType {
			klog.Fatalf("Pod IP Pool Type can be either Public or Private in VPC network, %s is not supported", podIPPoolType)
		}
	} else {
		// NSX-T T1 or VDS network
		if podIPPoolType != "" {
			klog.Fatal("Pod IP Pool Type can be set only when the network is VPC")
		}
	}

	ownerRef, err := readOwnerRef(VsphereParavirtualCloudProviderConfigPath)
	if err != nil {
		klog.Fatalf("Failed to read ownerRef:%s", err)
	}

	client, err := clientBuilder.Client(clientName)
	if err != nil {
		klog.Fatalf("Failed to create cloud provider client: %v", err)
	}

	cp.client = client
	cp.informMgr = k8s.NewInformer(client, true)
	cp.ownerReference = ownerRef

	kcfg, err := getRestConfig(SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to create rest config to communicate with supervisor: %v", err)
	}

	clusterNS, err := getNameSpace(SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to get cluster namespace: %v", err)
	}

	routes, err := NewRoutes(clusterNS, kcfg, *cp.ownerReference, vpcModeEnabled)
	if err != nil {
		klog.Errorf("Failed to init Route: %v", err)
	}
	cp.routes = routes

	cp.informMgr.AddNodeListener(cp.nodeAdded, cp.nodeDeleted, nil)

	lb, err := NewLoadBalancer(clusterNS, kcfg, cp.ownerReference)
	if err != nil {
		klog.Errorf("Failed to init LoadBalancer: %v", err)
	}
	cp.loadBalancer = lb

	instances, err := NewInstances(clusterNS, kcfg)
	if err != nil {
		klog.Errorf("Failed to init Instance: %v", err)
	}
	cp.instances = instances

	if RouteEnabled {
		klog.V(0).Info("Starting routable pod controllers")

		if err := routablepod.StartControllers(kcfg, client, cp.informMgr, ClusterName, clusterNS, ownerRef, vpcModeEnabled, podIPPoolType); err != nil {
			klog.Errorf("Failed to start Routable pod controllers: %v", err)
		}
	}

	zones, err := NewZones(clusterNS, kcfg)
	if err != nil {
		klog.Errorf("Failed to init Zones: %v", err)
	}
	cp.zones = zones

	cp.informMgr.Listen()
	klog.V(0).Info("Initing vSphere Paravirtual Cloud Provider Succeeded")
}

// LoadBalancer returns a balancer interface. Also returns true if the
// interface is supported, false otherwise.
func (cp *VSphereParavirtual) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	klog.V(1).Info("Enabling load balancer support in vsphere paravirtual cloud provider")
	return cp.loadBalancer, true
}

// Instances returns an instances interface. Also returns true if the
// interface is supported, false otherwise.
func (cp *VSphereParavirtual) Instances() (cloudprovider.Instances, bool) {
	klog.V(1).Info("Enabling Instances interface on vsphere paravirtual cloud provider")
	return cp.instances, true
}

// InstancesV2 returns an implementation of cloudprovider.InstancesV2.
//
//	TODO: implement this for v1.20
func (cp *VSphereParavirtual) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface
// is supported, false otherwise.
func (cp *VSphereParavirtual) Zones() (cloudprovider.Zones, bool) {
	klog.V(1).Info("Enabling Zones interface on vsphere paravirtual cloud provider")
	return cp.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface
// is supported, false otherwise.
func (cp *VSphereParavirtual) Clusters() (cloudprovider.Clusters, bool) {
	klog.V(1).Info("The vsphere paravirtual cloud provider does not support clusters")
	return nil, false
}

// Routes returns a routes interface along with whether the interface
// is supported.
func (cp *VSphereParavirtual) Routes() (cloudprovider.Routes, bool) {
	klog.V(1).Info("Enabling Routes interface on vsphere paravirtual cloud provider")
	return cp.routes, true
}

// ProviderName returns the cloud provider ID.
// Note: Returns 'vsphere' instead of 'vsphere-paravirtual'
// since CAPV expects the ProviderID to be in form 'vsphere://***'
// https://github.com/kubernetes/cloud-provider-vsphere/issues/447
func (cp *VSphereParavirtual) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required and set/
func (cp *VSphereParavirtual) HasClusterID() bool {
	return true
}

// Notification handler when node is added into k8s cluster.
func (cp *VSphereParavirtual) nodeAdded(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeAdded: unrecognized object %+v", obj)
		return
	}

	if cp.routes != nil {
		klog.V(6).Infof("adding node: %s", node.Name)
		cp.routes.AddNode(node)
	}
}

// Notification handler when node is removed from k8s cluster.
func (cp *VSphereParavirtual) nodeDeleted(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeDeleted: unrecognized object %+v", obj)
		return
	}

	if cp.routes != nil {
		klog.V(6).Infof("deleting node: %s", node.Name)
		cp.routes.DeleteNode(node)
	}
}
