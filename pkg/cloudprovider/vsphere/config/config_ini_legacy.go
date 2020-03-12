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

package config

import (
	"fmt"

	ini "gopkg.in/gcfg.v1"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (cci *CPIConfigINI) CreateConfig() *CPIConfig {
	cfg := &CPIConfig{
		*cci.ConfigINI.CreateConfig(),
		Nodes{
			InternalNetworkSubnetCIDR: cci.NodesINI.InternalNetworkSubnetCIDR,
			ExternalNetworkSubnetCIDR: cci.NodesINI.ExternalNetworkSubnetCIDR,
			InternalVMNetworkName:     cci.NodesINI.InternalVMNetworkName,
			ExternalVMNetworkName:     cci.NodesINI.ExternalVMNetworkName,
		},
	}

	return cfg
}

// ReadCPIConfigINI parses vSphere cloud config file and stores it into CPIConfigYAML.
func ReadCPIConfigINI(byConfig []byte) (*CPIConfig, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid INI file")
	}

	strConfig := string(byConfig[:len(byConfig)])

	vCFG, err := vcfg.ReadRawConfigINI(byConfig)
	if err != nil {
		return nil, err
	}

	cfg := &CPIConfigINI{*vCFG, NodesINI{}}

	if err := ini.FatalOnly(ini.ReadStringInto(cfg, strConfig)); err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
