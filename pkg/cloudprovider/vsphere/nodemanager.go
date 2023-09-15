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
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
	v1helper "k8s.io/cloud-provider/node/helpers"
	klog "k8s.io/klog/v2"

	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

// Errors
var (
	// ErrVCenterNotFound is returned when the configured vCenter cannot be
	// found.
	ErrVCenterNotFound = errors.New("vCenter not found")

	// ErrDatacenterNotFound is returned when the configured datacenter cannot
	// be found.
	ErrDatacenterNotFound = errors.New("datacenter not found")

	// ErrVMNotFound is returned when the specified VM cannot be found.
	ErrVMNotFound = errors.New("VM not found")
)

type (
	networkConfig struct {
		Ethernets map[string]struct {
			Name      string   `yaml:"set-name"`
			Addresses []string `yaml:"addresses"`
		} `yaml:"ethernets"`
	}
	cloudInitConfig struct {
		Network networkConfig `yaml:"network"`
	}
	encodedCloudInitConfig struct {
		Network string `yaml:"network"`
	}
)

func newNodeManager(cfg *ccfg.CPIConfig, cm *cm.ConnectionManager) *NodeManager {
	return &NodeManager{
		nodeNameMap:       make(map[string]*NodeInfo),
		nodeUUIDMap:       make(map[string]*NodeInfo),
		nodeRegUUIDMap:    make(map[string]*v1.Node),
		vcList:            make(map[string]*VCenterInfo),
		connectionManager: cm,
		cfg:               cfg,
	}
}

// RegisterNode is the handler for when a node is added to a K8s cluster.
func (nm *NodeManager) RegisterNode(node *v1.Node) {
	klog.V(4).Info("RegisterNode ENTER: ", node.Name)

	uuid := ConvertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)
	if err := nm.DiscoverNode(uuid, cm.FindVMByUUID); err != nil {
		klog.Errorf("error discovering node %s: %v", node.Name, err)
		return
	}

	nm.addNode(uuid, node)
	klog.V(4).Info("RegisterNode LEAVE: ", node.Name)
}

// UnregisterNode is the handler for when a node is removed from a K8s cluster.
func (nm *NodeManager) UnregisterNode(node *v1.Node) {
	klog.V(4).Info("UnregisterNode ENTER: ", node.Name)
	uuid := ConvertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)
	nm.removeNode(uuid, node)
	klog.V(4).Info("UnregisterNode LEAVE: ", node.Name)
}

func (nm *NodeManager) addNodeInfo(node *NodeInfo) {
	nm.nodeInfoLock.Lock()
	klog.V(4).Info("addNodeInfo NodeName: ", node.NodeName, ", UUID: ", node.UUID)
	nm.nodeNameMap[node.NodeName] = node
	nm.nodeUUIDMap[node.UUID] = node
	nm.AddNodeInfoToVCList(node.vcServer, node.dataCenter.Name(), node)
	nm.nodeInfoLock.Unlock()
}

func (nm *NodeManager) addNode(uuid string, node *v1.Node) {
	nm.nodeRegInfoLock.Lock()
	klog.V(4).Info("addNode NodeName: ", node.GetName(), ", UID: ", uuid)
	nm.nodeRegUUIDMap[uuid] = node
	nm.nodeRegInfoLock.Unlock()
}

func (nm *NodeManager) removeNode(uuid string, node *v1.Node) {
	nm.nodeRegInfoLock.Lock()
	klog.V(4).Info("removeNode NodeName: ", node.GetName(), ", UID: ", uuid)
	delete(nm.nodeRegUUIDMap, uuid)
	nm.nodeRegInfoLock.Unlock()

	nm.nodeInfoLock.Lock()
	klog.V(4).Info("removeNode from UUID and Name cache. NodeName: ", node.GetName(), ", UID: ", uuid)
	// in case of a race condition that node with same name create happens before delete event,
	// delete the node based on uuid
	name := nm.getNodeNameByUUID(uuid)
	if name != "" {
		delete(nm.nodeNameMap, name)
	} else {
		klog.V(4).Info("node name: ", node.GetName(), " has a different uuid. Skip deleting this node from cache.")
	}
	delete(nm.nodeUUIDMap, uuid)
	nm.nodeInfoLock.Unlock()
}

