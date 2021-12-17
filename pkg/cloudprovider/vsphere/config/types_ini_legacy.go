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

package config

import (
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

// NodesINI captures internal/external networks
type NodesINI struct {
	// IP address on VirtualMachine's network interfaces included in the fields' CIDRs
	// that will be used in respective status.addresses fields.
	InternalNetworkSubnetCIDR string `gcfg:"internal-network-subnet-cidr"`
	ExternalNetworkSubnetCIDR string `gcfg:"external-network-subnet-cidr"`
	// IP address on VirtualMachine's VM Network names that will be used to when searching
	// for status.addresses fields. Note that if InternalNetworkSubnetCIDR and
	// ExternalNetworkSubnetCIDR are not set, then the vNIC associated to this network must
	// only have a single IP address assigned to it.
	InternalVMNetworkName string `gcfg:"internal-vm-network-name"`
	ExternalVMNetworkName string `gcfg:"external-vm-network-name"`
	// IP addresses in these subnet ranges will be excluded when selecting
	// the IP address from the VirtualMachine's VM for use in the
	// status.addresses fields.
	ExcludeInternalNetworkSubnetCIDR string `gcfg:"exclude-internal-network-subnet-cidr"`
	ExcludeExternalNetworkSubnetCIDR string `gcfg:"exclude-external-network-subnet-cidr"`
}

// CPIConfigINI is the INI representation
type CPIConfigINI struct {
	vcfg.CommonConfigINI
	Nodes NodesINI
}
