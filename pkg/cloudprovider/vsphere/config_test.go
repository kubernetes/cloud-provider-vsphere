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

package vsphere

import (
	"strings"
	"testing"
)

const subnetCidrConfig = `
[Nodes]
internal-network-subnet-cidr = "192.0.2.0/24"
external-network-subnet-cidr = "198.51.100.0/24"
`

const networkNameConfig = `
[Nodes]
internal-vm-network-name = "Internal K8s Traffic"
external-vm-network-name = "External/Outbound Traffic"
`

const overrideConfig = `
[Nodes]
internal-network-subnet-cidr = "192.0.2.0/24"
internal-vm-network-name = "Internal K8s Traffic"
external-vm-network-name = "External/Outbound Traffic"
`

func TestReadConfigSubnetCidr(t *testing.T) {
	_, err := ReadCPIConfig(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadCPIConfig(strings.NewReader(subnetCidrConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.InternalNetworkSubnetCIDR != "192.0.2.0/24" {
		t.Errorf("incorrect internal network subnet cidr: %s", cfg.Nodes.InternalNetworkSubnetCIDR)
	}

	if cfg.Nodes.ExternalNetworkSubnetCIDR != "198.51.100.0/24" {
		t.Errorf("incorrect external network subnet cidr: %s", cfg.Nodes.ExternalNetworkSubnetCIDR)
	}
}

func TestReadConfigNetworkName(t *testing.T) {
	_, err := ReadCPIConfig(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadCPIConfig(strings.NewReader(networkNameConfig))
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

func TestEnvOverridesFile(t *testing.T) {
	cfg, err := ReadCPIConfig(strings.NewReader(overrideConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.InternalNetworkSubnetCIDR != "192.0.2.0/24" {
		t.Errorf("expected subnet: \"192.0.2.0/24\", got: \"%s\"", cfg.Nodes.InternalNetworkSubnetCIDR)
	}

	if cfg.Nodes.InternalVMNetworkName != "" {
		t.Errorf("expected vm network name should be empty, got: \"%s\"", cfg.Nodes.InternalVMNetworkName)
	}

	if cfg.Nodes.ExternalVMNetworkName != "External/Outbound Traffic" {
		t.Errorf("expected vm network name should be \"External/Outbound Traffic\", got: \"%s\"", cfg.Nodes.ExternalVMNetworkName)
	}
}