func (nm *NodeManager) shakeOutNodeIDLookup(ctx context.Context, nodeID string, searchBy cm.FindVM) (*cm.VMDiscoveryInfo, error) {
	// Search by NodeName
	if searchBy == cm.FindVMByName {
		vmDI, err := nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVM(searchBy))
		if err == nil {
			klog.Info("Discovered VM using FQDN or short-hand name")
			return vmDI, nil
		}

		if err != vclib.ErrNoVMFound {
			return nil, err
		}

		vmDI, err = nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVMByIP)
		if err == nil {
			klog.Info("Discovered VM using IP address")
			return vmDI, nil
		}

		klog.Errorf("WhichVCandDCByNodeID failed using VM name. Err: %v", err)
		return nil, err
	}

	// Search by UUID
	vmDI, err := nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVM(searchBy))
	if err == nil {
		klog.Info("Discovered VM using normal UUID format")
		return vmDI, nil
	}

	if err != vclib.ErrNoVMFound {
		return nil, err
	}

	// Need to lookup the original format of the UUID because photon 2.0 formats the UUID
	// different from Photon 3, RHEL, CentOS, Ubuntu, and etc
	klog.Errorf("WhichVCandDCByNodeID failed using normally formatted UUID. Err: %v", err)
	reverseUUID := ConvertK8sUUIDtoNormal(nodeID)
	vmDI, err = nm.connectionManager.WhichVCandDCByNodeID(ctx, reverseUUID, cm.FindVM(searchBy))
	if err == nil {
		klog.Info("Discovered VM using reverse UUID format")
		return vmDI, nil
	}

	klog.Errorf("WhichVCandDCByNodeID failed using UUID. Err: %v", err)
	return nil, err
}

type ipAddrNetworkName struct {
	ipAddr      string
	networkName string
}

func (c *ipAddrNetworkName) ip() net.IP {
	return net.ParseIP(c.ipAddr)
}

