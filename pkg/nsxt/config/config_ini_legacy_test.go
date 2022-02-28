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
	cfg := &NsxtINI{
		VMCAccessToken: "token",
	}
	err := cfg.validateConfig()
	assert.EqualError(t, err, "vmc auth host must be provided if auth token is provided")

	cfg.VMCAuthHost = "auth-host"
	err = cfg.validateConfig()
	assert.EqualError(t, err, "host is empty")

	cfg.Host = "server"
	err = cfg.validateConfig()
	assert.Nil(t, err)
}

func TestINIValidateUserConfig(t *testing.T) {
	cfg := &NsxtINI{
		User: "admin",
	}
	err := cfg.validateConfig()
	assert.EqualError(t, err, "password is empty")

	cfg.Password = "secret"
	err = cfg.validateConfig()
	assert.EqualError(t, err, "host is empty")

	cfg.Host = "server"
	err = cfg.validateConfig()
	assert.Nil(t, err)
}

func TestINIValidateCertConfig(t *testing.T) {
	testCases := []struct {
		name               string
		cfg                *NsxtINI
		expectedErrMessage string
	}{
		{
			name: "empty client cert file",
			cfg: &NsxtINI{
				ClientAuthKeyFile: "client-key",
			},
			expectedErrMessage: "client cert file is required if client key file is provided",
		},
		{
			name: "empty client key file",
			cfg: &NsxtINI{
				ClientAuthCertFile: "client-cert",
			},
			expectedErrMessage: "client key file is required if client cert file is provided",
		},
		{
			name: "empty host",
			cfg: &NsxtINI{
				ClientAuthKeyFile:  "client-key",
				ClientAuthCertFile: "client-cert",
			},
			expectedErrMessage: "host is empty",
		},
		{
			name: "valid config",
			cfg: &NsxtINI{
				ClientAuthKeyFile:  "client-key",
				ClientAuthCertFile: "client-cert",
				Host:               "server",
			},
			expectedErrMessage: "",
		},
	}

	for _, testCase := range testCases {
		err := testCase.cfg.validateConfig()
		if err != nil {
			assert.EqualError(t, err, testCase.expectedErrMessage)
		} else {
			assert.Equal(t, "", testCase.expectedErrMessage)
		}
	}
}

func TestINIValidateSecretConfig(t *testing.T) {
	testCases := []struct {
		name               string
		cfg                *NsxtINI
		expectedErrMessage string
	}{
		{
			name: "empty secret namespace",
			cfg: &NsxtINI{
				SecretName: "secret-name",
			},
			expectedErrMessage: "secret namespace is required if secret name is provided",
		},
		{
			name: "empty secret name",
			cfg: &NsxtINI{
				SecretNamespace: "secret-ns",
			},
			expectedErrMessage: "secret name is required if secret namespace is provided",
		},
		{
			name: "empty host",
			cfg: &NsxtINI{
				SecretName:      "secret-name",
				SecretNamespace: "secret-ns",
			},
			expectedErrMessage: "host is empty",
		},
		{
			name: "valid config",
			cfg: &NsxtINI{
				SecretName:      "secret-name",
				SecretNamespace: "secret-ns",
				Host:            "server",
			},
			expectedErrMessage: "",
		},
	}

	for _, testCase := range testCases {
		err := testCase.cfg.validateConfig()
		if err != nil {
			assert.EqualError(t, err, testCase.expectedErrMessage)
		} else {
			assert.Equal(t, "", testCase.expectedErrMessage)
		}
	}
}

func TestReadRawConfigINI(t *testing.T) {
	contents := `
[NSXT]
user = admin
password = secret
host = nsxt-server
insecure-flag = false
remote-auth = true
vmc-access-token = vmc-token
vmc-auth-host = vmc-host
client-auth-cert-file = client-cert-file
client-auth-key-file = client-key-file
ca-file = ca-file
secret-name = secret-name
secret-namespace = secret-ns
`
	config, err := ReadRawConfigINI([]byte(contents))
	if err != nil {
		t.Error(err)
		return
	}

	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("NSXT.user", config.NSXT.User, "admin")
	assertEquals("NSXT.password", config.NSXT.Password, "secret")
	assertEquals("NSXT.host", config.NSXT.Host, "nsxt-server")
	assert.Equal(t, false, config.NSXT.InsecureFlag)
	assert.Equal(t, true, config.NSXT.RemoteAuth)
	assertEquals("NSXT.vmc-access-token", config.NSXT.VMCAccessToken, "vmc-token")
	assertEquals("NSXT.vmc-auth-host", config.NSXT.VMCAuthHost, "vmc-host")
	assertEquals("NSXT.client-auth-cert-file", config.NSXT.ClientAuthCertFile, "client-cert-file")
	assertEquals("NSXT.client-auth-key-file", config.NSXT.ClientAuthKeyFile, "client-key-file")
	assertEquals("NSXT.ca-file", config.NSXT.CAFile, "ca-file")
	assertEquals("NSXT.secret-name", config.NSXT.SecretName, "secret-name")
	assertEquals("NSXT.secret-namespace", config.NSXT.SecretNamespace, "secret-ns")
}

func TestReadConfigINI(t *testing.T) {
	contents := `
[NSXT]
user = admin
password = secret
host = nsxt-server
insecure-flag = true
remote-auth = true
vmc-access-token = vmc-token
vmc-auth-host = vmc-host
client-auth-cert-file = client-cert-file
client-auth-key-file = client-key-file
ca-file = ca-file
secret-name = secret-name
secret-namespace = secret-ns
	`
	config, err := ReadConfigINI([]byte(contents))
	if err != nil {
		t.Error(err)
		return
	}

	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("NSXT.user", config.User, "admin")
	assertEquals("NSXT.password", config.Password, "secret")
	assertEquals("NSXT.host", config.Host, "nsxt-server")
	assert.Equal(t, true, config.InsecureFlag)
	assert.Equal(t, true, config.RemoteAuth)
	assertEquals("NSXT.vmc-access-token", config.VMCAccessToken, "vmc-token")
	assertEquals("NSXT.vmc-auth-host", config.VMCAuthHost, "vmc-host")
	assertEquals("NSXT.client-auth-cert-file", config.ClientAuthCertFile, "client-cert-file")
	assertEquals("NSXT.client-auth-key-file", config.ClientAuthKeyFile, "client-key-file")
	assertEquals("NSXT.ca-file", config.CAFile, "ca-file")
	assertEquals("NSXT.secret-name", config.SecretName, "secret-name")
	assertEquals("NSXT.secret-namespace", config.SecretNamespace, "secret-ns")
}
