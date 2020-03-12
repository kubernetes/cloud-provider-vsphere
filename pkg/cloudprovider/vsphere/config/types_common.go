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
	When the INI based cloud-config is deprecated. This file should be deleted and
	the structs in types_yaml.go will be renamed to replace the ones in this file.
*/

// Nodes captures internal/external networks
type Nodes struct {
	// IP address on VirtualMachine's network interfaces included in the fields' CIDRs
	// that will be used in respective status.addresses fields.
	InternalNetworkSubnetCIDR string
	ExternalNetworkSubnetCIDR string
	// IP address on VirtualMachine's VM Network names that will be used to when searching
	// for status.addresses fields. Note that if InternalNetworkSubnetCIDR and
	// ExternalNetworkSubnetCIDR are not set, then the vNIC associated to this network must
	// only have a single IP address assigned to it.
	InternalVMNetworkName string
	ExternalVMNetworkName string
}

// CPIConfig is used to read and store information (related only to the CPI) from the cloud configuration file
type CPIConfig struct {
	vcfg.Config
	Nodes Nodes
}
