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
	"fmt"
	"os"
	"strconv"

	klog "k8s.io/klog/v2"
)

// FromEnv initializes the provided configuration object with values
// obtained from environment variables. If an environment variable is set
// for a property that's already initialized, the environment variable's value
// takes precedence.
func (cfg *NsxtConfig) FromEnv() error {
	if v := os.Getenv("NSXT_MANAGER_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("NSXT_USERNAME"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("NSXT_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("NSXT_ALLOW_UNVERIFIED_SSL"); v != "" {
		InsecureFlag, err := strconv.ParseBool(v)
		if err != nil {
			klog.Errorf("Failed to parse NSXT_ALLOW_UNVERIFIED_SSL: %s", err)
			return fmt.Errorf("Failed to parse NSXT_ALLOW_UNVERIFIED_SSL: %s", err)
		}
		cfg.InsecureFlag = InsecureFlag
	}
	if v := os.Getenv("NSXT_CLIENT_AUTH_CERT_FILE"); v != "" {
		cfg.ClientAuthCertFile = v
	}
	if v := os.Getenv("NSXT_CLIENT_AUTH_KEY_FILE"); v != "" {
		cfg.ClientAuthKeyFile = v
	}
	if v := os.Getenv("NSXT_CA_FILE"); v != "" {
		cfg.CAFile = v
	}

	return nil
}
