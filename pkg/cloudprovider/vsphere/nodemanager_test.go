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
	expected_ip := "10.10.108.12"
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
				expected_ip,
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
				if adr.Address != expected_ip {
					t.Errorf("failed: InternalIP should be %v, not %v.", expected_ip, adr.Address)
				}
			}
			if adr.Type == "ExternalIP" {
				if adr.Address != expected_ip {
					t.Errorf("failed: InternalIP should be %v, not %v.", expected_ip, adr.Address)
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
