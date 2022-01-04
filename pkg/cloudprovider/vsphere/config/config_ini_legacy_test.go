/*
Copyright 2019 The Kubernetes Authors.
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
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

const subnetCidrINIConfig = `
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem

[Nodes]
internal-network-subnet-cidr = "192.0.2.0/24"
external-network-subnet-cidr = "198.51.100.0/24"
`

const networkNameINIConfig = `
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem

[Nodes]
internal-vm-network-name = "Internal K8s Traffic"
external-vm-network-name = "External/Outbound Traffic"
`

const excludeSubnetINIConfig = `
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem

[Nodes]
exclude-internal-network-subnet-cidr = "192.0.2.0/24,fe80::1/128"
exclude-external-network-subnet-cidr = "192.1.2.0/24,fe80::2/128"
`

func TestReadINIConfigSubnetCidr(t *testing.T) {
	_, err := ReadCPIConfigINI(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadCPIConfigINI([]byte(subnetCidrINIConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Global.VCenterIP != "0.0.0.0" {
		t.Errorf("incorrect global vcServerIP: %s", cfg.Global.VCenterIP)
	}

	if cfg.Nodes.InternalNetworkSubnetCIDR != "192.0.2.0/24" {
		t.Errorf("incorrect internal network subnet cidr: %s", cfg.Nodes.InternalNetworkSubnetCIDR)
	}
	if cfg.Nodes.ExternalNetworkSubnetCIDR != "198.51.100.0/24" {
		t.Errorf("incorrect external network subnet cidr: %s", cfg.Nodes.ExternalNetworkSubnetCIDR)
	}
}

func TestReadINIConfigNetworkName(t *testing.T) {
	_, err := ReadCPIConfigINI(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadCPIConfigINI([]byte(networkNameINIConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.InternalVMNetworkName != "Internal K8s Traffic" {
		t.Errorf("incorrect internal vm network name: %s", cfg.Nodes.InternalVMNetworkName)
	}

	if cfg.Nodes.ExternalVMNetworkName != "External/Outbound Traffic" {
		t.Errorf("incorrect internal vm network name: %s", cfg.Nodes.ExternalVMNetworkName)
	}
}

func TestReadINIConfigExcludeSubnetCidr(t *testing.T) {
	cfg, err := ReadCPIConfigINI([]byte(excludeSubnetINIConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.ExcludeInternalNetworkSubnetCIDR != "192.0.2.0/24,fe80::1/128" {
		t.Errorf("incorrect exclude internal network subnet cidrs: %s", cfg.Nodes.ExcludeInternalNetworkSubnetCIDR)
	}

	if cfg.Nodes.ExcludeExternalNetworkSubnetCIDR != "192.1.2.0/24,fe80::2/128" {
		t.Errorf("incorrect exclude external network subnet cidrs: %s", cfg.Nodes.ExcludeExternalNetworkSubnetCIDR)
	}
}
