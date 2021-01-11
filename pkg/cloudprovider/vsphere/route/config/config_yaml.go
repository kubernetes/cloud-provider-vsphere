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
	"errors"
	"fmt"

	yaml "gopkg.in/yaml.v2"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be merged into config.go
	and this file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (rcy *RouteConfigYAML) CreateConfig() *Config {
	cfg := &Config{}
	cfg.Route.RouterPath = rcy.Route.RouterPath
	// NSXT configurations
	cfg.NSXT.User = rcy.NSXT.User
	cfg.NSXT.Password = rcy.NSXT.Password
	cfg.NSXT.Host = rcy.NSXT.Host
	cfg.NSXT.InsecureFlag = rcy.NSXT.InsecureFlag
	cfg.NSXT.VMCAccessToken = rcy.NSXT.VMCAccessToken
	cfg.NSXT.VMCAuthHost = rcy.NSXT.VMCAuthHost
	cfg.NSXT.ClientAuthCertFile = rcy.NSXT.ClientAuthCertFile
	cfg.NSXT.ClientAuthKeyFile = rcy.NSXT.ClientAuthKeyFile
	cfg.NSXT.CAFile = rcy.NSXT.CAFile

	return cfg
}

func (rcy *RouteConfigYAML) validateConfig() error {
	if rcy.Route.RouterPath == "" {
		return errors.New("router path is required")
	}
	return rcy.NSXT.ValidateConfig()
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (rcy *RouteConfigYAML) CompleteAndValidate() error {
	return rcy.validateConfig()
}

// ReadRawConfigYAML parses vSphere cloud config file and stores it into ConfigYAML
func ReadRawConfigYAML(configData []byte) (*RouteConfigYAML, error) {
	if len(configData) == 0 {
		return nil, fmt.Errorf("Invalid YAML file")
	}

	cfg := RouteConfigYAML{}

	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		return nil, err
	}

	err := cfg.CompleteAndValidate()
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ReadConfigYAML parses vSphere cloud config file and stores it into Config
func ReadConfigYAML(configData []byte) (*Config, error) {
	cfg, err := ReadRawConfigYAML(configData)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
