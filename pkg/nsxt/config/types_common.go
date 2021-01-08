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

// Config is used to read and store information from the cloud configuration file
type Config struct {
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
	// SecretName is the secret name for NSX-T username and password
	SecretName string
	// SecretNamespace is the secret namespace for NSX-T username and password
	SecretNamespace string

	VMCAccessToken     string
	VMCAuthHost        string
	ClientAuthCertFile string
	ClientAuthKeyFile  string
	CAFile             string
}
