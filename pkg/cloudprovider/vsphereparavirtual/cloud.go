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

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/config"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/factory"
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

	// podIPPoolType specifies if Pod IP addresses are public or private.
	podIPPoolType string

	// serviceAnnotationPropagationEnabled if set to true, will propagate the service annotation to resource in supervisor cluster.
	serviceAnnotationPropagationEnabled bool

	// vmopAPIVersion is the VM Operator API version to use when communicating
	// with the Supervisor cluster. Defaults to v1alpha2 for backward compatibility.
	// Controlled via the --vm-operator-api-version flag.
	vmopAPIVersion string

	// clusterIPFamily is the raw --cluster-ip-family flag; Initialize resolves it
	// with ParseClusterIPFamily to one of: ipv4, ipv6, ipv4ipv6, or ipv6ipv4.
	clusterIPFamily string
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
	flag.BoolVar(&serviceAnnotationPropagationEnabled, "enable-service-annotation-propagation", false, "If true, will propagate the service annotation to resource in supervisor cluster.")
	flag.StringVar(&vmopAPIVersion, "vm-operator-api-version", factory.V1alpha2, "the API version to use when communicating with VM Operator in supervisor mode. Valid values are: "+factory.V1alpha2+", "+factory.V1alpha5+", "+factory.V1alpha6)
	flag.StringVar(&clusterIPFamily, "cluster-ip-family", "ipv4",
		"Cluster IP family for NodeInternalIP ordering and Routable Pods. "+
			"Valid values (case-insensitive): ipv4 (default), ipv6, ipv4ipv6, ipv6ipv4. "+
			"Non-ipv4 values require --vm-operator-api-version >= v1alpha6. "+
			"IPv6 or dual-stack with the route controller additionally requires --enable-vpc-mode=true.")
}

// newVSphereParavirtual creates a new VSphereParavirtual cloud provider from the given config.
func newVSphereParavirtual(cfg *cpcfg.Config) (*VSphereParavirtual, error) {
	cp := &VSphereParavirtual{
		cfg: cfg,
	}

	return cp, nil
}

// Initialize initializes the vSphere paravirtual cloud provider.
func (cp *VSphereParavirtual) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(0).Info("Initing vSphere Paravirtual Cloud Provider")

	err := config.CheckPodIPPoolType(vpcModeEnabled, podIPPoolType)
	if err != nil {
		klog.Fatalf("Invalid IP pool type: %v", err)
	}

	ownerRef, err := config.ReadOwnerRef(config.VsphereParavirtualCloudProviderConfigPath)
	if err != nil {
		klog.Fatalf("Failed to read ownerRef:%s", err)
	}

	client, err := clientBuilder.Client(clientName)
	if err != nil {
		klog.Fatalf("Failed to create cloud provider client: %v", err)
	}

	cp.client = client
	cp.informMgr = k8s.NewInformer(client)
	cp.ownerReference = ownerRef

	kcfg, err := config.GetRestConfig(config.SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to create rest config to communicate with supervisor: %v", err)
	}

	clusterNS, err := config.GetNameSpace(config.SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to get cluster namespace: %v", err)
	}

	resolvedClusterIPFamily, err := ParseClusterIPFamily(clusterIPFamily)
	if err != nil {
		klog.Fatalf("Invalid --cluster-ip-family: %v", err)
	}
	if err := validateIPFamilyConfig(resolvedClusterIPFamily, vmopAPIVersion); err != nil {
		klog.Fatalf("Invalid flag combination: %v", err)
	}

	ipv6Enabled := resolvedClusterIPFamily == ClusterIPFamilyIPv6 ||
		resolvedClusterIPFamily == ClusterIPFamilyIPv4IPv6 ||
		resolvedClusterIPFamily == ClusterIPFamilyIPv6IPv4

	// IPv6 (including dual stack) Routable Pods requires VPC mode; T1 networking does not
	// support per-family IPAddressAllocation or StaticRoute CRs.
	// This guard is scoped to RouteEnabled: IPv6 Node IP Report and Load Balancer do not
	// depend on VPC mode and must continue to work when route controllers are disabled.
	if RouteEnabled && ipv6Enabled && !vpcModeEnabled {
		klog.Fatalf("--cluster-ip-family=%s with route controller enabled requires --enable-vpc-mode=true: IPv6 and dual stack routable pods are not supported on the legacy T1 networking mode", resolvedClusterIPFamily)
	}
	klog.Infof("Using VM Operator API version: %s", vmopAPIVersion)
	vmopClient, err := factory.NewAdapter(vmopAPIVersion, kcfg)
	if err != nil {
		klog.Fatalf("Failed to create VM Operator adapter for version %s: %v", vmopAPIVersion, err)
	}

	routes, err := NewRoutes(clusterNS, kcfg, *cp.ownerReference, vpcModeEnabled, cp.informMgr.GetNodeLister())
	if err != nil {
		klog.Errorf("Failed to init Route: %v", err)
	}
	cp.routes = routes

	lb, err := NewLoadBalancer(clusterNS, vmopClient, cp.ownerReference, serviceAnnotationPropagationEnabled)
	if err != nil {
		klog.Errorf("Failed to init LoadBalancer: %v", err)
	}
	cp.loadBalancer = lb

	klog.Infof("Using cluster IP family: %s", resolvedClusterIPFamily)
	instances, err := NewInstances(clusterNS, vmopClient, resolvedClusterIPFamily)
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

	zones, err := NewZones(clusterNS, vmopClient)
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
// Not implemented; the CPI uses the v1 Instances interface instead.
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
