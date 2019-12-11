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
	"os"
	"strings"
	"testing"
)

const basicConfig = `
[Nodes]
internal-network-subnet-cidr = 192.0.2.0/24
external-network-subnet-cidr = 198.51.100.0/24
`

func TestReadConfigGlobal(t *testing.T) {
	_, err := ReadCPIConfig(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := ReadCPIConfig(strings.NewReader(basicConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.InternalNetworkSubnetCIDR != "192.0.2.0/24" {
		t.Errorf("incorrect vcenter ip: %s", cfg.Nodes.InternalNetworkSubnetCIDR)
	}

	if cfg.Nodes.ExternalNetworkSubnetCIDR != "198.51.100.0/24" {
		t.Errorf("incorrect datacenter: %s", cfg.Nodes.ExternalNetworkSubnetCIDR)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	subnet := "203.0.113.0/24"
	os.Setenv("VSPHERE_NODES_INTERNAL_NETWORK_SUBNET_CIDR", subnet)
	defer os.Unsetenv("VSPHERE_NODES_INTERNAL_NETWORK_SUBNET_CIDR")

	cfg, err := ReadCPIConfig(strings.NewReader(basicConfig))
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Nodes.InternalNetworkSubnetCIDR != subnet {
		t.Errorf("expected subnet: \"%s\", got: \"%s\"", subnet, cfg.Nodes.InternalNetworkSubnetCIDR)
	}
}
