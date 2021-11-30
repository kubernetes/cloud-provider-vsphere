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

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be renamed
	from types_yaml.go to types.go and the structs within this file should be named:

	LBConfigYAML -> LBConfig
	LoadBalancerClassConfigYAML -> LoadBalancerClassConfig
	LoadBalancerClassConfigYAML -> LoadBalancerClassConfig
*/

// LBConfigYAML  is used to read and store information from the cloud configuration file
type LBConfigYAML struct {
	LoadBalancer      LoadBalancerConfigYAML                  `yaml:"loadBalancer"`
	LoadBalancerClass map[string]*LoadBalancerClassConfigYAML `yaml:"loadBalancerClass"`
}

// LoadBalancerConfigYAML contains the configuration for the load balancer itself
type LoadBalancerConfigYAML struct {
	Size             string            `yaml:"size"`
	LBServiceID      string            `yaml:"lbServiceId"`
	Tier1GatewayPath string            `yaml:"tier1GatewayPath"`
	SnatDisabled     bool              `yaml:"snatDisabled"`
	AdditionalTags   map[string]string `yaml:"tags"`

	// this struct use to inherit from LoadBalancerClassConfigYAML, but the YAML parser
	// wasnt able to indirectly parse inherited fields
	IPPoolName        string `yaml:"ipPoolName"`
	IPPoolID          string `yaml:"ipPoolId"`
	TCPAppProfileName string `yaml:"tcpAppProfileName"`
	TCPAppProfilePath string `yaml:"tcpAppProfilePath"`
	UDPAppProfileName string `yaml:"udpAppProfileName"`
	UDPAppProfilePath string `yaml:"udpAppProfilePath"`
}

// LoadBalancerClassConfigYAML contains the configuration for a load balancer class
type LoadBalancerClassConfigYAML struct {
	IPPoolName        string `yaml:"ipPoolName"`
	IPPoolID          string `yaml:"ipPoolId"`
	TCPAppProfileName string `yaml:"tcpAppProfileName"`
	TCPAppProfilePath string `yaml:"tcpAppProfilePath"`
	UDPAppProfileName string `yaml:"udpAppProfileName"`
	UDPAppProfilePath string `yaml:"udpAppProfilePath"`
}
