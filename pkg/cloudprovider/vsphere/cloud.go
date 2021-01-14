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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"

	v1 "k8s.io/api/core/v1"
	klog "k8s.io/klog/v2"

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
	"k8s.io/cloud-provider-vsphere/pkg/nsxt"
	ncfg "k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
)

const (
	// ProviderName is the name of the cloud provider registered with
	// Kubernetes.
	ProviderName string = "vsphere"
	// ClientName is the user agent passed into the controller client builder.
	ClientName string = "vsphere-cloud-controller-manager"

	// dualStackFeatureGateEnv is a required environment variable when enabling dual-stack nodes
	dualStackFeatureGateEnv string = "ENABLE_ALPHA_DUAL_STACK"
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
		nsxtcfg, err := ncfg.ReadNsxtConfig(byConfig)
		if err != nil {
			klog.Errorf("ReadNsxtConfig failed: %s", err)
			nsxtcfg = nil
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

		return newVSphere(cfg, nsxtcfg, lbcfg, routecfg, true)
	})
}

var _ cloudprovider.Interface = &VSphere{}

// Creates new Controller node interface and returns
func newVSphere(cfg *ccfg.CPIConfig, nsxtcfg *ncfg.Config, lbcfg *lcfg.LBConfig, routecfg *rcfg.Config, finalize ...bool) (*VSphere, error) {
	vs, err := buildVSphereFromConfig(cfg, nsxtcfg, lbcfg, routecfg)
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
	err = vs.nsxtConnectorMgr.AddSecretListener(vs.informMgr.GetSecretInformer())
	if err != nil {
		klog.Warning("Adding NSXT secret listener failed: %v", err)
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

// InstancesV2 returns an implementation of cloudprovider.InstancesV2.
//  TODO: implement this for v1.20
func (vs *VSphere) InstancesV2() (cloudprovider.InstancesV2, bool) {
	klog.Warning("The vSphere cloud provider does not support InstancesV2")
	return nil, false
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
func buildVSphereFromConfig(cfg *ccfg.CPIConfig, nsxtcfg *ncfg.Config, lbcfg *lcfg.LBConfig, routecfg *rcfg.Config) (*VSphere, error) {
	nm := newNodeManager(cfg, nil)

	ncm, err := nsxt.NewConnectorManager(nsxtcfg)
	if err != nil {
		return nil, err
	}

	lb, err := loadbalancer.NewLBProvider(lbcfg, ncm.GetConnector())
	if err != nil {
		return nil, err
	}

	routes, err := route.NewRouteProvider(routecfg, ncm.GetConnector())
	if err != nil {
		return nil, err
	}

	// redirect vapi logging from the NSX-T GO SDK to klog
	log.SetLogger(NewKlogBridge())

	err = validateDualStack(cfg)
	if err != nil {
		return nil, err
	}

	vs := VSphere{
		cfg:              cfg,
		cfgLB:            lbcfg,
		nodeManager:      nm,
		nsxtConnectorMgr: ncm,
		loadbalancer:     lb,
		routes:           routes,
		instances:        newInstances(nm),
		zones:            newZones(nm, cfg.Labels.Zone, cfg.Labels.Region),
		server:           server.NewServer(cfg.Global.APIBinding, nm),
	}
	return &vs, nil
}

// validateDualStack returns an error if dual-stack was configured but not enabled
// using the alpha environment variable feature gate ENABLE_ALPHA_DUAL_STACK
func validateDualStack(cfg *ccfg.CPIConfig) error {
	_, dualStackEnabled := os.LookupEnv(dualStackFeatureGateEnv)
	if dualStackEnabled {
		return nil
	}

	for vcName, vcConfig := range cfg.VirtualCenter {
		if len(vcConfig.IPFamilyPriority) > 1 {
			return fmt.Errorf("mulitple IP families specified for virtual center %q but ENABLE_ALPHA_DUAL_STACK env var is not set", vcName)
		}
	}

	return nil
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