// DiscoverNode finds a node's VM using the specified search value and search
// type.
func (nm *NodeManager) DiscoverNode(nodeID string, searchBy cm.FindVM) error {
	ctx := context.Background()

	vmDI, err := nm.shakeOutNodeIDLookup(ctx, nodeID, searchBy)
	if err != nil {
		klog.Errorf("shakeOutNodeIDLookup failed. Err=%v", err)
		return err
	}

	if vmDI.UUID == "" {
		return errors.New("discovered VM UUID is empty")
	}

	var oVM mo.VirtualMachine
	err = vmDI.VM.Properties(ctx, vmDI.VM.Reference(), []string{"guest", "summary", "config"}, &oVM)
	if err != nil {
		klog.Errorf("Error collecting properties for vm=%+v in vc=%s and datacenter=%s: %v",
			vmDI.VM, vmDI.VcServer, vmDI.DataCenter.Name(), err)
		return err
	}

	if oVM.Guest == nil {
		return errors.New("VirtualMachine Guest property was nil")
	}

	if oVM.Guest.HostName == "" {
		return errors.New("VM Guest hostname is empty")
	}

	if len(oVM.Guest.Net) == 0 {
		klog.V(4).Infof("oVM.Guest.Net is empty, skipping node discovery. This could be cauesd by vmtool not reporting correct IP address")
		return errors.New("VM GuestNicInfo is empty")
	}

	tenantRef := vmDI.VcServer
	if vmDI.TenantRef != "" {
		tenantRef = vmDI.TenantRef
	}
	vcInstance := nm.connectionManager.VsphereInstanceMap[tenantRef]

	ipFamilies := []string{vcfg.DefaultIPFamily}
	if vcInstance != nil {
		ipFamilies = vcInstance.Cfg.IPFamilyPriority
	} else {
		klog.Warningf("Unable to find vcInstance for %s. Defaulting to ipv4.", tenantRef)
	}

	var internalNetworkSubnets []*net.IPNet
	var externalNetworkSubnets []*net.IPNet
	var excludeInternalNetworkSubnets []*net.IPNet
	var excludeExternalNetworkSubnets []*net.IPNet
	var internalVMNetworkName string
	var externalVMNetworkName string

	if nm.cfg != nil {
		internalNetworkSubnets, err = parseCIDRs(nm.cfg.Nodes.InternalNetworkSubnetCIDR)
		if err != nil {
			return err
		}
		externalNetworkSubnets, err = parseCIDRs(nm.cfg.Nodes.ExternalNetworkSubnetCIDR)
		if err != nil {
			return err
		}
		excludeInternalNetworkSubnets, err = parseCIDRs(nm.cfg.Nodes.ExcludeInternalNetworkSubnetCIDR)
		if err != nil {
			return err
		}
		excludeExternalNetworkSubnets, err = parseCIDRs(nm.cfg.Nodes.ExcludeExternalNetworkSubnetCIDR)
		if err != nil {
			return err
		}
		internalVMNetworkName = nm.cfg.Nodes.InternalVMNetworkName
		externalVMNetworkName = nm.cfg.Nodes.ExternalVMNetworkName
	}

	addrs := []v1.NodeAddress{}
	klog.V(2).Infof("Adding Hostname: %s", oVM.Guest.HostName)
	v1helper.AddToNodeAddresses(&addrs,
		v1.NodeAddress{
			Type:    v1.NodeHostName,
			Address: oVM.Guest.HostName,
		},
	)

	nonVNICDevices := collectNonVNICDevices(oVM.Guest.Net)
	for _, v := range nonVNICDevices {
		klog.V(6).Infof("internalVMNetworkName = %s", internalVMNetworkName)
		klog.V(6).Infof("externalVMNetworkName = %s", externalVMNetworkName)
		klog.V(6).Infof("v.Network = %s", v.Network)

		if (internalVMNetworkName != "" && !strings.EqualFold(internalVMNetworkName, v.Network)) &&
			(externalVMNetworkName != "" && !strings.EqualFold(externalVMNetworkName, v.Network)) {
			klog.V(4).Infof("Skipping device because vNIC Network=%s doesn't match internal=%s or external=%s network names",
				v.Network, internalVMNetworkName, externalVMNetworkName)
		}
	}

	existingNetworkNames := toNetworkNames(nonVNICDevices)
	if internalVMNetworkName != "" && externalVMNetworkName != "" {
		if !ArrayContainsCaseInsensitive(existingNetworkNames, internalVMNetworkName) &&
			!ArrayContainsCaseInsensitive(existingNetworkNames, externalVMNetworkName) {
			return fmt.Errorf("unable to find suitable IP address for node")
		}
	}

	ipAddrNetworkNames := toIPAddrNetworkNames(nonVNICDevices)
	nonLocalhostIPs := excludeLocalhostIPs(ipAddrNetworkNames)

	if len(nonLocalhostIPs) == 0 {
		klog.V(4).Infof("nonLocalhostIPs is empty")
		klog.V(4).Infof("oVM.Guest.Net=%v", oVM.Guest.Net)
		return fmt.Errorf("unable to find suitable IP address for node after filtering out localhost IPs")
	}

	sortedNonLocalhostIPs, err := sortStaticallyConfiguredAddressesFirst(oVM.Config.ExtraConfig, nonLocalhostIPs)
	if err != nil {
		klog.Errorf("Error sorting statically configured addresses for vm=%+v in vc=%s and datacenter=%s: %v",
			vmDI.VM, vmDI.VcServer, vmDI.DataCenter.Name(), err)
		return err
	}

	for _, ipFamily := range ipFamilies {
		klog.V(6).Infof("ipFamily: %q nonLocalhostIPs: %q", ipFamily, sortedNonLocalhostIPs)
		discoveredInternal, discoveredExternal := discoverIPs(
			sortedNonLocalhostIPs,
			ipFamily,
			internalNetworkSubnets,
			externalNetworkSubnets,
			excludeInternalNetworkSubnets,
			excludeExternalNetworkSubnets,
			internalVMNetworkName,
			externalVMNetworkName,
		)

		klog.V(6).Infof("ipFamily: %q discovered Internal: %q discoveredExternal: %q",
			ipFamily, discoveredInternal, discoveredExternal)

		if discoveredInternal != nil {
			v1helper.AddToNodeAddresses(&addrs,
				v1.NodeAddress{Type: v1.NodeInternalIP, Address: discoveredInternal.ipAddr},
			)
		}

		if discoveredExternal != nil {
			v1helper.AddToNodeAddresses(&addrs,
				v1.NodeAddress{Type: v1.NodeExternalIP, Address: discoveredExternal.ipAddr},
			)
		}

		if len(oVM.Guest.Net) > 0 {
			if discoveredInternal == nil && discoveredExternal == nil {
				klog.V(4).Infof("oVM.Guest.Net=%v", oVM.Guest.Net)
				return fmt.Errorf("unable to find suitable IP address for node %s with IP family %s", nodeID, ipFamilies)
			}
		}
	}

	klog.V(2).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
		nodeID, vmDI.VM, vmDI.VcServer, vmDI.DataCenter.Name())
	klog.V(2).Info("Hostname: ", oVM.Guest.HostName, " UUID: ", vmDI.UUID)

	os := "unknown"
	if g, ok := GuestOSLookup[oVM.Summary.Config.GuestId]; ok {
		os = g
	}

	// store instance type in nodeinfo map
	instanceType := fmt.Sprintf("vsphere-vm.cpu-%d.mem-%dgb.os-%s",
		oVM.Summary.Config.NumCpu,
		(oVM.Summary.Config.MemorySizeMB / 1024),
		os,
	)

	nodeInfo := &NodeInfo{
		tenantRef: tenantRef, dataCenter: vmDI.DataCenter, vm: vmDI.VM, vcServer: vmDI.VcServer,
		UUID: vmDI.UUID, NodeName: vmDI.NodeName, NodeType: instanceType, NodeAddresses: addrs,
	}
	nm.addNodeInfo(nodeInfo)

	return nil
}

