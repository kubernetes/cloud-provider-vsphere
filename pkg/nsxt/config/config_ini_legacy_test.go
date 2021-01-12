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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestINIValidateTokenConfig(t *testing.T) {
	cfg := &NsxtConfigINI{
		VMCAccessToken: "token",
	}
	err := cfg.ValidateConfig()
	assert.EqualError(t, err, "vmc auth host must be provided if auth token is provided")

	cfg.VMCAuthHost = "auth-host"
	err = cfg.ValidateConfig()
	assert.EqualError(t, err, "host is empty")

	cfg.Host = "server"
	err = cfg.ValidateConfig()
	assert.Nil(t, err)
}

func TestINIValidateUserConfig(t *testing.T) {
	cfg := &NsxtConfigINI{
		User: "admin",
	}
	err := cfg.ValidateConfig()
	assert.EqualError(t, err, "password is empty")

	cfg.Password = "secret"
	err = cfg.ValidateConfig()
	assert.EqualError(t, err, "host is empty")

	cfg.Host = "server"
	err = cfg.ValidateConfig()
	assert.Nil(t, err)
}

func TestINIValidateCertConfig(t *testing.T) {
	testCases := []struct {
		name               string
		cfg                *NsxtConfigINI
		expectedErrMessage string
	}{
		{
			name: "empty client cert file",
			cfg: &NsxtConfigINI{
				ClientAuthKeyFile: "client-key",
			},
			expectedErrMessage: "client cert file is required if client key file is provided",
		},
		{
			name: "empty client key file",
			cfg: &NsxtConfigINI{
				ClientAuthCertFile: "client-cert",
			},
			expectedErrMessage: "client key file is required if client cert file is provided",
		},
		{
			name: "empty host",
			cfg: &NsxtConfigINI{
				ClientAuthKeyFile:  "client-key",
				ClientAuthCertFile: "client-cert",
			},
			expectedErrMessage: "host is empty",
		},
		{
			name: "valid config",
			cfg: &NsxtConfigINI{
				ClientAuthKeyFile:  "client-key",
				ClientAuthCertFile: "client-cert",
				Host:               "server",
			},
			expectedErrMessage: "",
		},
	}

	for _, testCase := range testCases {
		err := testCase.cfg.ValidateConfig()
		if err != nil {
			assert.EqualError(t, err, testCase.expectedErrMessage)
		} else {
			assert.Equal(t, "", testCase.expectedErrMessage)
		}
	}
}
