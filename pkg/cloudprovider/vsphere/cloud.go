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
	"errors"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog"

	cloudprovider "k8s.io/cloud-provider"

	"github.com/vmware/vsphere-automation-sdk-go/runtime/log"

	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer"
	lcfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/route"
	rcfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/route/config"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/server"
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
		byConfig, err := ioutil.ReadAll(config)
		if err != nil {
			klog.Errorf("ReadAll failed: %s", err)
			return nil, err
		}

		cfg, err := ccfg.ReadCPIConfig(byConfig)
		if err != nil {
			return nil, err
		}
		lbcfg, err := lcfg.ReadLBConfig(byConfig)
		if err != nil {
			lbcfg = nil //Error reading LBConfig, explicitly set to nil
		}
		routecfg, err := rcfg.ReadRouteConfig(byConfig)
		if err != nil {
			klog.Errorf("ReadRouteConfig failed: %s", err)
			routecfg = nil
		}

		return newVSphere(cfg, lbcfg, routecfg, true)
	})
}

var _ cloudprovider.Interface = &VSphere{}

// Creates new Controller node interface and returns
func newVSphere(cfg *ccfg.CPIConfig, lbcfg *lcfg.LBConfig, routecfg *rcfg.Config, finalize ...bool) (*VSphere, error) {
	vs, err := buildVSphereFromConfig(cfg, lbcfg, routecfg)
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

		connMgr := cm.NewConnectionManager(&vs.cfg.Config, vs.informMgr, client)
		vs.connectionManager = connMgr
		vs.nodeManager.connectionManager = connMgr

		vs.informMgr.AddNodeListener(vs.nodeAdded, vs.nodeDeleted, nil)

		vs.informMgr.Listen()

		// if running secrets, init them
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
	if vs.isLoadBalancerSupportEnabled() {
		klog.Info("initializing load balancer support")
		if loadbalancer.ClusterName == "" {
			klog.Warning("Missing cluster id, no periodical cleanup possible")
		}
		vs.loadbalancer.Initialize(loadbalancer.ClusterName, client, stop)
	}
}

func (vs *VSphere) isLoadBalancerSupportEnabled() bool {
	return vs.loadbalancer != nil
}

// LoadBalancer returns a balancer interface. Also returns true if the
// interface is supported, false otherwise.
func (vs *VSphere) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	if vs.isLoadBalancerSupportEnabled() {
		return vs.loadbalancer, true
	}
	klog.Warning("The vSphere cloud provider does not support load balancers")
	return nil, false
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
	if vs.routes != nil {
		return vs.routes, true
	}
	klog.Warning("Routes interface was not configured")
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
func buildVSphereFromConfig(cfg *ccfg.CPIConfig, lbcfg *lcfg.LBConfig, routecfg *rcfg.Config) (*VSphere, error) {
	nm := newNodeManager(cfg, nil)

	lb, err := loadbalancer.NewLBProvider(lbcfg)
	if err != nil {
		return nil, err
	}
	if _, ok := os.LookupEnv("ENABLE_ALPHA_NSXT_LB"); ok {
		if lb == nil {
			klog.Warning("To enable NSX-T load balancer support you need to configure section LoadBalancer")
		} else {
			klog.Infof("NSX-T load balancer support enabled. This feature is alpha, use in production at your own risk.")
			// redirect vapi logging from the NSX-T GO SDK to klog
			log.SetLogger(NewKlogBridge())
		}
	} else {
		// explicitly nil the LB interface if ENABLE_ALPHA_NSXT_LB is not set even if the LBConfig is valid
		// ENABLE_ALPHA_NSXT_LB must be explicitly enabled
		lb = nil
	}

	// add alpha dual stack feature
	for tenant := range cfg.VirtualCenter {
		if len(cfg.VirtualCenter[tenant].IPFamilyPriority) > 1 {
			if _, ok := os.LookupEnv("ENABLE_ALPHA_DUAL_STACK"); !ok {
				klog.Errorf("number of ip family provided for VCenter %s is 2, ENABLE_ALPHA_DUAL_STACK env var is not set", tenant)
				return nil, errors.New("two IP families provided, but dual stack feature is not enabled")
			}
		}
	}

	routes, err := route.NewRouteProvider(routecfg)
	if err != nil {
		return nil, err
	}

	vs := VSphere{
		cfg:          cfg,
		cfgLB:        lbcfg,
		nodeManager:  nm,
		loadbalancer: lb,
		routes:       routes,
		instances:    newInstances(nm),
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
	if vs.routes != nil {
		vs.routes.AddNode(node)
	}
}

// Notification handler when node is removed from k8s cluster.
func (vs *VSphere) nodeDeleted(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		klog.Warningf("nodeDeleted: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.UnregisterNode(node)
	if vs.routes != nil {
		vs.routes.DeleteNode(node)
	}
}
