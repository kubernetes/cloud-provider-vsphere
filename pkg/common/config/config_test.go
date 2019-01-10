/*
Copyright 2016 The Kubernetes Authors.

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
	"strings"
	"testing"
)

func TestReadConfigGlobal(t *testing.T) {
	cfg := Config{}

	err := ReadConfig(&cfg, nil)
	if err == nil {
		t.Errorf("Should fail when no config file is provided: %s", err)
	}
	err = ReadConfig(nil, strings.NewReader(""))
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	err = ReadConfig(&cfg, strings.NewReader(`
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem
`))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Global.VCenterIP != "0.0.0.0" {
		t.Errorf("incorrect vcenter ip: %s", cfg.Global.VCenterIP)
	}

	if cfg.Global.Datacenters != "us-west" {
		t.Errorf("incorrect datacenter: %s", cfg.Global.Datacenters)
	}

	if cfg.Global.CAFile != "/some/path/to/a/ca.pem" {
		t.Errorf("incorrect ca-file: %s", cfg.Global.CAFile)
	}
}
