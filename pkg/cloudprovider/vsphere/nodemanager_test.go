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
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
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

	if len(nm.nodeNameMap) != 1 {
		t.Errorf("Failed: nodeNameMap should be a length of  1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Errorf("Failed: nodeUUIDMap should be a length of  1")
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

func TestExport(t *testing.T) {
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

	nodeList := make([]*pb.Node, 0)
	_ = nm.ExportNodes("", "", &nodeList)

	found := false
	for _, node := range nodeList {
		if node.Uuid == UUID {
			found = true
		}
	}

	if !found {
		t.Errorf("Node was not converted to protobuf")
	}

	nm.UnregisterNode(node)
}

func TestReturnIPsFromSpecificFamily(t *testing.T) {
	ipFamilies := []string{
		"10.161.34.192",
		"fd01:0:101:2609:bdd2:ee20:7bd7:5836",
		"fe80::98b5:4834:27a8:c58d",
	}

	ips := returnIPsFromSpecificFamily(vcfg.IPv6Family, ipFamilies)
	size := len(ips)
	if size != 1 {
		t.Errorf("Should only return single IPv6 address. expected: 1, actual: %d", size)
	} else if !strings.EqualFold(ips[0], "fd01:0:101:2609:bdd2:ee20:7bd7:5836") {
		t.Errorf("IPv6 does not match. expected: fd01:0:101:2609:bdd2:ee20:7bd7:5836, actual: %s", ips[0])
	}

	ips = returnIPsFromSpecificFamily(vcfg.IPv4Family, ipFamilies)
	size = len(ips)
	if size != 1 {
		t.Errorf("Should only return single IPv4 address. expected: 1, actual: %d", size)
	} else if !strings.EqualFold(ips[0], "10.161.34.192") {
		t.Errorf("IPv6 does not match. expected: 10.161.34.192, actual: %s", ips[0])
	}
}

func TestDiscoverNodeIPs(t *testing.T) {
	type testSetup struct {
		ipFamilyPriority []string
		cpiConfig        *ccfg.CPIConfig
		networks         []vimtypes.GuestNicInfo
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
			testName: "BySubnet_selectsIgnoringCase",
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
