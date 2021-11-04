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
	"strings"

	yaml "gopkg.in/yaml.v2"
	klog "k8s.io/klog/v2"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be merged into config.go
	and this file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (lbc *LBConfigYAML) CreateConfig() *LBConfig {
	cfg := &LBConfig{
		LoadBalancerClass: make(map[string]*LoadBalancerClassConfig),
	}

	//LoadBalancerClassConfig
	cfg.LoadBalancer.IPPoolName = lbc.LoadBalancer.IPPoolName
	cfg.LoadBalancer.IPPoolID = lbc.LoadBalancer.IPPoolID
	cfg.LoadBalancer.TCPAppProfileName = lbc.LoadBalancer.TCPAppProfileName
	cfg.LoadBalancer.TCPAppProfilePath = lbc.LoadBalancer.TCPAppProfilePath
	cfg.LoadBalancer.UDPAppProfileName = lbc.LoadBalancer.UDPAppProfileName
	cfg.LoadBalancer.UDPAppProfilePath = lbc.LoadBalancer.UDPAppProfilePath
	//LoadBalancerClassConfig -> LoadBalancerConfig
	cfg.LoadBalancer.Size = lbc.LoadBalancer.Size
	cfg.LoadBalancer.LBServiceID = lbc.LoadBalancer.LBServiceID
	cfg.LoadBalancer.Tier1GatewayPath = lbc.LoadBalancer.Tier1GatewayPath
	cfg.LoadBalancer.SnatDisabled = lbc.LoadBalancer.SnatDisabled
	cfg.LoadBalancer.AdditionalTags = lbc.LoadBalancer.AdditionalTags

	//LoadBalancerClass
	for key, value := range lbc.LoadBalancerClass {
		cfg.LoadBalancerClass[key] = &LoadBalancerClassConfig{
			IPPoolName:        value.IPPoolName,
			IPPoolID:          value.IPPoolID,
			TCPAppProfileName: value.TCPAppProfileName,
			TCPAppProfilePath: value.TCPAppProfilePath,
			UDPAppProfileName: value.UDPAppProfileName,
			UDPAppProfilePath: value.UDPAppProfilePath,
		}
	}
	return cfg
}

func (lbc *LBConfigYAML) isEnabled() bool {
	return len(lbc.LoadBalancerClass) > 0 || !lbc.LoadBalancer.isEmpty()
}

func (lbc *LBConfigYAML) validateConfig() error {
	if lbc.LoadBalancer.LBServiceID == "" && lbc.LoadBalancer.Tier1GatewayPath == "" {
		msg := "either load balancer service id or T1 gateway path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if lbc.LoadBalancer.TCPAppProfileName == "" && lbc.LoadBalancer.TCPAppProfilePath == "" {
		msg := "either load balancer TCP application profile name or path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if lbc.LoadBalancer.UDPAppProfileName == "" && lbc.LoadBalancer.UDPAppProfilePath == "" {
		msg := "either load balancer UDP application profile name or path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if !LoadBalancerSizes.Has(lbc.LoadBalancer.Size) {
		msg := fmt.Sprintf("load balancer size is invalid. Valid values are: %s", strings.Join(LoadBalancerSizes.List(), ","))
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if lbc.LoadBalancer.IPPoolID == "" && lbc.LoadBalancer.IPPoolName == "" {
		class, ok := lbc.LoadBalancerClass[DefaultLoadBalancerClass]
		if !ok {
			msg := "no default load balancer class defined"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		} else if class.IPPoolName == "" && class.IPPoolID == "" {
			msg := "default load balancer class: ipPoolName and ipPoolID is empty"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	} else {
		if lbc.LoadBalancer.IPPoolName != "" && lbc.LoadBalancer.IPPoolID != "" {
			msg := "either load balancer ipPoolName or ipPoolID can be set"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	}
	return nil
}

func (lbc *LoadBalancerConfigYAML) isEmpty() bool {
	return lbc.Size == "" && lbc.LBServiceID == "" &&
		lbc.IPPoolID == "" && lbc.IPPoolName == "" &&
		lbc.Tier1GatewayPath == ""
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (lbc *LBConfigYAML) CompleteAndValidate() error {
	if !lbc.isEnabled() {
		return nil
	}

	if lbc.LoadBalancerClass == nil {
		lbc.LoadBalancerClass = map[string]*LoadBalancerClassConfigYAML{}
	}
	for _, class := range lbc.LoadBalancerClass {
		if class.IPPoolName == "" {
			class.IPPoolName = lbc.LoadBalancer.IPPoolName
		}
		if class.IPPoolID == "" {
			class.IPPoolID = lbc.LoadBalancer.IPPoolID
		}
	}

	return lbc.validateConfig()
}

// ReadRawConfigYAML parses vSphere cloud config file and stores it into ConfigYAML
func ReadRawConfigYAML(byConfig []byte) (*LBConfigYAML, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid YAML file")
	}

	cfg := LBConfigYAML{
		LoadBalancerClass: make(map[string]*LoadBalancerClassConfigYAML),
	}

	if err := yaml.Unmarshal(byConfig, &cfg); err != nil {
		klog.Errorf("Unmarshal failed: %s", err)
		return nil, err
	}

	err := cfg.CompleteAndValidate()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ReadConfigYAML parses vSphere cloud config file and stores it into Config
func ReadConfigYAML(byConfig []byte) (*LBConfig, error) {
	cfg, err := ReadRawConfigYAML(byConfig)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
