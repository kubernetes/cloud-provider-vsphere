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

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

const basicConfigINI = `
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem
`

const multiVCDCsUsingSecretConfigINI = `
[Global]
port = 443
insecure-flag = true

[VirtualCenter "tenant1"]
server = "10.0.0.1"
datacenters = "vic0dc"
secret-name = "tenant1-secret"
secret-namespace = "kube-system"
# port, insecure-flag will be used from Global section.

[VirtualCenter "tenant2"]
server = "10.0.0.2"
datacenters = "vic1dc"
secret-name = "tenant2-secret"
secret-namespace = "kube-system"
# port, insecure-flag will be used from Global section.

[VirtualCenter "10.0.0.3"]
datacenters = "vicdc"
secret-name = "eu-secret"
secret-namespace = "kube-system"
# port, insecure-flag will be used from Global section.
`

func TestReadConfigINIGlobal(t *testing.T) {
	_, err := ReadConfigINI([]byte(""))
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadConfigINI([]byte(basicConfigINI))
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

/*
TODO: move to global
func TestBlankEnvFails(t *testing.T) {
	cfg := &ConfigINI{}

	err := cfg.FromEnv()
	if err == nil {
		t.Fatalf("Env only config should fail if env not set")
	}
}

func TestEnvOverridesFile(t *testing.T) {
	ip := "127.0.0.1"
	os.Setenv("VSPHERE_VCENTER", ip)
	defer os.Unsetenv("VSPHERE_VCENTER")

	cfg, err := ReadConfigINI([]byte(basicConfigINI))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Global.VCenterIP != ip {
		t.Errorf("expected IP: %s, got: %s", ip, cfg.Global.VCenterIP)
	}
}
*/

func TestIPFamiliesINI(t *testing.T) {
	vcci := VirtualCenterConfigINI{}

	vcci.IPFamily = "ipv6"
	err := vcci.validateIPFamily()
	if err != nil {
		t.Errorf("Valid ipv6 but yielded err: %s", err)
	}
	size := len(vcci.IPFamilyPriority)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	vcci.IPFamily = "ipv4"
	err = vcci.validateIPFamily()
	if err != nil {
		t.Errorf("Valid ipv4 but yielded err: %s", err)
	}
	size = len(vcci.IPFamilyPriority)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	vcci.IPFamily = "ipv4, "
	err = vcci.validateIPFamily()
	if err != nil {
		t.Errorf("Valid ipv4, but yielded err: %s", err)
	}
	size = len(vcci.IPFamilyPriority)
	if size != 1 {
		t.Errorf("Invalid family list expected: 1, actual: %d", size)
	}

	vcci.IPFamily = "ipv6,ipv4"
	err = vcci.validateIPFamily()
	if err != nil {
		t.Errorf("Valid ipv6/ipv4 but yielded err: %s", err)
	}
	size = len(vcci.IPFamilyPriority)
	if size != 2 {
		t.Errorf("Invalid family list expected: 2, actual: %d", size)
	}

	vcci.IPFamily = "ipv7"
	err = vcci.validateIPFamily()
	if err == nil {
		t.Errorf("Invalid ipv7 but successful")
	}

	vcci.IPFamily = "ipv4,ipv7"
	err = vcci.validateIPFamily()
	if err == nil {
		t.Errorf("Invalid ipv4,ipv7 but successful")
	}
}

func TestTenantRefsINI(t *testing.T) {
	cfg, err := ReadConfigINI([]byte(multiVCDCsUsingSecretConfigINI))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	vcConfig1 := cfg.VirtualCenter["tenant1"]
	if vcConfig1 == nil {
		t.Fatalf("Should return a valid vcConfig1")
	}
	if !strings.EqualFold(vcConfig1.VCenterIP, "10.0.0.1") {
		t.Errorf("vcConfig1 VCenterIP should be 10.0.0.1 but actual=%s", vcConfig1.VCenterIP)
	}
	if !strings.EqualFold(vcConfig1.TenantRef, "tenant1") {
		t.Errorf("vcConfig1 TenantRef should be tenant1 but actual=%s", vcConfig1.TenantRef)
	}
	if !strings.EqualFold(vcConfig1.SecretRef, "kube-system/tenant1-secret") {
		t.Errorf("vcConfig1 SecretRef should be kube-system/tenant1-secret but actual=%s", vcConfig1.SecretRef)
	}

	vcConfig2 := cfg.VirtualCenter["tenant2"]
	if vcConfig2 == nil {
		t.Fatalf("Should return a valid vcConfig2")
	}
	if !strings.EqualFold(vcConfig2.VCenterIP, "10.0.0.2") {
		t.Errorf("vcConfig2 VCenterIP should be 10.0.0.2 but actual=%s", vcConfig2.VCenterIP)
	}
	if !strings.EqualFold(vcConfig2.TenantRef, "tenant2") {
		t.Errorf("vcConfig2 TenantRef should be tenant2 but actual=%s", vcConfig2.TenantRef)
	}
	if !strings.EqualFold(vcConfig2.SecretRef, "kube-system/tenant2-secret") {
		t.Errorf("vcConfig2 SecretRef should be kube-system/tenant2-secret but actual=%s", vcConfig2.SecretRef)
	}

	vcConfig3 := cfg.VirtualCenter["10.0.0.3"]
	if vcConfig3 == nil {
		t.Fatalf("Should return a valid vcConfig3")
	}
	if !strings.EqualFold(vcConfig3.VCenterIP, "10.0.0.3") {
		t.Errorf("vcConfig3 VCenterIP should be 10.0.0.3 but actual=%s", vcConfig3.VCenterIP)
	}
	if !strings.EqualFold(vcConfig3.TenantRef, "10.0.0.3") {
		t.Errorf("vcConfig3 TenantRef should be eu-secret but actual=%s", vcConfig3.TenantRef)
	}
	if !strings.EqualFold(vcConfig3.SecretRef, "kube-system/eu-secret") {
		t.Errorf("vcConfig3 SecretRef should be kube-system/eu-secret but actual=%s", vcConfig3.SecretRef)
	}
}
