/*
Copyright 2018 The Kubernetes Authors.

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

package vsphere

import (
	"io"
	"runtime"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/server"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

const (
	// ProviderName is the name of the cloud provider registered with
	// Kubernetes.
	ProviderName string = "vsphere"
	// ClientName is the user agent passed into the controller client builder.
	ClientName string = "vsphere-cloud-controller-manager"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := vcfg.ReadConfig(config)
		if err != nil {
			return nil, err
		}
		return newVSphere(cfg, true)
	})
}

// Creates new Controller node interface and returns
func newVSphere(cfg *vcfg.Config, finalize ...bool) (*VSphere, error) {
	vs, err := buildVSphereFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	if len(finalize) == 1 && finalize[0] {
		// optional for use in tests
		runtime.SetFinalizer(vs, logout)
	}
	return vs, nil
}

// Initialize initializes the cloud provider.
func (vs *VSphere) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	client, err := clientBuilder.Client(ClientName)
	if err == nil {
		klog.V(1).Info("Kubernetes Client Init Succeeded")

		vs.informMgr = k8s.NewInformer(client, true)

		connMgr := cm.NewConnectionManager(vs.cfg, vs.informMgr, client)
		vs.connectionManager = connMgr
		vs.nodeManager.connectionManager = connMgr

		vs.informMgr.AddNodeListener(vs.nodeAdded, vs.nodeDeleted, nil)

		vs.informMgr.Listen()

		//if running secrets, init them
		connMgr.InitializeSecretLister()

		if !vs.cfg.Global.APIDisable {
			klog.V(1).Info("Starting the API Server")
			vs.server.Start()
		} else {
			klog.V(1).Info("API Server is disabled")
		}
	} else {
		klog.Errorf("Kubernetes Client Init Failed: %v", err)
	}
}

// LoadBalancer returns a balancer interface. Also returns true if the
// interface is supported, false otherwise.
func (vs *VSphere) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	if vs.loadbalancer == nil {
		klog.Warning("The vSphere cloud provider does not support load balancers")
		return nil, false
	}

	return vs.loadbalancer, true
}

// Instances returns an instances interface. Also returns true if the
// interface is supported, false otherwise.
func (vs *VSphere) Instances() (cloudprovider.Instances, bool) {
	klog.V(6).Info("Calling the Instances interface on vSphere cloud provider")
	return vs.instances, true
}

// Zones returns a zones interface. Also returns true if the interface
// is supported, false otherwise.
func (vs *VSphere) Zones() (cloudprovider.Zones, bool) {
	klog.V(6).Info("Calling the Zones interface on vSphere cloud provider")
	return vs.zones, true
}

// Clusters returns a clusters interface.  Also returns true if the interface
// is supported, false otherwise.
func (vs *VSphere) Clusters() (cloudprovider.Clusters, bool) {
	klog.Warning("The vSphere cloud provider does not support clusters")
	return nil, false
}

// Routes returns a routes interface along with whether the interface
// is supported.
func (vs *VSphere) Routes() (cloudprovider.Routes, bool) {
	klog.Warning("The vSphere cloud provider does not support routes")
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (vs *VSphere) ProviderName() string {
	return ProviderName
}

// ScrubDNS is not implemented.
// TODO(akutz) Add better documentation for this function.
func (vs *VSphere) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

// HasClusterID returns true if a ClusterID is required and set/
func (vs *VSphere) HasClusterID() bool {
	return true
}

// Initializes vSphere from vSphere CloudProvider Configuration
func buildVSphereFromConfig(cfg *vcfg.Config) (*VSphere, error) {
	nm := &NodeManager{
		nodeNameMap:    make(map[string]*NodeInfo),
		nodeUUIDMap:    make(map[string]*NodeInfo),
		nodeRegUUIDMap: make(map[string]*v1.Node),
		vcList:         make(map[string]*VCenterInfo),
	}

	var err error
	var loadbalancer cloudprovider.LoadBalancer
	if cfg.LoadbalancerNSXT != nil {
		// TODO: feature gate this config with an env var?
		loadbalancer, err = newNSXTLoadBalancer(cfg.Global.ClusterID, cfg.LoadbalancerNSXT)
		if err != nil {
			return nil, err
		}
	}

	vs := VSphere{
		cfg:          cfg,
		nodeManager:  nm,
		instances:    newInstances(nm),
		loadbalancer: loadbalancer,
		zones:        newZones(nm, cfg.Labels.Zone, cfg.Labels.Region),
		server:       server.NewServer(cfg.Global.APIBinding, nm),
	}
	return &vs, nil
}

func logout(vs *VSphere) {
	vs.connectionManager.Logout()
}

// Notification handler when node is added into k8s cluster.
func (vs *VSphere) nodeAdded(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeAdded: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.RegisterNode(node)
}

// Notification handler when node is removed from k8s cluster.
func (vs *VSphere) nodeDeleted(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeDeleted: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.UnregisterNode(node)
}