// discoverIPs returns a pair of *ipAddrNetworkNames. The first representing
// the internal network IP and the second being the external network IP.
//
// The returned ipAddrNetworkNames will match the given ipFamily.
//
// ipAddrNetworkNames that are contained in the excludeInternalNetworkSubnets
// will never be returned as an internal address, and similarly addresses
// contained in the exludedExternalNetworkSubnets will never be returned
// as an external address - no matter the method of discovery described below.
//
// The returned ipAddrNetworkNames will be selected first by attempting to
// match the given internalNetworkSubnets and externalNetworkSubnets. Subnet
// matching has the highest precedence.
//
// If subnet matches are not found, or if subnets are not provided, then an
// attempt is made to select ipAddrNetworkNames that match the given network
// names. Network name matching has the second highest precedence.
//
// If ipAddrNetworkNames are not found by subnet nor network name matching, then
// the first ipAddrNetworkName of the desired family is returned as both the
// internal and external matches.
//
// If either of these IPs cannot be discovered, nil will be returned instead.
func discoverIPs(ipAddrNetworkNames []*ipAddrNetworkName, ipFamily string,
	internalNetworkSubnets, externalNetworkSubnets,
	excludeInternalNetworkSubnets, excludeExternalNetworkSubnets []*net.IPNet,
	internalVMNetworkName, externalVMNetworkName string,
) (internal *ipAddrNetworkName, external *ipAddrNetworkName) {
	ipFamilyMatches := collectMatchesForIPFamily(ipAddrNetworkNames, ipFamily)

	var discoveredInternal *ipAddrNetworkName
	var discoveredExternal *ipAddrNetworkName

	filteredInternalMatches := filterSubnetExclusions(ipFamilyMatches, excludeInternalNetworkSubnets)
	filteredExternalMatches := filterSubnetExclusions(ipFamilyMatches, excludeExternalNetworkSubnets)

	if len(filteredInternalMatches) > 0 || len(filteredExternalMatches) > 0 {
		discoveredInternal = findSubnetMatch(filteredInternalMatches, internalNetworkSubnets)
		if discoveredInternal != nil {
			klog.V(2).Infof("Adding Internal IP by AddressMatching: %s", discoveredInternal.ipAddr)
		}
		discoveredExternal = findSubnetMatch(filteredExternalMatches, externalNetworkSubnets)
		if discoveredExternal != nil {
			klog.V(2).Infof("Adding External IP by AddressMatching: %s", discoveredExternal.ipAddr)
		}

		if discoveredInternal == nil && internalVMNetworkName != "" {
			discoveredInternal = findNetworkNameMatch(filteredInternalMatches, internalVMNetworkName)
			if discoveredInternal != nil {
				klog.V(2).Infof("Adding Internal IP by NetworkName: %s", discoveredInternal.ipAddr)
			}
		}

		if discoveredExternal == nil && externalVMNetworkName != "" {
			discoveredExternal = findNetworkNameMatch(filteredExternalMatches, externalVMNetworkName)
			if discoveredExternal != nil {
				klog.V(2).Infof("Adding External IP by NetworkName: %s", discoveredExternal.ipAddr)
			}
		}

		// Neither internal or external addresses were found. This defaults to the legacy
		// address selection behavior which is to only support a single address and
		// return the first one found
		if discoveredInternal == nil && discoveredExternal == nil {
			klog.V(5).Info("Default address selection.")
			if len(filteredInternalMatches) > 0 {
				klog.V(2).Infof("Adding Internal IP: %s", filteredInternalMatches[0].ipAddr)
				discoveredInternal = filteredInternalMatches[0]
			}

			if len(filteredExternalMatches) > 0 {
				klog.V(2).Infof("Adding External IP: %s", filteredExternalMatches[0].ipAddr)
				discoveredExternal = filteredExternalMatches[0]
			}
		} else {
			// At least one of the Internal or External addresses has been found.
			// Minimally the Internal needs to exist for the node to function correctly.
			// If only one was discovered, will log the warning and continue which will
			// ultimately be visible to the end user
			if discoveredInternal != nil && discoveredExternal == nil {
				klog.Warning("Internal address found, but external address not found. Returning what addresses were discovered.")
			} else if discoveredInternal == nil && discoveredExternal != nil {
				klog.Warning("External address found, but internal address not found. Returning what addresses were discovered.")
			}
		}
	}
	return discoveredInternal, discoveredExternal
}

