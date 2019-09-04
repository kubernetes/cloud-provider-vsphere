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

package connectionmanager

import (
	"sync"

	clientset "k8s.io/client-go/kubernetes"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/credentialmanager"
	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// ConnectionManager encapsulates vCenter connections
type ConnectionManager struct {
	sync.Mutex

	// The k8s client init from the cloud provider service account
	client clientset.Interface

	// Maps the VC server to VSphereInstance
	VsphereInstanceMap map[string]*VSphereInstance
	// CredentialManager per VC
	// The global CredentialManager will have an entry in this map with the key of "Global"
	credentialManagers map[string]*cm.CredentialManager
}

// VSphereInstance represents a vSphere instance where one or more kubernetes nodes are running.
type VSphereInstance struct {
	Conn *vclib.VSphereConnection
	Cfg  *vcfg.VirtualCenterConfig
}

// VMDiscoveryInfo contains VM info about a discovered VM
type VMDiscoveryInfo struct {
	TenantRef  string
	DataCenter *vclib.Datacenter
	VM         *vclib.VirtualMachine
	VcServer   string
	UUID       string
	NodeName   string
}

// FcdDiscoveryInfo contains FCD info about a discovered FCD
type FcdDiscoveryInfo struct {
	TenantRef  string
	DataCenter *vclib.Datacenter
	FCDInfo    *vclib.FirstClassDiskInfo
	VcServer   string
}

// ListDiscoveryInfo represents a VC/DC pair
type ListDiscoveryInfo struct {
	TenantRef  string
	VcServer   string
	DataCenter *vclib.Datacenter
}

// ZoneDiscoveryInfo contains VC+DC info based on a given zone
type ZoneDiscoveryInfo struct {
	TenantRef  string
	DataCenter *vclib.Datacenter
	VcServer   string
}
