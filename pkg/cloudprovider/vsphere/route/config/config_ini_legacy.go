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
func (rci *RouteConfigINI) CreateConfig() *Config {
	cfg := &Config{}
	cfg.Route.RouterPath = rci.Route.RouterPath
	return cfg
}

func (rci *RouteConfigINI) validateConfig() error {
	if rci.Route.RouterPath == "" {
		return errors.New("router path is required")
	}
	return nil
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (rci *RouteConfigINI) CompleteAndValidate() error {
	return rci.validateConfig()
}

// ReadRawConfigINI parses vSphere cloud config file and stores it into ConfigINI
func ReadRawConfigINI(configData []byte) (*RouteConfigINI, error) {
	if len(configData) == 0 {
		return nil, fmt.Errorf("Invalid INI file")
	}

	cfg := &RouteConfigINI{}

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
