/*
Copyright 2020 New The Kubernetes Authors.

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

	yaml "gopkg.in/yaml.v2"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be renamed
	from config_yaml.go to config.go
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (ccy *CPIConfigYAML) CreateConfig() *CPIConfig {
	cfg := &CPIConfig{
		*ccy.CommonConfigYAML.CreateConfig(),
		Nodes{
			InternalNetworkSubnetCIDR: ccy.Nodes.InternalNetworkSubnetCIDR,
			ExternalNetworkSubnetCIDR: ccy.Nodes.ExternalNetworkSubnetCIDR,
			InternalVMNetworkName:     ccy.Nodes.InternalVMNetworkName,
			ExternalVMNetworkName:     ccy.Nodes.ExternalVMNetworkName,
		},
	}

	return cfg
}

// ReadCPIConfigYAML parses vSphere cloud config file and stores it into CPIConfigYAML.
func ReadCPIConfigYAML(byConfig []byte) (*CPIConfig, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid YAML file")
	}

	// Must grab the entire config then overwrite it...
	cfgOLD := &CPIConfigYAML{}

	if err := yaml.Unmarshal(byConfig, cfgOLD); err != nil {
		return nil, err
	}

	// with this so that we can call the validate function within ReadRawConfigINI
	vCFG, err := vcfg.ReadRawConfigYAML(byConfig)
	if err != nil {
		return nil, err
	}

	cfg := &CPIConfigYAML{*vCFG, cfgOLD.Nodes}

	return cfg.CreateConfig(), nil
}
