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

// GRPCServer interface
type GRPCServer interface {
	Start()
}

// VSphere is an implementation of cloud provider Interface for VSphere.
type VSphere struct {
	cfg                *Config
	vsphereInstanceMap map[string]*VSphereInstance
	nodeManager        *NodeManager
	instances          cloudprovider.Instances
	server             GRPCServer
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
		// Specifies the path to a CA certificate in PEM format. Optional; if not
		// configured, the system's CA certificates will be used.
		CAFile string `gcfg:"ca-file"`
		// Thumbprint of the VCenter's certificate thumbprint
		Thumbprint string `gcfg:"thumbprint"`
		// Name of the secret were vCenter credentials are present.
		SecretName string `gcfg:"secret-name"`
		// Secret Namespace where secret will be present that has vCenter credentials.
		SecretNamespace string `gcfg:"secret-namespace"`
		// The kubernetes service account used to launch the cloud controller manager.
		// Default: cloud-controller-manager
		ServiceAccount string `gcfg:"service-account"`
		// Disable the vSphere CCM API
		// Default: true
		APIDisable bool `gcfg:"api-disable"`
		// Configurable vSphere CCM API port
		// Default: 43001
		APIBinding string `gcfg:"api-binding"`
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
	// Specifies the path to a CA certificate in PEM format. Optional; if not
	// configured, the system's CA certificates will be used.
	CAFile string `gcfg:"ca-file"`
	// Thumbprint of the VCenter's certificate thumbprint
	Thumbprint string `gcfg:"thumbprint"`
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

type DatacenterInfo struct {
	name   string
	vmList map[string]*NodeInfo
}

type VCenterInfo struct {
	address string
	dcList  map[string]*DatacenterInfo
}

type NodeManager struct {
	// Maps the VC server to VSphereInstance
	vsphereInstanceMap map[string]*VSphereInstance
	// Maps node name to node info
	nodeNameMap map[string]*NodeInfo
	// Maps UUID to node info.
	nodeUUIDMap map[string]*NodeInfo
	// Maps VC -> DC -> VM
	vcList map[string]*VCenterInfo
	// Maps UUID to node info.
	nodeRegUUIDMap map[string]*v1.Node
	// CredentialsManager
	credentialManager *SecretCredentialManager
	// NodeLister to track Node properties
	nodeLister clientv1.NodeLister

	// Mutexes
	nodeInfoLock          sync.RWMutex
	nodeRegInfoLock       sync.RWMutex
	credentialManagerLock sync.Mutex
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
