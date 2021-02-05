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
	"fmt"
	"io"
	"io/ioutil"

	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	cloudprovider "k8s.io/cloud-provider"

	cpcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

const (
	// ProviderName is the name of the cloud provider registered with
	// Kubernetes.
	ProviderName string = "vsphere-paravirtual"
	clientName   string = "vsphere-paravirtual-cloud-controller-manager"

	// CloudControllerManagerNS is the namespace for vsphere paravirtual cluster cloud provider
	CloudControllerManagerNS = "vmware-system-cloud-provider"
)

var (
	// SupervisorClusterSecret is the name of vsphere paravirtual supervisor cluster cloud provider secret
	SupervisorClusterSecret = "cloud-provider-creds"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		if config == nil {
			return nil, errors.New("no vSphere paravirtual cloud provider config file given")
		}

		// read the config file
		data, err := ioutil.ReadAll(config)
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
}

// Creates new Controller node interface and returns
func newVSphereParavirtual(cfg *cpcfg.Config) (*VSphereParavirtual, error) {
	cp := &VSphereParavirtual{
		cfg: cfg,
	}

	return cp, nil
}

// Initialize initializes the cloud provider.
func (cp *VSphereParavirtual) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	klog.V(0).Info("Initing vSphere Paravirtual Cloud Provider")

	ownerRef, err := readOwnerRef(VsphereParavirtualCloudProviderConfigPath)
	if err != nil {
		klog.Fatalf("Failed to read ownderRef:%s", err)
	}

	client, err := clientBuilder.Client(clientName)
	if err != nil {
		klog.Fatalf("Failed to create cloud provider client: %v", err)
	}

	cp.client = client
	cp.informMgr = k8s.NewInformer(client, true)
	cp.informMgr.Listen()
	cp.ownerReference = ownerRef

	kcfg, err := getRestConfig(SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to create rest config to communicate with supervisor: %v", err)
	}

	clusterNS, err := getNameSpace(SupervisorClusterConfigPath)
	if err != nil {
		klog.Fatalf("Failed to get cluster namespace: %v", err)
	}

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
	klog.V(6).Info("Enabling Instances interface on vsphere paravirtual cloud provider")
	return cp.instances, true
}

// InstancesV2 returns an implementation of cloudprovider.InstancesV2.
//  TODO: implement this for v1.20
func (cp *VSphereParavirtual) InstancesV2() (cloudprovider.InstancesV2, bool) {
	klog.Warning("The vSphere cloud provider does not support InstancesV2")
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface
// is supported, false otherwise.
func (cp *VSphereParavirtual) Zones() (cloudprovider.Zones, bool) {
	klog.V(1).Info("Enabling Zones interface on vsphere paravirtual cloud provider")
	return nil, false
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
	klog.V(1).Info("The vsphere paravirtual cloud provider does not support routes")
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (cp *VSphereParavirtual) ProviderName() string {
	return ProviderName
}

// HasClusterID returns true if a ClusterID is required and set/
func (cp *VSphereParavirtual) HasClusterID() bool {
	return true
}
