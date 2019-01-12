/*
Copyright 2018 The Kubernetes Authors.

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

// Config is used to read and store information from the cloud configuration file
type Config struct {
	Global struct {
		// vCenter username.
		User string `gcfg:"user"`
		// vCenter password in clear text.
		Password string `gcfg:"password"`
		// Deprecated. Use VirtualCenter to specify multiple vCenter Servers.
		// vCenter IP.
		VCenterIP string `gcfg:"server"`
		// vCenter port.
		VCenterPort string `gcfg:"port"`
		// True if vCenter uses self-signed cert.
		InsecureFlag bool `gcfg:"insecure-flag"`
		// Datacenter in which VMs are located.
		Datacenters string `gcfg:"datacenters"`
		// Soap round tripper count (retries = RoundTripper - 1)
		RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
		// Specifies the path to a CA certificate in PEM format. Optional; if not
		// configured, the system's CA certificates will be used.
		CAFile string `gcfg:"ca-file"`
		// Thumbprint of the VCenter's certificate thumbprint
		Thumbprint string `gcfg:"thumbprint"`
		// Name of the secret were vCenter credentials are present.
		SecretName string `gcfg:"secret-name"`
		// Secret Namespace where secret will be present that has vCenter credentials.
		SecretNamespace string `gcfg:"secret-namespace"`
		// The kubernetes service account used to launch the cloud controller manager.
		// Default: cloud-controller-manager
		ServiceAccount string `gcfg:"service-account"`
		// Secret directory in the event that:
		// 1) we don't want to use the k8s API to listen for changes to secrets
		// 2) we are not in a k8s env, namely DC/OS, since CSI is CO agnostic
		// Default: /etc/cloud/credentials
		SecretsDirectory string `gcfg:"secrets-directory"`
		// Disable the vSphere CCM API
		// Default: true
		APIDisable bool `gcfg:"api-disable"`
		// Configurable vSphere CCM API port
		// Default: 43001
		APIBinding string `gcfg:"api-binding"`
	}

	// Virtual Center configurations
	VirtualCenter map[string]*VirtualCenterConfig

	// Tag categories and tags which correspond to "built-in node labels: zones and region"
	Labels struct {
		Zone   string `gcfg:"zone"`
		Region string `gcfg:"region"`
	}
}

// Structure that represents Virtual Center configuration
type VirtualCenterConfig struct {
	// vCenter username.
	User string `gcfg:"user"`
	// vCenter password in clear text.
	Password string `gcfg:"password"`
	// vCenter port.
	VCenterPort string `gcfg:"port"`
	// True if vCenter uses self-signed cert.
	InsecureFlag bool `gcfg:"insecure-flag"`
	// Datacenter in which VMs are located.
	Datacenters string `gcfg:"datacenters"`
	// Soap round tripper count (retries = RoundTripper - 1)
	RoundTripperCount uint `gcfg:"soap-roundtrip-count"`
	// Specifies the path to a CA certificate in PEM format. Optional; if not
	// configured, the system's CA certificates will be used.
	CAFile string `gcfg:"ca-file"`
	// Thumbprint of the VCenter's certificate thumbprint
	Thumbprint string `gcfg:"thumbprint"`
}
