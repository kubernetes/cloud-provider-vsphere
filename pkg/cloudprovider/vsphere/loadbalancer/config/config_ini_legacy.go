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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/gcfg.v1"

	klog "k8s.io/klog/v2"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (lbc *LBConfigINI) CreateConfig() *LBConfig {
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

func (lbc *LBConfigINI) isEnabled() bool {
	return len(lbc.LoadBalancerClass) > 0 || !lbc.LoadBalancer.isEmpty()
}

func (lbc *LBConfigINI) validateConfig() error {
	if lbc.LoadBalancer.LBServiceID == "" && lbc.LoadBalancer.Tier1GatewayPath == "" {
		msg := "either load balancer service id or T1 gateway path required"
		klog.Error(msg)
		return errors.New(msg)
	}
	if lbc.LoadBalancer.TCPAppProfileName == "" && lbc.LoadBalancer.TCPAppProfilePath == "" {
		msg := "either load balancer TCP application profile name or path required"
		klog.Error(msg)
		return errors.New(msg)
	}
	if lbc.LoadBalancer.UDPAppProfileName == "" && lbc.LoadBalancer.UDPAppProfilePath == "" {
		msg := "either load balancer UDP application profile name or path required"
		klog.Error(msg)
		return errors.New(msg)
	}
	if !LoadBalancerSizes.Has(lbc.LoadBalancer.Size) {
		msg := fmt.Sprintf("load balancer size is invalid. Valid values are: %s", strings.Join(LoadBalancerSizes.List(), ","))
		klog.Error(msg)
		return errors.New(msg)
	}
	if lbc.LoadBalancer.IPPoolID == "" && lbc.LoadBalancer.IPPoolName == "" {
		class, ok := lbc.LoadBalancerClass[DefaultLoadBalancerClass]
		if !ok {
			msg := "no default load balancer class defined"
			klog.Error(msg)
			return errors.New(msg)
		} else if class.IPPoolName == "" && class.IPPoolID == "" {
			msg := "default load balancer class: ipPoolName and ipPoolID is empty"
			klog.Error(msg)
			return errors.New(msg)
		}
	} else {
		if lbc.LoadBalancer.IPPoolName != "" && lbc.LoadBalancer.IPPoolID != "" {
			msg := "either load balancer ipPoolName or ipPoolID can be set"
			klog.Error(msg)
			return errors.New(msg)
		}
	}
	return nil
}

func (lbc *LoadBalancerConfigINI) isEmpty() bool {
	return lbc.Size == "" && lbc.LBServiceID == "" &&
		lbc.IPPoolID == "" && lbc.IPPoolName == "" &&
		lbc.Tier1GatewayPath == ""
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (lbc *LBConfigINI) CompleteAndValidate() error {
	if !lbc.isEnabled() {
		return nil
	}

	lbc.LoadBalancer.AdditionalTags = map[string]string{}
	if lbc.LoadBalancer.RawTags != "" {
		err := json.Unmarshal([]byte(lbc.LoadBalancer.RawTags), &lbc.LoadBalancer.AdditionalTags)
		if err != nil {
			return fmt.Errorf("unmarshalling load balancer tags failed: %s", err)
		}
	}
	if lbc.LoadBalancerClass == nil {
		lbc.LoadBalancerClass = map[string]*LoadBalancerClassConfigINI{}
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

// ReadRawConfigINI parses vSphere cloud config file and stores it into ConfigINI
func ReadRawConfigINI(byConfig []byte) (*LBConfigINI, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid INI file")
	}

	strConfig := string(byConfig[:])

	cfg := &LBConfigINI{
		LoadBalancerClass: make(map[string]*LoadBalancerClassConfigINI),
	}

	if err := gcfg.FatalOnly(gcfg.ReadStringInto(cfg, strConfig)); err != nil {
		return nil, err
	}

	err := cfg.CompleteAndValidate()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// ReadConfigINI parses vSphere cloud config file and stores it into Config
func ReadConfigINI(byConfig []byte) (*LBConfig, error) {
	cfg, err := ReadRawConfigINI(byConfig)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
