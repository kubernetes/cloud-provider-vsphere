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
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

// GRPCServer interface
type GRPCServer interface {
	Start()
}

// VSphere is an implementation of cloud provider Interface for VSphere.
type VSphere struct {
	cfg                *vcfg.Config
	vsphereInstanceMap map[string]*vcfg.VSphereInstance
	nodeManager        *NodeManager
	instances          cloudprovider.Instances
	server             GRPCServer
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
	vsphereInstanceMap map[string]*vcfg.VSphereInstance
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
