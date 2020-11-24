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

// LBConfig  is used to read and store information from the cloud configuration file
type LBConfig struct {
	LoadBalancer      LoadBalancerConfig
	LoadBalancerClass map[string]*LoadBalancerClassConfig
	NSXT              NsxtConfig
}

// LoadBalancerConfig contains the configuration for the load balancer itself
type LoadBalancerConfig struct {
	LoadBalancerClassConfig
	Size             string
	LBServiceID      string
	Tier1GatewayPath string
	AdditionalTags   map[string]string
}

// LoadBalancerClassConfig contains the configuration for a load balancer class
type LoadBalancerClassConfig struct {
	IPPoolName        string
	IPPoolID          string
	TCPAppProfileName string
	TCPAppProfilePath string
	UDPAppProfileName string
	UDPAppProfilePath string
}

// NsxtConfig contains the NSX-T specific configuration
type NsxtConfig struct {
	// NSX-T username.
	User string
	// NSX-T password in clear text.
	Password string
	// NSX-T host.
	Host string
	// InsecureFlag is to be set to true if NSX-T uses self-signed cert.
	InsecureFlag bool
	// RemoteAuth is to be set to true if NSX-T uses remote authentication (authentication done through the vIDM).
	RemoteAuth bool

	VMCAccessToken     string
	VMCAuthHost        string
	ClientAuthCertFile string
	ClientAuthKeyFile  string
	CAFile             string
}
