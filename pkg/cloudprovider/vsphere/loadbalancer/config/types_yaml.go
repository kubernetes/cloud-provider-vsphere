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
	NsxtConfigYAML -> NsxtConfig
*/

// LBConfigYAML  is used to read and store information from the cloud configuration file
type LBConfigYAML struct {
	LoadBalancer        LoadBalancerConfigYAML                  `yaml:"LoadBalancer"`
	LoadBalancerClasses map[string]*LoadBalancerClassConfigYAML `yaml:"LoadBalancerClass"`
	NSXT                NsxtConfigYAML                          `yaml:"NSX-T"`
}

// LoadBalancerConfigYAML contains the configuration for the load balancer itself
type LoadBalancerConfigYAML struct {
	LoadBalancerClassConfigYAML
	Size             string            `yaml:"size"`
	LBServiceID      string            `yaml:"lbServiceId"`
	Tier1GatewayPath string            `yaml:"tier1GatewayPath"`
	AdditionalTags   map[string]string `yaml:"tags"`
}

// LoadBalancerClassConfigYAML contains the configuration for a load balancer class
type LoadBalancerClassConfigYAML struct {
	IPPoolName        string `yaml:"ipPoolName"`
	IPPoolID          string `yaml:"ipPoolID"`
	TCPAppProfileName string `yaml:"tcpAppProfileName"`
	TCPAppProfilePath string `yaml:"tcpAppProfilePath"`
	UDPAppProfileName string `yaml:"udpAppProfileName"`
	UDPAppProfilePath string `yaml:"udpAppProfilePath"`
}

// NsxtConfigYAML contains the NSX-T specific configuration
type NsxtConfigYAML struct {
	// NSX-T username.
	User string `yaml:"user"`
	// NSX-T password in clear text.
	Password string `yaml:"password"`
	// NSX-T host.
	Host string `yaml:"host"`
	// InsecureFlag is to be set to true if NSX-T uses self-signed cert.
	InsecureFlag bool `yaml:"insecure-flag"`

	VMCAccessToken     string `yaml:"vmcAccessToken"`
	VMCAuthHost        string `yaml:"vmcAuthHost"`
	ClientAuthCertFile string `yaml:"client-auth-cert-file"`
	ClientAuthKeyFile  string `yaml:"client-auth-key-file"`
	CAFile             string `yaml:"ca-file"`
}
