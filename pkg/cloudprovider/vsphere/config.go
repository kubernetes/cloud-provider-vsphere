/*
Copyright 2019New The Kubernetes Authors.

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
	"os"

	"gopkg.in/gcfg.v1"
)

// CPIConfig is used to read and store information (related only to the CPI) from the cloud configuration file
type CPIConfig struct {
	Nodes struct {
		// IP address on VirtualMachine's network interfaces included in the fields' CIDRs
		// that will be used in respective status.addresses fields.
		InternalNetworkSubnetCIDR string `gcfg:"internal-network-subnet-cidr"`
		ExternalNetworkSubnetCIDR string `gcfg:"external-network-subnet-cidr"`
		// IP address on VirtualMachine's VM Network names that will be used in respective
		// status.addresses fields. Note that in this configuration there must only be a
		// single IP address bound to a given vNIC when associated with these
		// VM Network names.
		// NOTE: when InternalNetworkSubnetCIDR and ExternalNetworkSubnetCIDR are set,
		// these values take presedence over InternalVMNetworkName and ExternalVMNetworkName
		InternalVMNetworkName string `gcfg:"internal-vm-network-name"`
		ExternalVMNetworkName string `gcfg:"external-vm-network-name"`
	}
}

// FromEnv initializes the provided configuratoin object with values
// obtained from environment variables. If an environment variable is set
// for a property that's already initialized, the environment variable's value
// takes precedence.
func (cfg *CPIConfig) FromEnv() {
	if v := os.Getenv("VSPHERE_NODES_INTERNAL_NETWORK_SUBNET_CIDR"); v != "" {
		cfg.Nodes.InternalNetworkSubnetCIDR = v
	}
	if v := os.Getenv("VSPHERE_NODES_EXTERNAL_NETWORK_SUBNET_CIDR"); v != "" {
		cfg.Nodes.ExternalNetworkSubnetCIDR = v
	}

	if v := os.Getenv("VSPHERE_NODES_INTERNAL_VM_NETWORK_NAME"); v != "" {
		cfg.Nodes.InternalVMNetworkName = v
	}
	if v := os.Getenv("VSPHERE_NODES_EXTERNAL_VM_NETWORK_NAME"); v != "" {
		cfg.Nodes.ExternalVMNetworkName = v
	}

	cfg.validateConfig()
}

func (cfg *CPIConfig) validateConfig() {
	if cfg.Nodes.InternalNetworkSubnetCIDR != "" {
		cfg.Nodes.InternalVMNetworkName = ""
	}
	if cfg.Nodes.ExternalNetworkSubnetCIDR != "" {
		cfg.Nodes.ExternalVMNetworkName = ""
	}
}

// ReadCPIConfig parses vSphere cloud config file and stores it into CPIConfig.
// Environment variables are also checked
func ReadCPIConfig(config io.Reader) (*CPIConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("no vSphere cloud provider config file given")
	}

	cfg := &CPIConfig{}

	if err := gcfg.FatalOnly(gcfg.ReadInto(cfg, config)); err != nil {
		return nil, err
	}

	// Env Vars should override config file entries if present
	cfg.FromEnv()

	return cfg, nil
}
