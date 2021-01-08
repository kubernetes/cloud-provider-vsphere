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

	"gopkg.in/gcfg.v1"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (nci *NsxtConfigINI) CreateConfig() *Config {
	cfg := &Config{}
	cfg.User = nci.NSXT.User
	cfg.Password = nci.NSXT.Password
	cfg.Host = nci.NSXT.Host
	cfg.InsecureFlag = nci.NSXT.InsecureFlag
	cfg.VMCAccessToken = nci.NSXT.VMCAccessToken
	cfg.VMCAuthHost = nci.NSXT.VMCAuthHost
	cfg.ClientAuthCertFile = nci.NSXT.ClientAuthCertFile
	cfg.ClientAuthKeyFile = nci.NSXT.ClientAuthKeyFile
	cfg.CAFile = nci.NSXT.CAFile
	cfg.SecretName = nci.NSXT.SecretName
	cfg.SecretNamespace = nci.NSXT.SecretNamespace

	return cfg
}

// validateConfig checks NSXT configurations
func (cfg *NsxtINI) validateConfig() error {
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
func (nci *NsxtConfigINI) CompleteAndValidate() error {
	return nci.NSXT.validateConfig()
}

// ReadRawConfigINI parses vSphere cloud config file and stores it into ConfigINI
func ReadRawConfigINI(configData []byte) (*NsxtConfigINI, error) {
	if len(configData) == 0 {
		return nil, fmt.Errorf("Invalid INI file")
	}

	cfg := &NsxtConfigINI{}

	if err := gcfg.FatalOnly(gcfg.ReadStringInto(cfg, string(configData))); err != nil {
		return nil, err
	}

	err := cfg.CompleteAndValidate()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// ReadConfigINI parses vSphere cloud config file and stores it into Config
func ReadConfigINI(configData []byte) (*Config, error) {
	cfg, err := ReadRawConfigINI(configData)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
