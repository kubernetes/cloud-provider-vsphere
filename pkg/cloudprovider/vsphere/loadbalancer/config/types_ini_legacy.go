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
	LoadBalancer      LoadBalancerConfigINI                  `gcfg:"loadbalancer"`
	LoadBalancerClass map[string]*LoadBalancerClassConfigINI `gcfg:"loadbalancerclass"`
	NSXT              NsxtConfigINI                          `gcfg:"nsxt"`
}

// LoadBalancerConfigINI contains the configuration for the load balancer itself
type LoadBalancerConfigINI struct {
	LoadBalancerClassConfigINI
	Size             string `gcfg:"size"`
	LBServiceID      string `gcfg:"lb-service-id"`
	Tier1GatewayPath string `gcfg:"tier1-gateway-path"`
	RawTags          string `gcfg:"tags"`
	AdditionalTags   map[string]string
}

// LoadBalancerClassConfigINI contains the configuration for a load balancer class
type LoadBalancerClassConfigINI struct {
	IPPoolName        string `gcfg:"ip-pool-name"`
	IPPoolID          string `gcfg:"ip-pool-id"`
	TCPAppProfileName string `gcfg:"tcp-app-profile-name"`
	TCPAppProfilePath string `gcfg:"tcp-app-profile-path"`
	UDPAppProfileName string `gcfg:"udp-app-profile-name"`
	UDPAppProfilePath string `gcfg:"udp-app-profile-path"`
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
	// RemoteAuth is to be set to true if NSX-T uses remote authentication (authentication done through the vIDM).
	RemoteAuth bool `gcfg:"remote-auth"`

	VMCAccessToken     string `gcfg:"vmc-access-token"`
	VMCAuthHost        string `gcfg:"vmc-auth-host"`
	ClientAuthCertFile string `gcfg:"client-auth-cert-file"`
	ClientAuthKeyFile  string `gcfg:"client-auth-key-file"`
	CAFile             string `gcfg:"ca-file"`
}
