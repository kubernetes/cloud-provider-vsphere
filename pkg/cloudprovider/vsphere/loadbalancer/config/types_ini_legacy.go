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

// LBConfigINI  is used to read and store information from the cloud configuration file
type LBConfigINI struct {
	LoadBalancer        LoadBalancerConfigINI                  `gcfg:"LoadBalancer"`
	LoadBalancerClasses map[string]*LoadBalancerClassConfigINI `gcfg:"LoadBalancerClass"`
	NSXT                NsxtConfigINI                          `gcfg:"NSX-T"`
}

// LoadBalancerConfigINI contains the configuration for the load balancer itself
type LoadBalancerConfigINI struct {
	LoadBalancerClassConfigINI
	Size             string `gcfg:"size"`
	LBServiceID      string `gcfg:"lbServiceId"`
	Tier1GatewayPath string `gcfg:"tier1GatewayPath"`
	RawTags          string `gcfg:"tags"`
	AdditionalTags   map[string]string
}

// LoadBalancerClassConfigINI contains the configuration for a load balancer class
type LoadBalancerClassConfigINI struct {
	IPPoolName        string `gcfg:"ipPoolName"`
	IPPoolID          string `gcfg:"ipPoolID"`
	TCPAppProfileName string `gcfg:"tcpAppProfileName"`
	TCPAppProfilePath string `gcfg:"tcpAppProfilePath"`
	UDPAppProfileName string `gcfg:"udpAppProfileName"`
	UDPAppProfilePath string `gcfg:"udpAppProfilePath"`
}

// NsxtConfigINI contains the NSX-T specific configuration
type NsxtConfigINI struct {
	// NSX-T username.
	User string `gcfg:"user"`
	// NSX-T password in clear text.
	Password string `gcfg:"password"`
	// NSX-T host.
	Host string `gcfg:"host"`
	// InsecureFlag is to be set to true if NSX-T uses self-signed cert.
	InsecureFlag bool `gcfg:"insecure-flag"`

	VMCAccessToken     string `gcfg:"vmcAccessToken"`
	VMCAuthHost        string `gcfg:"vmcAuthHost"`
	ClientAuthCertFile string `gcfg:"client-auth-cert-file"`
	ClientAuthKeyFile  string `gcfg:"client-auth-key-file"`
	CAFile             string `gcfg:"ca-file"`
}
