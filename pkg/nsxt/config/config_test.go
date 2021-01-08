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
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromEnv(t *testing.T) {
	cfg := &Config{}
	os.Setenv("NSXT_MANAGER_HOST", "nsxt-server")
	os.Setenv("NSXT_USERNAME", "admin")
	os.Setenv("NSXT_PASSWORD", "secret")
	os.Setenv("NSXT_ALLOW_UNVERIFIED_SSL", "false")
	os.Setenv("NSXT_CLIENT_AUTH_CERT_FILE", "client-cert")
	os.Setenv("NSXT_CLIENT_AUTH_KEY_FILE", "client-key")
	os.Setenv("NSXT_CA_FILE", "ca-cert")
	os.Setenv("NSXT_SECRET_NAME", "secret-name")
	os.Setenv("NSXT_SECRET_NAMESPACE", "secret-ns")

	err := cfg.FromEnv()
	if err != nil {
		t.Errorf("FromEnv failed: %s", err)
	}
	assert.Equal(t, "nsxt-server", cfg.Host)
	assert.Equal(t, "admin", cfg.User)
	assert.Equal(t, "secret", cfg.Password)
	assert.Equal(t, false, cfg.InsecureFlag)
	assert.Equal(t, "client-cert", cfg.ClientAuthCertFile)
	assert.Equal(t, "client-key", cfg.ClientAuthKeyFile)
	assert.Equal(t, "ca-cert", cfg.CAFile)
	assert.Equal(t, "secret-name", cfg.SecretName)
	assert.Equal(t, "secret-ns", cfg.SecretNamespace)

	clearNsxtEnv()
}

func clearNsxtEnv() {
	env := os.Environ()
	for _, pair := range env {
		if strings.HasPrefix(pair, "NSXT_") {
			i := strings.Index(pair, "=")
			os.Unsetenv(pair[:i])
		}
	}
}
