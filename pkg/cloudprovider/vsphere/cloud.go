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

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/sample-controller/pkg/signals"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/server"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
)

const (
	ProviderName string = "vsphere"
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := vcfg.ReadConfig(config)
		if err != nil {
			return nil, err
		}
		return newVSphere(cfg)
	})
}

// Creates new Controller node interface and returns
func newVSphere(cfg vcfg.Config) (*VSphere, error) {
	vs, err := buildVSphereFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	runtime.SetFinalizer(vs, logout)
	return vs, nil
}

func (vs *VSphere) Initialize(clientBuilder controller.ControllerClientBuilder) {
	client, err := clientBuilder.Client(vs.cfg.Global.ServiceAccount)
	if err == nil {
		glog.V(1).Info("Kubernetes Client Init Succeeded")

		stopCh := signals.SetupSignalHandler()

		informerFactory := informers.NewSharedInformerFactory(client, controller.NoResyncPeriodFunc())

		var connMgr *cm.ConnectionManager
		if vs.cfg.Global.SecretNamespace != "" && vs.cfg.Global.SecretName != "" {
			glog.V(4).Info("NewConnectionManagerK8s")
			secretInformer := informerFactory.Core().V1().Secrets()
			connMgr = cm.NewConnectionManagerK8s(vs.cfg, secretInformer.Lister())
		} else if vs.cfg.Global.SecretsDirectory != "" {
			glog.V(4).Info("NewConnectionManagerGeneric")
			connMgr = cm.NewConnectionManagerGeneric(vs.cfg)
		}
		vs.connectionManager = connMgr
		vs.nodeManager.connectionManager = connMgr

		nodeInformer := informerFactory.Core().V1().Nodes().Informer()
		nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    vs.nodeAdded,
			DeleteFunc: vs.nodeDeleted,
		})

		if !vs.cfg.Global.APIDisable {
			glog.V(1).Info("Starting the API Server")
			vs.server.Start()
		} else {
			glog.V(1).Info("API Server is disabled")
		}

		go informerFactory.Start(stopCh)
	} else {
		glog.Errorf("Kubernetes Client Init Failed: %v", err)
	}
}

func (vs *VSphere) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	glog.V(1).Info("The vSphere cloud provider does not support load balancers")
	return nil, false
}

func (vs *VSphere) Instances() (cloudprovider.Instances, bool) {
	return vs.instances, true
}

func (vs *VSphere) Zones() (cloudprovider.Zones, bool) {
	glog.V(1).Info("The vSphere cloud provider does not support zones")
	return nil, false
}

func (vs *VSphere) Clusters() (cloudprovider.Clusters, bool) {
	return nil, true
}

func (vs *VSphere) Routes() (cloudprovider.Routes, bool) {
	glog.V(1).Info("The vSphere cloud provider does not support routes")
	return nil, false
}

func (vs *VSphere) ProviderName() string {
	return ProviderName
}

func (vs *VSphere) ScrubDNS(nameservers, searches []string) (nsOut, srchOut []string) {
	return nil, nil
}

func (vs *VSphere) HasClusterID() bool {
	return true
}

// Initializes vSphere from vSphere CloudProvider Configuration
func buildVSphereFromConfig(cfg vcfg.Config) (*VSphere, error) {
	nm := NodeManager{
		nodeNameMap:    make(map[string]*NodeInfo),
		nodeUUIDMap:    make(map[string]*NodeInfo),
		nodeRegUUIDMap: make(map[string]*v1.Node),
		vcList:         make(map[string]*VCenterInfo),
	}

	var nodeMgr server.NodeManagerInterface
	nodeMgr = &nm
	vs := VSphere{
		cfg:         &cfg,
		nodeManager: &nm,
		instances:   newInstances(&nm),
		server:      server.NewServer(cfg.Global.APIBinding, nodeMgr),
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
		glog.Warningf("nodeAdded: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.RegisterNode(node)
}

// Notification handler when node is removed from k8s cluster.
func (vs *VSphere) nodeDeleted(obj interface{}) {
	node, ok := obj.(*v1.Node)
	if node == nil || !ok {
		glog.Warningf("nodeDeleted: unrecognized object %+v", obj)
		return
	}

	vs.nodeManager.UnregisterNode(node)
}