// collectNonVNICDevices filters out NICs that are virtual NIC devices. The IPs of
// these NICs should not be added to the node status.
func collectNonVNICDevices(guestNicInfos []types.GuestNicInfo) []types.GuestNicInfo {
	var toReturn []types.GuestNicInfo
	for _, v := range guestNicInfos {
		if v.DeviceConfigId == -1 {
			klog.V(4).Info("Skipping device because not a vNIC")
			continue
		}
		toReturn = append(toReturn, v)
	}
	return toReturn
}

// parseCIDRs converts a comma delimited string of CIDRs to
// a slice of IPNet pointers.
func parseCIDRs(cidrsString string) ([]*net.IPNet, error) {
	if cidrsString != "" {
		cidrStringSlice := strings.Split(cidrsString, ",")
		subnets := make([]*net.IPNet, len(cidrStringSlice))
		for i, cidrString := range cidrStringSlice {
			_, ipNet, err := net.ParseCIDR(cidrString)
			if err != nil {
				return nil, err
			}
			subnets[i] = ipNet
		}
		return subnets, nil
	}
	return nil, nil
}

// toIPAddrNetworkNames maps an array of GuestNicInfo to and array of *ipAddrNetworkName.
func toIPAddrNetworkNames(guestNicInfos []types.GuestNicInfo) []*ipAddrNetworkName {
	var candidates []*ipAddrNetworkName
	for _, v := range guestNicInfos {
		for _, ip := range v.IpAddress {
			candidates = append(candidates, &ipAddrNetworkName{ipAddr: ip, networkName: v.Network})
		}
	}
	return candidates
}

// toNetworkNames maps an array of GuestNicInfo to an array of network name strings
func toNetworkNames(guestNicInfos []types.GuestNicInfo) []string {
	var existingNetworkNames []string
	for _, v := range guestNicInfos {
		existingNetworkNames = append(existingNetworkNames, v.Network)
	}
	return existingNetworkNames
}

// collectMatchesForIPFamily collects all ipAddrNetworkNames that have ips of the
// desired IP family
func collectMatchesForIPFamily(ipAddrNetworkNames []*ipAddrNetworkName, ipFamily string) []*ipAddrNetworkName {
	return filter(ipAddrNetworkNames, func(candidate *ipAddrNetworkName) bool {
		return matchesFamily(candidate.ip(), ipFamily)
	})
}

// matchesFamily detects whether a given IP matches the given IP family.
func matchesFamily(ip net.IP, ipFamily string) bool {
	if ipFamily == vcfg.IPv6Family {
		return ip.To4() == nil && ip.To16() != nil
	}

	if ipFamily == vcfg.IPv4Family {
		return ip.To4() != nil
	}

	return false
}

