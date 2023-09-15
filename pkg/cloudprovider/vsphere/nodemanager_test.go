/*
Copyright 2018 The Kubernetes Authors.

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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

func TestRegUnregNode(t *testing.T) {
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	connMgr := cm.NewConnectionManager(cfg, nil, nil)
	defer connMgr.Logout()

	nm := newNodeManager(nil, connMgr)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	vm.Guest.HostName = vm.Name
	vm.Guest.Net = []vimtypes.GuestNicInfo{
		{
			Network:   "foo-bar",
			IpAddress: []string{"10.0.0.1"},
		},
	}

	name := vm.Name
	UUID := vm.Config.Uuid
	k8sUUID := ConvertK8sUUIDtoNormal(UUID)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: k8sUUID,
			},
		},
	}

	nm.RegisterNode(node)

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of 1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}
	if len(nm.nodeRegUUIDMap) != 1 {
		t.Errorf("Failed: nodeRegUUIDMap should be a length of  1")
	}

	nm.UnregisterNode(node)

	if len(nm.nodeNameMap) != 0 {
		t.Errorf("Failed: nodeNameMap should be a length of  0")
	}
	if len(nm.nodeUUIDMap) != 0 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  0")
	}
	if len(nm.nodeRegUUIDMap) != 0 {
		t.Errorf("Failed: nodeRegUUIDMap should be a length of 0")
	}
}

func TestDiscoverNodeByName(t *testing.T) {
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	connMgr := cm.NewConnectionManager(cfg, nil, nil)
	defer connMgr.Logout()

	nm := newNodeManager(nil, connMgr)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	vm.Guest.HostName = strings.ToLower(vm.Name) // simulator.SearchIndex.FindByDnsName matches against the guest.hostName property
	vm.Guest.Net = []vimtypes.GuestNicInfo{
		{
			Network:   "foo-bar",
			IpAddress: []string{"10.0.0.1"},
		},
	}
	name := vm.Name

	err := connMgr.Connect(context.Background(), connMgr.VsphereInstanceMap[cfg.Global.VCenterIP])
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	err = nm.DiscoverNode(name, cm.FindVMByName)
	if err != nil {
		t.Errorf("Failed DiscoverNode: %s", err)
	}

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of 1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}
}

func TestDiscoverNodeByNameWithNamesClash(t *testing.T) {
	const vmHostname = "foo.foo.foo"
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	connMgr := cm.NewConnectionManager(cfg, nil, nil)
	defer connMgr.Logout()

	nm := newNodeManager(nil, connMgr)

	vms := simulator.Map.All("VirtualMachine")
	vmOne := vms[0].(*simulator.VirtualMachine)
	vmOne.Guest.HostName = vmHostname
	vmTwo := vms[1].(*simulator.VirtualMachine)
	vmTwo.Guest.HostName = vmHostname

	err := connMgr.Connect(context.Background(), connMgr.VsphereInstanceMap[cfg.Global.VCenterIP])
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	err = nm.DiscoverNode(vmHostname, cm.FindVMByName)
	if err == nil {
		t.Errorf("MiltipleVMFound error expected")
	}

	if err != vclib.ErrMultipleVMsFound {
		t.Errorf("ErrMultipleVMsFound expected, another error occured: %s", err)
	}
}

func TestDiscoverNodeWithMultiIFByName(t *testing.T) {
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	connMgr := cm.NewConnectionManager(cfg, nil, nil)
	defer connMgr.Logout()

	nm := newNodeManager(nil, connMgr)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	vm.Guest.HostName = strings.ToLower(vm.Name) // simulator.SearchIndex.FindByDnsName matches against the guest.hostName property
	expectedIP := "10.10.108.12"
	vm.Guest.Net = []vimtypes.GuestNicInfo{
		{
			Network: "test_k8s_tenant_c123",
			IpAddress: []string{
				"fe80::250:56ff:fe89:d2c7",
			},
		},
		{
			Network: "test_k8s_tenant_c123",
			IpAddress: []string{
				expectedIP,
				"10.10.108.10",
				"fe80::250:56ff:fe89:d2c7",
			},
		},
	}
	name := vm.Name

	err := connMgr.Connect(context.Background(), connMgr.VsphereInstanceMap[cfg.Global.VCenterIP])
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	err = nm.DiscoverNode(name, cm.FindVMByName)
	if err != nil {
		t.Errorf("Failed DiscoverNode: %s", err)
	}

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of 1")
	}

	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
	}

	if nodeInfo, ok := nm.nodeNameMap[strings.ToLower(name)]; ok {
		for _, adr := range nodeInfo.NodeAddresses {
			if adr.Type == "InternalIP" {
				if adr.Address != expectedIP {
					t.Errorf("failed: InternalIP should be %v, not %v.", expectedIP, adr.Address)
				}
			}
			if adr.Type == "ExternalIP" {
				if adr.Address != expectedIP {
					t.Errorf("failed: InternalIP should be %v, not %v.", expectedIP, adr.Address)
				}
			}
		}
	} else {
		t.Errorf("failed: %v not found", name)
	}
}

func TestDiscoverNodeIPs(t *testing.T) {
	type testSetup struct {
		ipFamilyPriority []string
		cpiConfig        *ccfg.CPIConfig
		networks         []vimtypes.GuestNicInfo
		guestinfo        string
	}
	testcases := []struct {
		testName               string
		setup                  testSetup
		expectedIPs            []v1.NodeAddress
		expectedErrorSubstring string
	}{
		{
			testName: "BySubnet",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"20.30.40.50",
							"10.10.1.22",
							"10.10.1.23",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.10"},
			},
		},
		{
			testName: "ByNetworkName",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"127.0.0.6",
							"10.10.1.22",
							"10.10.1.23",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.10"},
			},
		},
		{
			testName: "ByDefaultSelection",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"10.10.1.22",
							"10.10.1.23",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "10.10.1.22"},
			},
		},
		{
			testName: "BySubnetIPv6",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "fd00:cccc::/64",
						ExternalNetworkSubnetCIDR: "fd00:bbbb::/64",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"fe80::1",
							"fd00:aaaa::1",
							"fd00:cccc::1",
							"fd00:cccc::2",
							"fd00:bbbb::1",
							"fd00:bbbb::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:bbbb::1"},
			},
		},
		{
			testName: "ByNetworkNameIPv6",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"fe80::3",
							"fd00:cccc::1",
							"fd00:cccc::2",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"fe80::2",
							"fd00:bbbb::1",
							"fd00:bbbb::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:bbbb::1"},
			},
		},
		{
			testName: "ByDefaultSelectionIPv6",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"fe80::3",
							"fd00:cccc::1",
							"fd00:cccc::2",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"fe80::2",
							"fd00:bbbb::1",
							"fd00:bbbb::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:cccc::1"},
			},
		},
		{
			testName: "ByNetworkNameAndTwoNICs_desiredIPsAfterFirstNIC",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"10.10.10.10",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.10.10"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByMultipleSubnets_dualstack_itSelectsBothIPv4andIPv6Addrs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16,fd00:cccc::/64",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16,fd00:dddd::/64",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_foo",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "net_bar",
						IpAddress: []string{
							"10.10.1.22",
							"fd00:dddd::11",
						},
					},
					{
						Network: "net_baz",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
				{Type: "InternalIP", Address: "fd00:cccc::22"},
				{Type: "ExternalIP", Address: "fd00:dddd::11"},
			},
		},
		{
			testName: "ByMultipleSubnets_dualstack_WhenNoIPsOfFamilyMatchAnySubnets_itFallsThroughToDefaultSelection",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16,fd00:ffff::/64",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16,fd00:eeee::/64",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_foo",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "net_bar",
						IpAddress: []string{
							"10.10.1.22",
							"fd00:dddd::11",
						},
					},
					{
						Network: "net_baz",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
				{Type: "InternalIP", Address: "fd00:dddd::11"},
				{Type: "ExternalIP", Address: "fd00:dddd::11"},
			},
		},
		{
			testName: "ByMultipleSubnets_dualstack_WhenNoIPsOfFamilyMatchesInternalOrExternalSubnets_itUsesSubnetSelectionAndOmitsTheIPThatHasNoMatch",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16,fd00:ffff::/64",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16,fd00:dddd::/64",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_foo",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "net_bar",
						IpAddress: []string{
							"10.10.1.22",
							"fd00:dddd::11",
						},
					},
					{
						Network: "net_baz",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "fd00:dddd::11"},
			},
		},
		{
			testName: "ByMultipleSubnets",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "170.12.0.0/16,10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"10.10.1.22",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "BySubnetAndTwoNICs_desiredIPsAfterFirstNIC",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"10.10.1.22",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "BySubnetAndTwoNICs_desiredIPsAreSplitAcrossNICs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
							"10.10.1.22",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "BySubnet_whenExternalCIDRHasNoMatch_itReturnsOnlyInternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
							"10.10.1.22",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
			},
		},
		{
			testName: "BySubnet_whenInternalCIDRHasNoMatch_itReturnsOnlyExternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
							"172.15.108.11",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByNetworkName_whenInternalNameHasNoMatch_itReturnsOnlyExternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "no-matches",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"10.10.5.8",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"172.15.2.3",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "ExternalIP", Address: "172.15.2.3"},
			},
		},
		{
			testName: "ByNetworkName_whenExternalNameHasNoMatch_itReturnsOnlyInternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "no-matches",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"10.10.5.8",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"172.15.2.3",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.5.8"},
			},
		},
		{
			testName: "BySubnet_whenOnlyExternalCIDRIsSet_itReturnsOnlyExternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"20.30.40.50",
							"10.10.1.22",
							"10.10.1.23",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "ExternalIP", Address: "172.15.108.10"},
			},
		},
		{
			testName: "BySubnet_whenOnlyInternalCIDRIsSet_itReturnsOnlyInternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"20.30.40.50",
							"10.10.1.22",
							"10.10.1.23",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
			},
		},

		{
			testName: "ByNetworkName_selectsIgnoringCase",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "InTerNal_NEt",
						ExternalVMNetworkName: "ExTeRnAL_NeT",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"127.0.0.6",
							"20.30.40.50",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"127.0.0.6",
							"20.30.40.51",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "20.30.40.50"},
				{Type: "ExternalIP", Address: "20.30.40.51"},
			},
		},
		{
			testName: "ByNetworkName_whenOnlyExternalNetworkIsSet_onlyExternalNetIsSet",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						// TODO: update test net names
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"127.0.0.6",
							"10.10.1.22",
							"10.10.1.23",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "ExternalIP", Address: "172.15.108.10"},
			},
		},
		{
			testName: "ByNetworkName_whenOnlyInternalNetworkIsSet_itReturnsOnlyInternalIP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"127.0.0.6",
							"10.10.1.22",
							"10.10.1.23",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.10",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
			},
		},
		{
			testName: "BySubnetAndNetworkNameTwoNICs_desiredIPsAreSplitAcrossNICs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalVMNetworkName:     "test_another_nic",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"127.0.0.6",
							"169.0.1.2",
							"10.10.1.22",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "BySettingBothNetworkNameAndSubnets_SubnetSelectionHasPrecedenceWhenMatchesAreFound",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "10.10.0.0/16",
						ExternalNetworkSubnetCIDR: "172.15.0.0/16",
						InternalVMNetworkName:     "internal_net",
						ExternalVMNetworkName:     "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"22.22.22.22",
							"172.15.108.11",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"33.33.33.33",
							"10.10.1.22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "BySettingBothNetworkNameAndSubnets_whenSubnetsMatchNoIPs_itUsesNetworkNameSelection",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "254.10.0.0/16",
						ExternalNetworkSubnetCIDR: "253.15.0.0/16",
						InternalVMNetworkName:     "internal_net",
						ExternalVMNetworkName:     "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"22.22.22.22",
							"172.15.108.11",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"33.33.33.33",
							"10.10.1.22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "22.22.22.22"},
				{Type: "ExternalIP", Address: "33.33.33.33"},
			},
		},
		{
			testName: "ItIgnoresVNICDevices",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "254.10.0.0/16",
						ExternalNetworkSubnetCIDR: "253.15.0.0/16",
						InternalVMNetworkName:     "internal_net",
						ExternalVMNetworkName:     "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						DeviceConfigId: -1,
						Network:        "vnic-device",
						IpAddress: []string{
							"254.10.1.2",
							"253.15.2.4",
						},
					},
					{
						Network: "internal_net",
						IpAddress: []string{
							"22.22.22.22",
							"172.15.108.11",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"33.33.33.33",
							"10.10.1.22",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "22.22.22.22"},
				{Type: "ExternalIP", Address: "33.33.33.33"},
			},
		},
		{
			testName: "BySettingANetworkNameThatDoesntExist",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"10.10.1.22",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedErrorSubstring: "unable to find suitable IP address for node",
		},
		{
			testName: "ByDiscoveringAnUnParsableIP_itIsIgnored",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_123abc",
						IpAddress: []string{
							"blarg",
							"127.0.0.6",
							"10.10.1.22",
							"10.10.1.23",
						},
					},
					{
						Network: "test_another_nic",
						IpAddress: []string{
							"127.0.0.7",
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.1.22"},
				{Type: "ExternalIP", Address: "10.10.1.22"},
			},
		},
		{
			testName: "ByDefaultSelection_whenTheSecondNICHasNoIPs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
					{
						Network:   "net_b",
						IpAddress: []string{},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByDefaultSelection_whenTheFirstNICHasNoIPs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network:   "net_a",
						IpAddress: []string{},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByDefaultSelection_whenTheFirstNICHasNoIPsOfTheDesiredFamily",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"fd00:cccc::1",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"172.15.108.11",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByDefaultSelection_TheSecondNICHasNoIPsOfTheDesiredFamily",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"172.15.108.11",
							"fe80:cccc::1",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"fe80:cccc::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByDefaultSelection_whenDualStackIPv4Primary_itReturnsIPv4AddrsFirst",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::1",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"fd00:cccc::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:cccc::1"},
			},
		},
		{
			testName: "ByDefaultSelection_itDoesNotSelectIPsFromtheExclusionCIDRList",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						ExcludeInternalNetworkSubnetCIDR: "172.15.108.11/32,fd00:cccc::1/128,fd00:cccc::2/128",
						ExcludeExternalNetworkSubnetCIDR: "172.15.108.11/32,172.15.108.12/32,fd00:cccc::1/128",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"172.15.108.11",
							"172.15.108.12",
							"172.15.108.13",
							"fd00:cccc::1",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"fd00:cccc::2",
							"fd00:cccc::3",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.12"},
				{Type: "ExternalIP", Address: "172.15.108.13"},
				{Type: "InternalIP", Address: "fd00:cccc::3"},
				{Type: "ExternalIP", Address: "fd00:cccc::2"},
			},
		},
		{
			testName: "ByDefaultSelection_DualStackIPv6Primary_itReturnsIPv6AddrsFirst",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6", "ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "net_a",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::1",
						},
					},
					{
						Network: "net_b",
						IpAddress: []string{
							"fd00:cccc::2",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:cccc::1"},
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.11"},
			},
		},
		{
			testName: "ByNetworkName_whenDualStack",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6", "ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "internal_net",
						ExternalVMNetworkName: "external_net",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::1",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"fd00:cccc::2",
							"172.15.108.12",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd00:cccc::1"},
				{Type: "ExternalIP", Address: "fd00:cccc::2"},
				{Type: "InternalIP", Address: "172.15.108.11"},
				{Type: "ExternalIP", Address: "172.15.108.12"},
			},
		},
		{
			testName: "BySubnet_itDoesNotSelectIPsFromtheExclusionCIDRList",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "172.15.0.0/16,fd00:cccc::0/32",
						ExternalNetworkSubnetCIDR: "173.15.0.0/16,fd01:cccc::0/32",

						ExcludeInternalNetworkSubnetCIDR: "172.15.108.11/32,fd00:cccc::1/128,fd00:cccc::2/128",
						ExcludeExternalNetworkSubnetCIDR: "173.15.108.11/32,173.15.108.12/32,fd01:cccc::1/128",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"172.15.108.11",
							"172.15.108.12",
							"172.15.108.13",
							"fd00:cccc::1",
							"fd00:cccc::2",
							"fd00:cccc::3",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"173.15.108.11",
							"173.15.108.12",
							"173.15.108.13",
							"fd01:cccc::1",
							"fd01:cccc::2",
							"fd01:cccc::3",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.12"},
				{Type: "ExternalIP", Address: "173.15.108.13"},
				{Type: "InternalIP", Address: "fd00:cccc::3"},
				{Type: "ExternalIP", Address: "fd01:cccc::2"},
			},
		},
		{
			testName: "ByNetworkName_itDoesNotSelectIPsFromtheExclusionCIDRList",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4", "ipv6"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName:            "internal_net",
						ExternalVMNetworkName:            "external_net",
						ExcludeInternalNetworkSubnetCIDR: "172.15.108.11/32,fd00:cccc::1/128,fd00:cccc::2/128",
						ExcludeExternalNetworkSubnetCIDR: "173.15.108.11/32,173.15.108.12/32,fd01:cccc::1/128",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"172.15.108.11",
							"172.15.108.12",
							"172.15.108.13",
							"fd00:cccc::1",
							"fd00:cccc::2",
							"fd00:cccc::3",
						},
					},
					{
						Network: "external_net",
						IpAddress: []string{
							"173.15.108.11",
							"173.15.108.12",
							"173.15.108.13",
							"fd01:cccc::1",
							"fd01:cccc::2",
							"fd01:cccc::3",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "172.15.108.12"},
				{Type: "ExternalIP", Address: "173.15.108.13"},
				{Type: "InternalIP", Address: "fd00:cccc::3"},
				{Type: "ExternalIP", Address: "fd01:cccc::2"},
			},
		},
		{
			testName: "Dualstack_ExcludingSubnets_whenNoIPv4AddrIsDiscovered",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6", "ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						ExcludeInternalNetworkSubnetCIDR: "172.15.108.11/8",
						ExcludeExternalNetworkSubnetCIDR: "172.15.108.11/8",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::1",
						},
					},
				},
			},
			expectedErrorSubstring: "unable to find suitable IP address for node",
		},
		{
			testName: "Dualstack_ExcludingSubnets_whenNoIPv6AddrIsDiscovered",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6", "ipv4"},
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						ExcludeInternalNetworkSubnetCIDR: "fd00:cccc::1/16",
						ExcludeExternalNetworkSubnetCIDR: "fd00:cccc::1/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"172.15.108.11",
							"fd00:cccc::1",
						},
					},
				},
			},
			expectedErrorSubstring: "unable to find suitable IP address for node",
		},
		{
			testName: "DualStack_whenNoIPsOfOneFamilyAreDiscovered",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6", "ipv4"},
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"127.0.0.1",
							"fd00:cccc::1",
						},
					},
				},
			},
			expectedErrorSubstring: "unable to find suitable IP address for node",
		},
		{
			testName: "IPv6_guestInfoWithDHCP",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				guestinfo:        guestInfoWithIPv6DHCP(),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"fe80::1",
							"fd01:1234::1",
							"fd01:cccc::1",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd01:1234::1"},
				{Type: "ExternalIP", Address: "fd01:1234::1"},
			},
		},
		{
			testName: "StaticAddresses_IPv6_usesStaticAddressForExternalInternal",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				guestinfo:        guestInfoWithAddresses("fd01:cccc::1/128"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"fe80::1",
							"fd01:1234::1",
							"fd01:cccc::1",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd01:cccc::1"},
				{Type: "ExternalIP", Address: "fd01:cccc::1"},
			},
		},
		{
			testName: "StaticAddresses_IPv4_usesStaticAddressForExternalInternal",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("192.168.1.12/64"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
							"192.168.1.12",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.12"},
				{Type: "ExternalIP", Address: "192.168.1.12"},
			},
		},
		{
			testName: "StaticAddresses_prioritizesOrderFromAddresses",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("192.168.1.12/64,192.168.1.10/64"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
							"192.168.1.12",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.12"},
				{Type: "ExternalIP", Address: "192.168.1.12"},
			},
		},
		{
			testName: "StaticAddresses_usesTheStaticAddressInTheNetworkCIDR",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("10.10.10.10/64,192.168.1.12/64"),
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalNetworkSubnetCIDR: "192.168.0.0/16",
						ExternalNetworkSubnetCIDR: "192.168.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
							"192.168.1.12",
							"10.10.10.10",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.12"},
				{Type: "ExternalIP", Address: "192.168.1.12"},
			},
		},
		{
			testName: "StaticAddresses_ignoresStaticAddressWhenWithinExcludeCIDR",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("192.168.1.12/64,10.10.10.10/64"),
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						ExcludeInternalNetworkSubnetCIDR: "192.168.0.0/16",
						ExcludeExternalNetworkSubnetCIDR: "192.168.0.0/16",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
							"192.168.1.12",
							"10.10.10.10",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "10.10.10.10"},
				{Type: "ExternalIP", Address: "10.10.10.10"},
			},
		},
		{
			testName: "StaticAddresses_usesTheStaticAddressInTheConfiguredNetworkName",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("192.168.1.8/64,192.168.1.12/64,10.10.10.10/64"),
				cpiConfig: &ccfg.CPIConfig{
					Nodes: ccfg.Nodes{
						InternalVMNetworkName: "VM Network",
						ExternalVMNetworkName: "VM Network",
					},
				},
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "internal_net",
						IpAddress: []string{
							"192.168.1.8",
						},
					},
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
							"192.168.1.12",
							"10.10.10.10",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.12"},
				{Type: "ExternalIP", Address: "192.168.1.12"},
			},
		},
		{
			testName: "StaticAddresses_addressesAreNotAssignedToTheNIC",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv4"},
				guestinfo:        guestInfoWithAddresses("192.168.1.12/64,10.10.10.10/64"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.8",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "192.168.1.8"},
				{Type: "ExternalIP", Address: "192.168.1.8"},
			},
		},
		{
			testName: "StaticAddresses_IPv6_handlesShorthandVsLonghandAddrs",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				guestinfo:        guestInfoWithAddresses("fd01:1:2:2919:abba:0000:0000:401/128"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"fd00::1",
							"fd01:1:2:2919:abba::401",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd01:1:2:2919:abba::401"},
				{Type: "ExternalIP", Address: "fd01:1:2:2919:abba::401"},
			},
		},
		{
			testName: "StaticAddresses_IPv6_usesNetworkB64EncodedStaticAddressForExternalInternal",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				guestinfo:        guestInfoEncodedNetconfigWithAddresses("gzip+base64", "fd01:cccc::1/128"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"fe80::1",
							"fd01:1234::1",
							"fd01:cccc::1",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd01:cccc::1"},
				{Type: "ExternalIP", Address: "fd01:cccc::1"},
			},
		},
		{
			testName: "StaticAddresses_IPv6_usesNetworkGZB64EncodedStaticAddressForExternalInternal",
			setup: testSetup{
				ipFamilyPriority: []string{"ipv6"},
				guestinfo:        guestInfoEncodedNetconfigWithAddresses("base64", "fd01:cccc::1/128"),
				cpiConfig:        nil,
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"fe80::1",
							"fd01:1234::1",
							"fd01:cccc::1",
						},
					},
				},
			},
			expectedIPs: []v1.NodeAddress{
				{Type: "InternalIP", Address: "fd01:cccc::1"},
				{Type: "ExternalIP", Address: "fd01:cccc::1"},
			},
		},
		{
			testName: "StaticAddresses_errorsOnInvalidGuestInfoFormat",
			setup: testSetup{
				guestinfo: "not-valid-yaml this should error",
				networks: []vimtypes.GuestNicInfo{
					{
						Network: "VM Network",
						IpAddress: []string{
							"192.168.1.10",
						},
					},
				},
			},
			expectedErrorSubstring: "cannot unmarshal",
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.testName, func(t *testing.T) {
			cfg, fin := configFromEnvOrSim(true)
			defer fin()

			cfg.VirtualCenter[cfg.Global.VCenterIP].IPFamilyPriority = testcase.setup.ipFamilyPriority
			connMgr := cm.NewConnectionManager(cfg, nil, nil)
			defer connMgr.Logout()

			nm := newNodeManager(testcase.setup.cpiConfig, connMgr)

			vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
			vm.Guest.HostName = strings.ToLower(vm.Name) // simulator.SearchIndex.FindByDnsName matches against the guest.hostName property
			vm.Guest.Net = testcase.setup.networks
			if testcase.setup.guestinfo != "" {
				vm.Config.ExtraConfig = []vimtypes.BaseOptionValue{
					&vimtypes.OptionValue{
						Key:   "guestinfo.metadata",
						Value: base64.StdEncoding.EncodeToString([]byte(testcase.setup.guestinfo)),
					},
					&vimtypes.OptionValue{
						Key:   "guestinfo.metadata.encoding",
						Value: "base64",
					},
				}
			}

			name := vm.Name

			err := connMgr.Connect(context.Background(), connMgr.VsphereInstanceMap[cfg.Global.VCenterIP])
			if err != nil {
				t.Errorf("Failed to Connect to vSphere: %s", err)
			}

			// subject
			err = nm.DiscoverNode(name, cm.FindVMByName)
			if testcase.expectedErrorSubstring != "" {
				if err == nil {
					t.Errorf("failed: expected DiscoverNode to return error containing: %q but no error occurred", testcase.expectedErrorSubstring)
					return
				}
				if !strings.Contains(err.Error(), testcase.expectedErrorSubstring) {
					t.Errorf("failed: expected DiscoverNode to return error containing: %q but was %q", testcase.expectedErrorSubstring, err.Error())
				}
				return
			} else if err != nil {
				t.Errorf("Failed DiscoverNode: %s", err)
				return
			}

			nodeInfo, ok := nm.nodeNameMap[strings.ToLower(name)]
			if !ok {
				t.Errorf("failed: %v not found", name)
			}

			// hostname is always returned first, then the expected ips
			expectations := append(
				[]v1.NodeAddress{{Type: "Hostname", Address: strings.ToLower(vm.Name)}},
				testcase.expectedIPs...,
			)
			if len(nodeInfo.NodeAddresses) != len(expectations) {
				t.Errorf("failed: nodeInfo.NodeAddresses should be length %d but was %d", len(testcase.expectedIPs)+1, len(nodeInfo.NodeAddresses))
			}
			for i, nodeAddress := range expectations {
				if nodeInfo.NodeAddresses[i].Address != nodeAddress.Address {
					t.Errorf("failed: NodeAddresses[%d].Address should eq %q but was %q", i, nodeAddress.Address, nodeInfo.NodeAddresses[i].Address)
				}
				if nodeInfo.NodeAddresses[i].Type != nodeAddress.Type {
					t.Errorf("failed: NodeAddresses[%d].Type should eq %q but was %q", i, nodeAddress.Type, nodeInfo.NodeAddresses[i].Type)
				}
			}
		})
	}
}

func TestCollectNonVNICDevices(t *testing.T) {
	guestNicInfos := []vimtypes.GuestNicInfo{
		{DeviceConfigId: 10},
		{DeviceConfigId: -1},
	}

	returnedGuestNicInfos := collectNonVNICDevices(guestNicInfos)

	if len(returnedGuestNicInfos) != 1 {
		t.Errorf("failed: expected one GuestNicInfo, got %d", len(returnedGuestNicInfos))
	}

	if returnedGuestNicInfos[0].DeviceConfigId != 10 {
		t.Errorf("failed: expected GuestNicInfo.DeviceConfigId to equal 10 but was %d", returnedGuestNicInfos[0].DeviceConfigId)
	}
}

func TestToIPAddrNetworkNames(t *testing.T) {
	guestNicInfos := []vimtypes.GuestNicInfo{
		{Network: "internal_net", IpAddress: []string{"192.168.1.1", "fd00:1:4::1"}},
		{Network: "external_net", IpAddress: []string{"10.10.50.12", "fd00:100:64::1"}},
	}

	actual := toIPAddrNetworkNames(guestNicInfos)

	if len(actual) != 4 {
		t.Errorf("failed: expected four returned ipAddrNetworkNames, got: %d", len(actual))
	}

	if actual[0].networkName != "internal_net" || actual[0].ipAddr != "192.168.1.1" {
		t.Errorf("failed: expected the first entry to have a networkName of \"internal_net\" and a ipAddr of \"192.168.1.1\", but got: %s %s", actual[0].networkName, actual[0].ipAddr)
	}

	if actual[1].networkName != "internal_net" || actual[1].ipAddr != "fd00:1:4::1" {
		t.Errorf("failed: expected the first entry to have a networkName of \"internal_net\" and a ipAddr of \"fd00:1:4::1\", but got: %s %s", actual[1].networkName, actual[1].ipAddr)
	}

	if actual[2].networkName != "external_net" || actual[2].ipAddr != "10.10.50.12" {
		t.Errorf("failed: expected the first entry to have a networkName of \"external_net\" and a ipAddr of \"10.10.50.12\", but got: %s %s", actual[2].networkName, actual[2].ipAddr)
	}

	if actual[3].networkName != "external_net" || actual[3].ipAddr != "fd00:100:64::1" {
		t.Errorf("failed: expected the first entry to have a networkName of \"external_net\" and a ipAddr of \"fd00:100:64::1\", but got: %s %s", actual[3].networkName, actual[3].ipAddr)
	}
}

func TestToNetworkNames(t *testing.T) {
	guestNicInfos := []vimtypes.GuestNicInfo{
		{Network: "internal_net"},
		{Network: "external_net"},
	}

	actual := toNetworkNames(guestNicInfos)

	if len(actual) != 2 {
		t.Errorf("failed: expected two returned network names: %d", len(actual))
	}

	if actual[0] != "internal_net" {
		t.Errorf("failed: expected the first entry to equal of \"internal_net\", but got: %s ", actual[0])
	}

	if actual[1] != "external_net" {
		t.Errorf("failed: expected the first entry to equal of \"external_net\", but got: %s ", actual[1])
	}
}

func TestCollectMatchesForIPFamily(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		{ipAddr: "192.168.1.1"},
		{ipAddr: "fd00:100:64::1"},
	}

	ipv4IPAddrs := collectMatchesForIPFamily(ipAddrNetworkNames, "ipv4")

	if len(ipv4IPAddrs) != 1 {
		t.Errorf("failed: expected one ipv4 match, but got: %d", len(ipv4IPAddrs))
	}

	if ipv4IPAddrs[0].ipAddr != "192.168.1.1" {
		t.Errorf("failed: expected ipAddr to equal \"192.168.1.1\", but got: %s", ipv4IPAddrs[0].ipAddr)
	}

	ipv6IPAddrs := collectMatchesForIPFamily(ipAddrNetworkNames, "ipv6")

	if len(ipv6IPAddrs) != 1 {
		t.Errorf("failed: expected one ipv6 match, but got: %d", len(ipv4IPAddrs))
	}

	if ipv6IPAddrs[0].ipAddr != "fd00:100:64::1" {
		t.Errorf("failed: expected ipAddr to equal \"fd00:100:64::1\", but got: %s", ipv6IPAddrs[0].ipAddr)
	}
}

func TestMatchesFamily(t *testing.T) {
	if !matchesFamily(net.ParseIP("192.168.1.1"), "ipv4") {
		t.Errorf("failed: expected 192.168.1.1 to match ipFamily ipv4, but it did not")
	}

	if matchesFamily(net.ParseIP("192.168.1.1"), "ipv6") {
		t.Errorf("failed: expected 192.168.1.1 not to match ipFamily ipv6, but it did")
	}

	if !matchesFamily(net.ParseIP("fd00:1::1"), "ipv6") {
		t.Errorf("failed: expected fd00:1::1to match ipFamily ipv6, but it did not")
	}

	if matchesFamily(net.ParseIP("fd00:1::1"), "ipv4") {
		t.Errorf("failed: expected fd00:1::1 not to match ipFamily ipv4, but it did")
	}

	if matchesFamily(net.ParseIP("garbage"), "ipv6") {
		t.Errorf("failed: expected garbage not to match ipFamily ipv6, but it did")
	}

	if matchesFamily(net.ParseIP("garbage"), "ipv4") {
		t.Errorf("failed: expected garbage not to match ipFamily ipv4, but it did")
	}

	if matchesFamily(net.ParseIP("fd00:1::1"), "ipv7") {
		t.Errorf("failed: expected fd00:1::1 not to match ipFamily ipv7, but it did")
	}

	if matchesFamily(net.ParseIP("192.168.1.1"), "ipv7") {
		t.Errorf("failed: expected 192.168.1.1 not to match ipFamily ipv7, but it did")
	}
}

func TestFilter(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		{networkName: "foo"},
		{networkName: "bar"},
	}

	actual := filter(ipAddrNetworkNames, func(n *ipAddrNetworkName) bool {
		return n.networkName == "foo"
	})

	if len(actual) != 1 {
		t.Errorf("failed: expected one ipAddrNetworkName, but got: %d", len(actual))
	}

	if actual[0].networkName != "foo" {
		t.Errorf("failed: expected filtered network name to be \"foo\", but got %s", actual[0].networkName)
	}
}

func TestFindSubnetMatch(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		{ipAddr: "192.168.1.1"},
		{ipAddr: "10.10.1.2"},
		{ipAddr: "10.10.1.3"},
	}

	_, ipNetA, err := net.ParseCIDR("10.11.0.0/16")
	if err != nil {
		t.Errorf("failed to parse CIDR")
	}
	_, ipNetB, err := net.ParseCIDR("10.10.0.0/16")
	if err != nil {
		t.Errorf("failed to parse CIDR")
	}

	actual := findSubnetMatch(ipAddrNetworkNames, []*net.IPNet{ipNetA, ipNetB})

	if actual.ipAddr != "10.10.1.2" {
		t.Errorf("failed: expected ipAddr to equal 10.10.1.2, but was %s", actual.ipAddr)
	}

	ipAddrNetworkNames = []*ipAddrNetworkName{
		{ipAddr: "fc11::1"},
		{ipAddr: "fd00:100:64::1"},
		{ipAddr: "fd00:100:64::2"},
	}

	_, ipNet, err := net.ParseCIDR("fd00:100:64::/64")
	if err != nil {
		t.Errorf("failed to parse CIDR")
	}

	actual = findSubnetMatch(ipAddrNetworkNames, []*net.IPNet{ipNet})

	if actual.ipAddr != "fd00:100:64::1" {
		t.Errorf("failed: expected ipAddr to equal fd00:100:64::1, but was %s", actual.ipAddr)
	}

	ipAddrNetworkNames = []*ipAddrNetworkName{
		{ipAddr: "fc11::1"},
		{ipAddr: "fd00:101:64::2"},
		{ipAddr: "fd00:100:64::1"},
		{ipAddr: "fd00:100:64::2"},
	}

	_, ipNet1, err := net.ParseCIDR("fd00:100:64::/64")
	if err != nil {
		t.Errorf("failed to parse CIDR")
	}

	_, ipNet2, err := net.ParseCIDR("fd00:101:64::/64")
	if err != nil {
		t.Errorf("failed to parse CIDR")
	}

	actual = findSubnetMatch(ipAddrNetworkNames, []*net.IPNet{ipNet1, ipNet2})

	if actual.ipAddr != "fd00:100:64::1" {
		t.Errorf("failed: expected ipAddr to equal fd00:100:64::1, but was %s", actual.ipAddr)
	}
}

func TestFindFirst(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		{networkName: "foo", ipAddr: "::1"},
		{networkName: "bar", ipAddr: "::2"},
		{networkName: "baz", ipAddr: "::3"},
	}

	actual := findFirst(ipAddrNetworkNames, func(i *ipAddrNetworkName) bool {
		return i.networkName == "bar"
	})

	if actual.networkName != "bar" {
		t.Errorf("failed: expected ipAddr to have name 'bar', but was %s", actual.networkName)
	}
}

func TestFindNetworkNameMatch(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		{networkName: "foo", ipAddr: "::1"},
		{networkName: "bar", ipAddr: "::1"},
		{networkName: "bar", ipAddr: "192.168.1.1"},
	}

	match := findNetworkNameMatch(ipAddrNetworkNames, "bar")

	if match.networkName != "bar" || match.ipAddr != "::1" {
		t.Errorf("failed: expected a match of name \"bar\" with an ipAddr of \"::1\", but got: %s %s", match.networkName, match.ipAddr)
	}
}

func TestExcludeLocalhostIPs(t *testing.T) {
	ipAddrNetworkNames := []*ipAddrNetworkName{
		// doesn't parse
		{ipAddr: "garbage"},
		// unspecified
		{ipAddr: "0.0.0.0"},
		{ipAddr: "::"},
		// link local multicast
		{ipAddr: "224.0.0.1"},
		{ipAddr: "ff02::1"},
		// link local unicast
		{ipAddr: "169.254.0.1"},
		{ipAddr: "fe80::1"},
		// loopback
		{ipAddr: "127.0.0.1"},
		{ipAddr: "::1"},

		{ipAddr: "192.168.1.1"},
		{ipAddr: "fd00:100:64::1"},
	}

	actual := excludeLocalhostIPs(ipAddrNetworkNames)

	if len(actual) != 2 {
		t.Errorf("failure: expected non localhosts matches to have len 2, but was %d", len(actual))
	}

	if actual[0].ipAddr != "192.168.1.1" {
		t.Errorf("failure: expected ipAddr to equal 192.168.1.1, but was %s", actual[0].ipAddr)
	}

	if actual[1].ipAddr != "fd00:100:64::1" {
		t.Errorf("failure: expected ipAddr to equal fd00:100:64::1, but was %s", actual[1].ipAddr)
	}
}

func guestInfoWithIPv6DHCP() string {
	return `instance-id: "tkg-mgmt-vc"
local-hostname: "tkg-mgmt-vc"
wait-on-network:
  ipv4: false
  ipv6: false
network:
  version: 2
  ethernets:
    id0:
      match:
        macaddress: "00:11:22"
      set-name: "eth0"
      wakeonlan: true
      dhcp4: false
      dhcp6: true`
}

func guestInfoWithAddresses(addresses string) string {
	return fmt.Sprintf(`instance-id: "tkg-mgmt-vc"
local-hostname: "tkg-mgmt-vc"
wait-on-network:
  ipv4: false
  ipv6: false
network:
  version: 2
  ethernets:
    id0:
      addresses: [%s]
      match:
        macaddress: "00:11:22"
      set-name: "eth0"
      wakeonlan: true
      dhcp4: false
      dhcp6: false`,
		addresses)
}

func guestInfoEncodedNetconfigWithAddresses(encoding, addresses string) string {
	var (
		networkConfig = []byte(fmt.Sprintf(`version: 2
ethernets:
  id0:
    addresses: [%s]
    match:
    macaddress: "00:11:22"
    set-name: "eth0"
    wakeonlan: true
    dhcp4: false
    dhcp6: false`,
			addresses))

		encodedNetconfig string
	)

	switch encoding {
	case "base64":
		encodedNetconfig = base64.StdEncoding.EncodeToString(networkConfig)
	case "gzip+base64":
		buf := bytes.NewBuffer(nil)
		gw := gzip.NewWriter(buf)
		if _, err := gw.Write(networkConfig); err != nil {
			return err.Error()
		}
		if err := gw.Close(); err != nil {
			return err.Error()
		}
		encodedNetconfig = base64.StdEncoding.EncodeToString(buf.Bytes())
	default:
		return guestInfoWithAddresses(addresses)
	}

	return fmt.Sprintf(`instance-id: "tkg-mgmt-vc"
local-hostname: "tkg-mgmt-vc"
wait-on-network:
  ipv4: false
  ipv6: false
network.encoding: %s
network: %s`,
		encoding, encodedNetconfig)
}
