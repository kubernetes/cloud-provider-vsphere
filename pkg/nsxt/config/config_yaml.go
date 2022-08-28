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

	"gopkg.in/yaml.v2"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be merged into config.go
	and this file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (ncy *NsxtConfigYAML) CreateConfig() *Config {
	cfg := &Config{}
	cfg.User = ncy.NSXT.User
	cfg.Password = ncy.NSXT.Password
	cfg.Host = ncy.NSXT.Host
	cfg.InsecureFlag = ncy.NSXT.InsecureFlag
	cfg.RemoteAuth = ncy.NSXT.RemoteAuth
	cfg.VMCAccessToken = ncy.NSXT.VMCAccessToken
	cfg.VMCAuthHost = ncy.NSXT.VMCAuthHost
	cfg.ClientAuthCertFile = ncy.NSXT.ClientAuthCertFile
	cfg.ClientAuthKeyFile = ncy.NSXT.ClientAuthKeyFile
	cfg.CAFile = ncy.NSXT.CAFile
	cfg.SecretName = ncy.NSXT.SecretName
	cfg.SecretNamespace = ncy.NSXT.SecretNamespace

	return cfg
}

// validateConfig checks NSXT configurations
func (cfg *NsxtYAML) validateConfig() error {
	if cfg.VMCAccessToken != "" {
		if cfg.VMCAuthHost == "" {
			return errors.New("vmc auth host must be provided if auth token is provided")
		}
	} else if cfg.User != "" {
		if cfg.Password == "" {
			return errors.New("password is empty")
		}
	} else if cfg.ClientAuthKeyFile != "" {
		if cfg.ClientAuthCertFile == "" {
			return errors.New("client cert file is required if client key file is provided")
		}
	} else if cfg.ClientAuthCertFile != "" {
		if cfg.ClientAuthKeyFile == "" {
			return errors.New("client key file is required if client cert file is provided")
		}
	} else if cfg.SecretName != "" {
		if cfg.SecretNamespace == "" {
			return errors.New("secret namespace is required if secret name is provided")
		}
	} else if cfg.SecretNamespace != "" {
		if cfg.SecretName == "" {
			return errors.New("secret name is required if secret namespace is provided")
		}
	} else {
		return errors.New("user or vmc access token or client cert file must be set")
	}
	if cfg.Host == "" {
		return errors.New("host is empty")
	}
	return nil
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (ncy *NsxtConfigYAML) CompleteAndValidate() error {
	return ncy.NSXT.validateConfig()
}

// ReadRawConfigYAML parses vSphere cloud config file and stores it into ConfigYAML
func ReadRawConfigYAML(configData []byte) (*NsxtConfigYAML, error) {
	if len(configData) == 0 {
		return nil, fmt.Errorf("Invalid YAML file")
	}

	cfg := NsxtConfigYAML{}

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