// filter returns a subset of given ipAddrNetworkNames based on whether the
// items in the collection pass the given predicate function.
func filter(ipAddrNetworkNames []*ipAddrNetworkName, predicate func(*ipAddrNetworkName) bool) []*ipAddrNetworkName {
	var filtered []*ipAddrNetworkName
	for _, item := range ipAddrNetworkNames {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// findSubnetMatch finds the first *ipAddrNetworkName that has an IP in the
// given network subnets.
func findSubnetMatch(ipAddrNetworkNames []*ipAddrNetworkName, networkSubnets []*net.IPNet) *ipAddrNetworkName {
	for _, networkSubnet := range networkSubnets {
		match := findFirst(ipAddrNetworkNames, func(candidate *ipAddrNetworkName) bool {
			return networkSubnet.Contains(candidate.ip())
		})

		if match != nil {
			return match
		}
	}
	return nil
}

// findNetworkNameMatch finds the first *ipAddrNetworkName that matches the
// given network name, ignoring case.
func findNetworkNameMatch(ipAddrNetworkNames []*ipAddrNetworkName, networkName string) *ipAddrNetworkName {
	if networkName != "" {
		return findFirst(ipAddrNetworkNames, func(candidate *ipAddrNetworkName) bool {
			return strings.EqualFold(networkName, candidate.networkName)
		})
	}
	return nil
}

// findFirst returns the first occurance that matches the given predicate
func findFirst(ipAddrNetworkNames []*ipAddrNetworkName, predicate func(*ipAddrNetworkName) bool) *ipAddrNetworkName {
	for _, item := range ipAddrNetworkNames {
		if predicate(item) {
			return item
		}
	}
	return nil
}

// excludeLocalhostIPs collects ipAddrNetworkNames that have valid IPs, ipv4 or
// ipv6, that are not localhost IPs. Localhost IPs should not be added to the
// node status.
func excludeLocalhostIPs(ipAddrNetworkNames []*ipAddrNetworkName) []*ipAddrNetworkName {
	return filter(ipAddrNetworkNames, func(i *ipAddrNetworkName) bool {
		err := ErrOnLocalOnlyIPAddr(i.ipAddr)
		if err != nil {
			klog.V(4).Infof("IP is local only or there was an error. ip=%q err=%v", i.ipAddr, err)
		}
		return err == nil
	})
}

func filterSubnetExclusions(ipAddrNetworkNames []*ipAddrNetworkName, exlusionSubnets []*net.IPNet) []*ipAddrNetworkName {
	return filter(ipAddrNetworkNames, func(i *ipAddrNetworkName) bool {
		for _, exlusionSubnet := range exlusionSubnets {
			if exlusionSubnet.Contains(i.ip()) {
				klog.V(4).Infof("IP is excluded %q because it is contained in exlusion subnet %q", i.ipAddr, exlusionSubnet.String())
				return false
			}
		}
		return true
	})
}

// AddNodeInfoToVCList creates a relational mapping from VC -> DC -> VM/Node
func (nm *NodeManager) AddNodeInfoToVCList(vcenter string, datacenter string, node *NodeInfo) {
	if nm.vcList[vcenter] == nil {
		nm.vcList[vcenter] = &VCenterInfo{
			address: vcenter,
			dcList:  make(map[string]*DatacenterInfo),
		}
	}
	vc := nm.vcList[vcenter]

	if vc.dcList[datacenter] == nil {
		vc.dcList[datacenter] = &DatacenterInfo{
			name:   datacenter,
			vmList: make(map[string]*NodeInfo),
		}
	}
	dc := vc.dcList[datacenter]

	dc.vmList[node.UUID] = node
}

// FindDatacenterInfoInVCList retrieves the DatacenterInfo from the tree
func (nm *NodeManager) FindDatacenterInfoInVCList(vcenter string, datacenter string) (*DatacenterInfo, error) {
	vc := nm.vcList[vcenter]
	if vc == nil {
		return nil, ErrVCenterNotFound
	}

	dc := vc.dcList[datacenter]
	if dc == nil {
		return nil, ErrDatacenterNotFound
	}

	return dc, nil
}

// FindNodeInfo retrieves the NodeInfo from the tree
func (nm *NodeManager) FindNodeInfo(UUID string) (*NodeInfo, error) {
	nm.nodeRegInfoLock.Lock()
	defer nm.nodeRegInfoLock.Unlock()

	UUIDlower := strings.ToLower(UUID)

	if nm.nodeRegUUIDMap[UUIDlower] == nil {
		klog.Errorf("FindNodeInfo( %s ) NOT ACTIVE", UUIDlower)
		return nil, ErrVMNotFound
	}

	nodeInfo := nm.nodeUUIDMap[UUIDlower]
	if nodeInfo == nil {
		klog.Errorf("FindNodeInfo( %s ) NOT FOUND", UUIDlower)
		return nil, ErrVMNotFound
	}

	klog.V(4).Infof("FindNodeInfo( %s ) FOUND", UUIDlower)
	return nodeInfo, nil
}

func (nm *NodeManager) getNodeNameByUUID(UUID string) string {
	for k, v := range nm.nodeNameMap {
		if v.UUID == UUID {
			return k
		}
	}
	return ""
}

func guestInfoMetadata(extraConfig []types.BaseOptionValue) (string, string) {
	var guestInfo, encoding string
	for _, option := range extraConfig {
		value := option.GetOptionValue()
		switch value.Key {
		case "guestinfo.metadata":
			guestInfo, _ = value.Value.(string)
		case "guestinfo.metadata.encoding":
			encoding, _ = value.Value.(string)
		}
	}
	return guestInfo, encoding
}

// sortStaticallyConfiguredAddressesFirst prefers addresses that are from the
// guestInfo but only if they are on a NIC already. It preserves the order in which
// the addresses appear in the guestInfo. For addresses not found in the guestInfo,
// it preserves the order in which they appear in nonlocalhostIPs.
func sortStaticallyConfiguredAddressesFirst(extraConfig []types.BaseOptionValue, nonLocalhostIPs []*ipAddrNetworkName) ([]*ipAddrNetworkName, error) {
	guestInfo, encoding := guestInfoMetadata(extraConfig)

	if guestInfo == "" || encoding != "base64" {
		return nonLocalhostIPs, nil
	}

	value, err := base64.StdEncoding.DecodeString(guestInfo)
	if err != nil {
		return nil, err
	}

	ne := struct {
		NetworkEncoding string `yaml:"network.encoding"`
	}{}
	if err := yaml.Unmarshal(value, &ne); err != nil {
		return nil, err
	}

	var netConfig networkConfig
	switch ne.NetworkEncoding {
	case "base64", "b64":
		var encNetconfig encodedCloudInitConfig
		if err := yaml.Unmarshal(value, &encNetconfig); err != nil {
			return nil, err
		}

		if value, err = base64.RawStdEncoding.DecodeString(encNetconfig.Network); err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(value, &netConfig); err != nil {
			return nil, err
		}
	case "gzip+base64", "gz+b64":
		var encNetconfig encodedCloudInitConfig
		if err := yaml.Unmarshal(value, &encNetconfig); err != nil {
			return nil, err
		}

		gzData, err := base64.RawStdEncoding.DecodeString(encNetconfig.Network)
		if err != nil {
			return nil, err
		}

		r := bytes.NewReader(gzData)
		gr, err := gzip.NewReader(r)
		if err != nil {
			return nil, err
		}

		if value, err = io.ReadAll(gr); err != nil {
			return nil, err
		}

		if err := gr.Close(); err != nil {
			return nil, err
		}

		if err := yaml.Unmarshal(value, &netConfig); err != nil {
			return nil, err
		}
	default: // raw data
		cloudInitCfg := &cloudInitConfig{}
		if err := yaml.Unmarshal(value, cloudInitCfg); err != nil {
			return nil, err
		}
		netConfig = cloudInitCfg.Network
	}

	// Map of guestInfo IP -> index that describes the order they appear in the guestInfo
	guestInfoAddresses := make(map[string]int)
	for _, eth := range netConfig.Ethernets {
		for _, address := range eth.Addresses {
			ip := net.ParseIP(strings.Split(address, "/")[0])
			guestInfoAddresses[ip.String()] = len(guestInfoAddresses)
		}
	}

	// Sort nonlocalhostIPs by the following comparator for two IP addresses: a and b
	// if a is statically configured, but b is not then a should be prioritized before b
	// if b is statically configured, but a is not then a should not be prioritized before b
	// if a and b are both statically configured, then use the index from the guest info
	sort.SliceStable(nonLocalhostIPs, func(i, j int) bool {
		aIndex, aFound := guestInfoAddresses[nonLocalhostIPs[i].ipAddr]
		bIndex, bFound := guestInfoAddresses[nonLocalhostIPs[j].ipAddr]

		return aFound && !bFound || aFound && bFound && aIndex < bIndex
	})

	return nonLocalhostIPs, nil
}
