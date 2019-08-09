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
	"os"
	"strings"
	"testing"
)

const basicConfig = `
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem
`

func TestReadConfigGlobal(t *testing.T) {
	_, err := ReadConfig(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadConfig(strings.NewReader(basicConfig))
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

func TestEnvOverridesFile(t *testing.T) {
	ip := "127.0.0.1"
	os.Setenv("VSPHERE_VCENTER", ip)
	defer os.Unsetenv("VSPHERE_VCENTER")

	cfg, err := ReadConfig(strings.NewReader(basicConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Global.VCenterIP != ip {
		t.Errorf("expected IP: %s, got: %s", ip, cfg.Global.VCenterIP)
	}
}

func TestBlankEnvFails(t *testing.T) {
	err := FromEnv(&Config{})
	if err == nil {
		t.Fatalf("Env only config should fail if env not set")
	}
}

func TestIPFamilies(t *testing.T) {
	input := "ipv6"
	ipFamilies, err := validateIPFamily(input)
	if err != nil {
		t.Errorf("Valid ipv6 but yielded err: %s", err)
	}
	size := len(ipFamilies)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	input = "ipv4"
	ipFamilies, err = validateIPFamily(input)
	if err != nil {
		t.Errorf("Valid ipv4 but yielded err: %s", err)
	}
	size = len(ipFamilies)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	input = "ipv4, "
	ipFamilies, err = validateIPFamily(input)
	if err != nil {
		t.Errorf("Valid ipv4, but yielded err: %s", err)
	}
	size = len(ipFamilies)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	input = "ipv6,ipv4"
	ipFamilies, err = validateIPFamily(input)
	if err != nil {
		t.Errorf("Valid ipv6/ipv4 but yielded err: %s", err)
	}
	size = len(ipFamilies)
	if size != 2 {
		t.Errorf("Invalid family list expected: 2, actual: %d", size)
	}

	input = "ipv7"
	_, err = validateIPFamily(input)
	if err == nil {
		t.Errorf("Invalid ipv7 but successful")
	}

	input = "ipv4,ipv7"
	_, err = validateIPFamily(input)
	if err == nil {
		t.Errorf("Invalid ipv4,ipv7 but successful")
	}
}
