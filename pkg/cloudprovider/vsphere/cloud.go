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
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"

	"github.com/golang/glog"

	"gopkg.in/gcfg.v1"

	"k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-vsphere/pkg/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/sample-controller/pkg/signals"
)

const (
	ProviderName             string = "vsphere"
	RoundTripperDefaultCount uint   = 3
)

// Error Messages
const (
	MissingUsernameErrMsg = "Username is missing"
	MissingPasswordErrMsg = "Password is missing"
)

// Error constants
var (
	ErrUsernameMissing = errors.New(MissingUsernameErrMsg)
	ErrPasswordMissing = errors.New(MissingPasswordErrMsg)
)

func init() {
	cloudprovider.RegisterCloudProvider(ProviderName, func(config io.Reader) (cloudprovider.Interface, error) {
		cfg, err := readConfig(config)
		if err != nil {
			return nil, err
		}
		return newVSphere(cfg)
	})
}

// Parses vSphere cloud config file and stores it into VSphereConfig.
func readConfig(config io.Reader) (Config, error) {
	if config == nil {
		return Config{}, fmt.Errorf("no vSphere cloud provider config file given")
	}

	var cfg Config
	err := gcfg.ReadInto(&cfg, config)
	return cfg, err
}

// Creates new Controller node interface and returns
func newVSphere(cfg Config) (*VSphere, error) {
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
		secretInformer := informerFactory.Core().V1().Secrets()
		vs.nodeManager.credentialManager = &SecretCredentialManager{
			SecretName:      vs.cfg.Global.SecretName,
			SecretNamespace: vs.cfg.Global.SecretNamespace,
			SecretLister:    secretInformer.Lister(),
			Cache: &SecretCache{
				VirtualCenter: make(map[string]*Credential),
			},
		}
		nodeInformer := informerFactory.Core().V1().Nodes().Informer()
		nodeInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    vs.nodeAdded,
			DeleteFunc: vs.nodeDeleted,
		})

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
func buildVSphereFromConfig(cfg Config) (*VSphere, error) {
	if cfg.Global.RoundTripperCount == 0 {
		cfg.Global.RoundTripperCount = RoundTripperDefaultCount
	}

	if cfg.Global.ServiceAccount == "" {
		cfg.Global.ServiceAccount = "cloud-controller-manager"
	}

	if cfg.Global.VCenterPort == "" {
		cfg.Global.VCenterPort = "443"
	}

	vsphereInstanceMap, err := populateVsphereInstanceMap(&cfg)
	if err != nil {
		return nil, err
	}

	nm := NodeManager{
		vsphereInstanceMap: vsphereInstanceMap,
		nodeNameMap:        make(map[string]*NodeInfo),
		nodeUUIDMap:        make(map[string]*NodeInfo),
		nodeRegUUIDMap:     make(map[string]*v1.Node),
	}

	vs := VSphere{
		cfg:                &cfg,
		vsphereInstanceMap: vsphereInstanceMap,
		nodeManager:        &nm,
		instances:          newInstances(&nm),
	}
	return &vs, nil
}

func populateVsphereInstanceMap(cfg *Config) (map[string]*VSphereInstance, error) {
	vsphereInstanceMap := make(map[string]*VSphereInstance)
	isSecretInfoProvided := true

	if cfg.Global.SecretName == "" || cfg.Global.SecretNamespace == "" {
		isSecretInfoProvided = false
	}

	// vsphere.conf is no longer supported in the old format.
	for vcServer, vcConfig := range cfg.VirtualCenter {
		glog.V(4).Infof("Initializing vc server %s", vcServer)
		if vcServer == "" {
			glog.Error("vsphere.conf does not have the VirtualCenter IP address specified")
			return nil, errors.New("vsphere.conf does not have the VirtualCenter IP address specified")
		}

		if !isSecretInfoProvided {
			if vcConfig.User == "" {
				vcConfig.User = cfg.Global.User
				if vcConfig.User == "" {
					glog.Errorf("vcConfig.User is empty for vc %s!", vcServer)
					fmt.Println("Throwing ErrUsernameMissing")
					return nil, ErrUsernameMissing
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cfg.Global.Password
				if vcConfig.Password == "" {
					glog.Errorf("vcConfig.Password is empty for vc %s!", vcServer)
					fmt.Println("Throwing ErrPasswordMissing")
					return nil, ErrPasswordMissing
				}
			}
		}

		if vcConfig.VCenterPort == "" {
			vcConfig.VCenterPort = cfg.Global.VCenterPort
		}

		if vcConfig.Datacenters == "" {
			if cfg.Global.Datacenters != "" {
				vcConfig.Datacenters = cfg.Global.Datacenters
			}
		}
		if vcConfig.RoundTripperCount == 0 {
			vcConfig.RoundTripperCount = cfg.Global.RoundTripperCount
		}
		if vcConfig.CAFile == "" {
			vcConfig.CAFile = cfg.Global.CAFile
		}
		if vcConfig.Thumbprint == "" {
			vcConfig.Thumbprint = cfg.Global.Thumbprint
		}

		// Note: If secrets info is provided username and password will be populated
		// once secret is created.
		vSphereConn := vclib.VSphereConnection{
			Username:          vcConfig.User,
			Password:          vcConfig.Password,
			Hostname:          vcServer,
			Insecure:          cfg.Global.InsecureFlag,
			RoundTripperCount: vcConfig.RoundTripperCount,
			Port:              vcConfig.VCenterPort,
			CACert:            vcConfig.CAFile,
			Thumbprint:        vcConfig.Thumbprint,
		}
		vsphereIns := VSphereInstance{
			conn: &vSphereConn,
			cfg:  vcConfig,
		}
		vsphereInstanceMap[vcServer] = &vsphereIns
	}

	// Create a single instance of VSphereInstance for the Global VCenterIP if the
	// VSphereInstance doesnt already exist in the map
	if !isSecretInfoProvided && cfg.Global.VCenterIP != "" && vsphereInstanceMap[cfg.Global.VCenterIP] == nil {
		glog.V(4).Infof("Creating a vc server %s for the global instance", cfg.Global.VCenterIP)
		vcConfig := &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			VCenterPort:       cfg.Global.VCenterPort,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			CAFile:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
		}
		vSphereConn := vclib.VSphereConnection{
			Username:          cfg.Global.User,
			Password:          cfg.Global.Password,
			Hostname:          cfg.Global.VCenterIP,
			Insecure:          cfg.Global.InsecureFlag,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			Port:              cfg.Global.VCenterPort,
			CACert:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
		}
		vsphereIns := VSphereInstance{
			conn: &vSphereConn,
			cfg:  vcConfig,
		}
		vsphereInstanceMap[cfg.Global.VCenterIP] = &vsphereIns
	}

	return vsphereInstanceMap, nil
}

func logout(vs *VSphere) {
	for _, vsphereIns := range vs.vsphereInstanceMap {
		if vsphereIns.conn.Client != nil {
			vsphereIns.conn.Logout(context.TODO())
		}
	}
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
