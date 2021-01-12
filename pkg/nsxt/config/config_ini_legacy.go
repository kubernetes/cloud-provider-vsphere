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

import "errors"

// ValidateConfig checks NSXT configurations
func (cfg *NsxtConfigINI) ValidateConfig() error {
	if cfg.VMCAccessToken != "" {
		if cfg.VMCAuthHost == "" {
			return errors.New("vmc auth host must be provided if auth token is provided")
		}
	} else if cfg.User != "" {
		if cfg.Password == "" {
			return errors.New("password is empty")
		}
	} else if cfg.ClientAuthKeyFile != "" {
		if cfg.ClientAuthCertFile == "" {
			return errors.New("client cert file is required if client key file is provided")
		}
	} else if cfg.ClientAuthCertFile != "" {
		if cfg.ClientAuthKeyFile == "" {
			return errors.New("client key file is required if client cert file is provided")
		}
	} else {
		return errors.New("user or vmc access token or client cert file must be set")
	}
	if cfg.Host == "" {
		return errors.New("host is empty")
	}
	return nil
}
