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
	"sync"

	"k8s.io/api/core/v1"
	clientv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/cloud-provider-vsphere/pkg/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

// VSphere is an implementation of cloud provider Interface for VSphere.
type VSphere struct {
	// client        *godo.Client
	// cfg                *Config
	vsphereInstanceMap map[string]*VSphereInstance
	nodeManager        *NodeManager
	instances          cloudprovider.Instances
}

// Config is used to read and store information from the cloud configuration file
type Config struct {
	Global struct {
		// vCenter username.
		User string `gcfg:"user"`
		// vCenter password in clear text.
		Password string `gcfg:"password"`
		// Deprecated. Use VirtualCenter to specify multiple vCenter Servers.
		// vCenter IP.
		VCenterIP string `gcfg:"server"`
		// vCenter port.
		VCenterPort string `gcfg:"port"`
		// True if vCenter uses self-signed cert.
		InsecureFlag bool `gcfg:"insecure-flag"`
		// Datacenter in which VMs are located.
		Datacenters string `gcfg:"datacenters"`
		// Soap round tripper count (retries = RoundTripper - 1)
		RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
		// Name of the secret were vCenter credentials are present.
		SecretName string `gcfg:"secret-name"`
		// Secret Namespace where secret will be present that has vCenter credentials.
		SecretNamespace string `gcfg:"secret-namespace"`
	}
	VirtualCenter map[string]*VirtualCenterConfig

	Network struct {
		// PublicNetwork is name of the network the VMs are joined to.
		PublicNetwork string `gcfg:"public-network"`
	}
}

// Represents a vSphere instance where one or more kubernetes nodes are running.
type VSphereInstance struct {
	conn *vclib.VSphereConnection
	cfg  *VirtualCenterConfig
}

// Structure that represents Virtual Center configuration
type VirtualCenterConfig struct {
	// vCenter username.
	User string `gcfg:"user"`
	// vCenter password in clear text.
	Password string `gcfg:"password"`
	// vCenter port.
	VCenterPort string `gcfg:"port"`
	// Datacenter in which VMs are located.
	Datacenters string `gcfg:"datacenters"`
	// Soap round tripper count (retries = RoundTripper - 1)
	RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
	// PublicNetwork is name of the network the VMs are joined to.
	PublicNetwork string `gcfg:"public-network"`
}

// Stores info about the kubernetes node
type NodeInfo struct {
	dataCenter    *vclib.Datacenter
	vm            *vclib.VirtualMachine
	vcServer      string
	UUID          string
	NodeName      string
	NodeAddresses []v1.NodeAddress
}

type NodeManager struct {
	// TODO: replace map with concurrent map when k8s supports go v1.9

	// Maps the VC server to VSphereInstance
	vsphereInstanceMap map[string]*VSphereInstance
	// Maps node name to node info.
	nodeInfoMap map[string]*NodeInfo
	//CredentialsManager
	credentialManager *SecretCredentialManager

	// Mutexes
	nodeInfoLock          sync.RWMutex
	credentialManagerLock sync.Mutex
}

type NodeDetails struct {
	NodeName string
	UUID     string
}

type SecretCache struct {
	cacheLock     sync.Mutex
	VirtualCenter map[string]*Credential
	Secret        *v1.Secret
}

type Credential struct {
	User     string `gcfg:"user"`
	Password string `gcfg:"password"`
}

type SecretCredentialManager struct {
	SecretName      string
	SecretNamespace string
	SecretLister    clientv1.SecretLister
	Cache           *SecretCache
}

type instances struct {
	nodeManager *NodeManager
}
