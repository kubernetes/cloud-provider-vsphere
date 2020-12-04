/*
 Copyright 2020 The Kubernetes Authors.

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

	klog "k8s.io/klog/v2"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this functions below should be preserved
*/

// IsEnabled checks whether the load balancer feature is enabled
// It is enabled if any flavor of the load balancer configuration is given.
func (cfg *LBConfig) IsEnabled() bool {
	return len(cfg.LoadBalancerClass) > 0 || !cfg.LoadBalancer.IsEmpty()
}

// IsEmpty checks whether the load balancer config is empty (no values specified)
func (cfg *LoadBalancerConfig) IsEmpty() bool {
	return cfg.Size == "" && cfg.LBServiceID == "" &&
		cfg.IPPoolID == "" && cfg.IPPoolName == "" &&
		cfg.Tier1GatewayPath == ""
}

/*
	TODO:
	When the INI based cloud-config is deprecated, the references to the
	INI based code (ie the call to ReadConfigINI) below should be deleted.
*/

// ReadLBConfig parses vSphere cloud config file and stores it into VSphereConfig.
// Environment variables are also checked
func ReadLBConfig(byConfig []byte) (*LBConfig, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid YAML/INI file")
	}

	cfg, err := ReadConfigYAML(byConfig)
	if err != nil {
		klog.Warningf("ReadConfigYAML failed: %s", err)

		cfg, err = ReadConfigINI(byConfig)
		if err != nil {
			klog.Errorf("ReadConfigINI failed: %s", err)
			return nil, err
		}

		klog.Info("ReadConfig INI succeeded. LoadBalancer INI-based cloud-config is deprecated and will be removed in 2.0. Please use YAML based cloud-config.")
	} else {
		klog.Info("ReadConfig YAML succeeded")
	}

	// Env Vars should override config file entries if present
	if err := cfg.NSXT.FromEnv(); err != nil {
		klog.Errorf("FromEnv failed: %s", err)
		return nil, err
	}

	klog.Info("Config initialized")
	return cfg, nil
}
