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
	"os"

	klog "k8s.io/klog/v2"
)

// FromCPIEnv initializes the provided configuration object with values
// obtained from environment variables. If an environment variable is set
// for a property that's already initialized, the environment variable's value
// takes precedence.
func (cfg *CPIConfig) FromCPIEnv() error {
	if err := cfg.FromEnv(); err != nil {
		return err
	}

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

	return nil
}

/*
	TODO:
	When the INI based cloud-config is deprecated, the references to the
	INI based code (ie the call to ReadConfigINI) below should be deleted.
*/

// ReadCPIConfig parses vSphere cloud config file and stores it into CPIConfig.
// Environment variables are also checked
func ReadCPIConfig(byConfig []byte) (*CPIConfig, error) {
	if len(byConfig) == 0 {
		err := fmt.Errorf("no vSphere cloud provider config file given")
		klog.Error("config is nil")
		return nil, err
	}

	cfg, err := ReadCPIConfigYAML(byConfig)
	if err != nil {
		klog.Warningf("ReadCPIConfigYAML failed: %s", err)

		cfg, err = ReadCPIConfigINI(byConfig)
		if err != nil {
			klog.Errorf("ReadConfigINI failed: %s", err)
			return nil, err
		}

		klog.Info("ReadConfig INI succeeded. CPI INI-based cloud-config is deprecated and will be removed in 2.0. Please use YAML based cloud-config.")
	} else {
		klog.Info("ReadConfig YAML succeeded")
	}

	// Env Vars should override config file entries if present
	if err := cfg.FromCPIEnv(); err != nil {
		klog.Errorf("FromEnv failed: %s", err)
		return nil, err
	}

	klog.Info("Config initialized")
	return cfg, nil
}
