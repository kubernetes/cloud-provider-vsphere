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

	"k8s.io/cloud-provider-vsphere/pkg/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	MacOUIVCPrefix           string = "00:50:56"
	MacOUIESXPrefix          string = "00:0c:29"
	ProviderName             string = "vsphere"
	RoundTripperDefaultCount uint   = 3
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
		vs.nodeManager.credentialManager = &SecretCredentialManager{
			SecretName:      vs.cfg.Global.SecretName,
			SecretNamespace: vs.cfg.Global.SecretNamespace,
			Client:          client,
			Cache: &SecretCache{
				VirtualCenter: make(map[string]*Credential),
			},
		}
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
		nodeInfoMap:        make(map[string]*NodeInfo),
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
					return nil, errors.New("Username is missing")
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cfg.Global.Password
				if vcConfig.Password == "" {
					glog.Errorf("vcConfig.Password is empty for vc %s!", vcServer)
					return nil, errors.New("Password is missing")
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

		// Note: If secrets info is provided username and password will be populated
		// once secret is created.
		vSphereConn := vclib.VSphereConnection{
			Username:          vcConfig.User,
			Password:          vcConfig.Password,
			Hostname:          vcServer,
			Insecure:          cfg.Global.InsecureFlag,
			RoundTripperCount: vcConfig.RoundTripperCount,
			Port:              vcConfig.VCenterPort,
		}
		vsphereIns := VSphereInstance{
			conn: &vSphereConn,
			cfg:  vcConfig,
		}
		vsphereInstanceMap[vcServer] = &vsphereIns
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
