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

	NsxtConfigYAML -> NsxtConfig
*/

// NsxtConfigYAML is used to read and store information from the cloud configuration file
type NsxtConfigYAML struct {
	NSXT NsxtYAML `yaml:"nsxt"`
}

// NsxtYAML contains the NSX-T specific configuration
type NsxtYAML struct {
	// NSX-T username.
	User string `yaml:"user"`
	// NSX-T password in clear text.
	Password string `yaml:"password"`
	// NSX-T host.
	Host string `yaml:"host"`
	// InsecureFlag is to be set to true if NSX-T uses self-signed cert.
	InsecureFlag bool `yaml:"insecureFlag"`
	// RemoteAuth is to be set to true if NSX-T uses remote authentication (authentication done through the vIDM).
	RemoteAuth bool `yaml:"remoteAuth"`
	// SecretName is the secret name for NSX-T username and password
	SecretName string `yaml:"secretName"`
	// SecretNamespace is the secret namespace for NSX-T username and password
	SecretNamespace string `yaml:"secretNamespace"`

	VMCAccessToken     string `yaml:"vmcAccessToken"`
	VMCAuthHost        string `yaml:"vmcAuthHost"`
	ClientAuthCertFile string `yaml:"clientAuthCertFile"`
	ClientAuthKeyFile  string `yaml:"clientAuthKeyFile"`
	CAFile             string `yaml:"caFile"`
}
