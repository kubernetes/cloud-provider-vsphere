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
	nsxtcfg "k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be renamed
	from types_yaml.go to types.go and the structs within this file should be named:

	RouteConfigYAML -> RouteConfig
	NsxtConfigYAML -> NsxtConfig
*/

// RouteConfigYAML is used to read and store information from the cloud configuration file
type RouteConfigYAML struct {
	Route RouteYAML              `yaml:"route"`
	NSXT  nsxtcfg.NsxtConfigYAML `yaml:"nsxt"`
}

// RouteYAML contains the configuration for route
type RouteYAML struct {
	RouterPath string `yaml:"routerPath"`
}
