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
	"strings"
	"testing"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be merged into config_test.go
	and this file should be deleted.
*/

const basicConfigYAML = `
global:
  server: 0.0.0.0
  port: 443
  user: user
  password: password
  insecureFlag: true
  datacenters:
    - us-west
  caFile: /some/path/to/a/ca.pem
`

const multiVCDCsUsingSecretConfigYAML = `
global:
  port: 443
  insecureFlag: true

vcenter:
  tenant1:
    server: 10.0.0.1
    datacenters:
      - vic0dc
    secretName: tenant1-secret
    secretNamespace: kube-system
  tenant2:
    server: 10.0.0.2
    datacenters:
      - vic1dc
    secretName: tenant2-secret
    secretNamespace: kube-system
  10.0.0.3:
    server: 10.0.0.3
    datacenters:
      - vicdc
    secretName: eu-secret
    secretNamespace: kube-system
`

func TestReadConfigYAMLGlobal(t *testing.T) {
	_, err := ReadConfigYAML([]byte(""))
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadConfigYAML([]byte(basicConfigYAML))
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
		t.Errorf("incorrect caFile: %s", cfg.Global.CAFile)
	}
}

func TestTenantRefsYAML(t *testing.T) {
	cfg, err := ReadConfigYAML([]byte(multiVCDCsUsingSecretConfigYAML))
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
