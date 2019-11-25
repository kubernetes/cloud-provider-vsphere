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
	"errors"
	"fmt"
	"net"
	"strings"

	v1 "k8s.io/api/core/v1"
	clientv1 "k8s.io/client-go/listers/core/v1"
	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	v1helper "k8s.io/cloud-provider/node/helpers"
	"k8s.io/klog"

	"github.com/vmware/govmomi/vim25/mo"
)

// Errors
var (
	// ErrVCenterNotFound is returned when the configured vCenter cannot be
	// found.
	ErrVCenterNotFound = errors.New("vCenter not found")

	// ErrDatacenterNotFound is returned when the configured datacenter cannot
	// be found.
	ErrDatacenterNotFound = errors.New("Datacenter not found")

	// ErrVMNotFound is returned when the specified VM cannot be found.
	ErrVMNotFound = errors.New("VM not found")
)

func newNodeManager(cm *cm.ConnectionManager, lister clientv1.NodeLister) *NodeManager {
	return &NodeManager{
		nodeNameMap:       make(map[string]*NodeInfo),
		nodeUUIDMap:       make(map[string]*NodeInfo),
		vcList:            make(map[string]*VCenterInfo),
		connectionManager: cm,
		nodeLister:        lister,
	}
}

// RegisterNode is the handler for when a node is added to a K8s cluster.
func (nm *NodeManager) RegisterNode(node *v1.Node) {
	klog.V(4).Info("RegisterNode ENTER: ", node.Name)
	uuid := ConvertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)
	nm.DiscoverNode(uuid, cm.FindVMByUUID)
	klog.V(4).Info("RegisterNode LEAVE: ", node.Name)
}

// UnregisterNode is the handler for when a node is removed from a K8s cluster.
func (nm *NodeManager) UnregisterNode(node *v1.Node) {
	klog.V(4).Info("UnregisterNode ENTER: ", node.Name)
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

func (nm *NodeManager) shakeOutNodeIDLookup(ctx context.Context, nodeID string, searchBy cm.FindVM) (*cm.VMDiscoveryInfo, error) {
	// Search by NodeName
	if searchBy == cm.FindVMByName {
		vmDI, err := nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVM(searchBy))
		if err == nil {
			klog.Info("Discovered VM using FQDN or short-hand name")
			return vmDI, err
		}

		vmDI, err = nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVMByIP)
		if err == nil {
			klog.Info("Discovered VM using IP address")
			return vmDI, err
		}

		klog.Errorf("WhichVCandDCByNodeID failed using VM name. Err: %v", err)
		return nil, err
	}

	// Search by UUID
	vmDI, err := nm.connectionManager.WhichVCandDCByNodeID(ctx, nodeID, cm.FindVM(searchBy))
	if err == nil {
		klog.Info("Discovered VM using normal UUID format")
		return vmDI, err
	}

	// Need to lookup the original format of the UUID because photon 2.0 formats the UUID
	// different from Photon 3, RHEL, CentOS, Ubuntu, and etc
	klog.Errorf("WhichVCandDCByNodeID failed using normally formatted UUID. Err: %v", err)
	reverseUUID := ConvertK8sUUIDtoNormal(nodeID)
	vmDI, err = nm.connectionManager.WhichVCandDCByNodeID(ctx, reverseUUID, cm.FindVM(searchBy))
	if err == nil {
		klog.Info("Discovered VM using reverse UUID format")
		return vmDI, err
	}

	klog.Errorf("WhichVCandDCByNodeID failed using UUID. Err: %v", err)
	return nil, err
}

func returnIPsFromSpecificFamily(family string, ips []string) []string {
	var matching []string

	for _, ip := range ips {
		if err := ErrOnLocalOnlyIPAddr(ip); err != nil {
			klog.V(4).Infof("IP is local only or there was an error. ip=%q err=%v", ip, err)
			continue
		}

		if strings.EqualFold(family, vcfg.IPv6Family) && net.ParseIP(ip).To4() == nil {
			matching = append(matching, ip)
		} else if strings.EqualFold(family, vcfg.IPv4Family) && net.ParseIP(ip).To4() != nil {
			matching = append(matching, ip)
		}
	}

	return matching
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

	var oVM mo.VirtualMachine
	err = vmDI.VM.Properties(ctx, vmDI.VM.Reference(), []string{"guest", "summary"}, &oVM)
	if err != nil {
		klog.Errorf("Error collecting properties for vm=%+v in vc=%s and datacenter=%s: %v",
			vmDI.VM, vmDI.VcServer, vmDI.DataCenter.Name(), err)
		return err
	}

	tenantRef := vmDI.VcServer
	if vmDI.TenantRef != "" {
		tenantRef = vmDI.TenantRef
	}
	vcInstance := nm.connectionManager.VsphereInstanceMap[tenantRef]

	ipFamily := []string{vcfg.DefaultIPFamily}
	if vcInstance != nil {
		ipFamily = vcInstance.Cfg.IPFamilyPriority
	} else {
		klog.Warningf("Unable to find vcInstance for %s. Defaulting to ipv4.", tenantRef)
	}

	var internalNetworkSubnet *net.IPNet
	var externalNetworkSubnet *net.IPNet

	if nm.cpiCfg != nil {
		if nm.cpiCfg.Nodes.InternalNetworkSubnetCIDR != "" {
			_, internalNetworkSubnet, err = net.ParseCIDR(nm.cpiCfg.Nodes.InternalNetworkSubnetCIDR)
			if err != nil {
				return err
			}
		}
		if nm.cpiCfg.Nodes.ExternalNetworkSubnetCIDR != "" {
			_, externalNetworkSubnet, err = net.ParseCIDR(nm.cpiCfg.Nodes.ExternalNetworkSubnetCIDR)
			if err != nil {
				return err
			}
		}
	}

	var addressMatchingEnabled bool
	if internalNetworkSubnet != nil || externalNetworkSubnet != nil {
		addressMatchingEnabled = true
	}

	found := false
	addrs := []v1.NodeAddress{}
	for _, v := range oVM.Guest.Net {
		if v.DeviceConfigId == -1 {
			klog.V(4).Info("Skipping device because not a vNIC")
			continue
		}

		// Only return a single IP address based on the preference of IPFamily
		// Must break out of loop in the event of ipv6,ipv4 where the NIC does
		// contain a valid IPv6 and IPV4 address
		for _, family := range ipFamily {
			ips := returnIPsFromSpecificFamily(family, v.IpAddress)

			if addressMatchingEnabled {
				klog.V(2).Infof("Adding Hostname: %s", oVM.Guest.HostName)
				v1helper.AddToNodeAddresses(&addrs,
					v1.NodeAddress{
						Type:    v1.NodeHostName,
						Address: oVM.Guest.HostName,
					},
				)

				var internalIP string
				var externalIP string
				for _, ip := range ips {
					parsedIP := net.ParseIP(ip)
					if parsedIP == nil {
						return fmt.Errorf("can't parse IP: %s", ip)
					}

					if internalIP == "" && internalNetworkSubnet != nil && internalNetworkSubnet.Contains(parsedIP) {
						internalIP = ip
					}

					if externalIP == "" && externalNetworkSubnet != nil && externalNetworkSubnet.Contains(parsedIP) {
						externalIP = ip
					}
				}

				if internalIP != "" {
					klog.V(2).Infof("Adding Internal IP: %s", internalIP)
					v1helper.AddToNodeAddresses(&addrs,
						v1.NodeAddress{
							Type:    v1.NodeInternalIP,
							Address: internalIP,
						},
					)
					found = true
				}
				if externalIP != "" {
					klog.V(2).Infof("Adding External IP: %s", externalIP)
					v1helper.AddToNodeAddresses(&addrs,
						v1.NodeAddress{
							Type:    v1.NodeExternalIP,
							Address: externalIP,
						},
					)
					found = true
				}

			} else {
				for _, ip := range ips {
					klog.V(2).Infof("Adding IP: %s", ip)
					v1helper.AddToNodeAddresses(&addrs,
						v1.NodeAddress{
							Type:    v1.NodeExternalIP,
							Address: ip,
						}, v1.NodeAddress{
							Type:    v1.NodeInternalIP,
							Address: ip,
						}, v1.NodeAddress{
							Type:    v1.NodeHostName,
							Address: oVM.Guest.HostName,
						},
					)
					found = true
					break
				}
			}

			if found {
				break
			}
		}
	}

	if !found {
		klog.Warningf("Unable to find a suitable IP address. ipFamily: %s", ipFamily)
	}

	klog.V(2).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
		nodeID, vmDI.VM, vmDI.VcServer, vmDI.DataCenter.Name())
	klog.V(2).Info("Hostname: ", oVM.Guest.HostName, " UUID: ", oVM.Summary.Config.Uuid)

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

	nodeInfo := &NodeInfo{tenantRef: tenantRef, dataCenter: vmDI.DataCenter, vm: vmDI.VM, vcServer: vmDI.VcServer,
		UUID: vmDI.UUID, NodeName: vmDI.NodeName, NodeType: instanceType, NodeAddresses: addrs}
	nm.addNodeInfo(nodeInfo)

	return nil
}

// ExportNodes transforms the NodeInfoList to []*pb.Node
func (nm *NodeManager) ExportNodes(vcenter string, datacenter string, nodeList *[]*pb.Node) error {
	nm.nodeInfoLock.Lock()

	if vcenter != "" && datacenter != "" {
		dc, err := nm.FindDatacenterInfoInVCList(vcenter, datacenter)
		if err != nil {
			nm.nodeInfoLock.Unlock()
			return err
		}

		nm.datacenterToNodeList(dc.vmList, nodeList)
	} else if vcenter != "" {
		if nm.vcList[vcenter] == nil {
			nm.nodeInfoLock.Unlock()
			return ErrVCenterNotFound
		}

		for _, dc := range nm.vcList[vcenter].dcList {
			nm.datacenterToNodeList(dc.vmList, nodeList)
		}
	} else {
		for _, vc := range nm.vcList {
			for _, dc := range vc.dcList {
				nm.datacenterToNodeList(dc.vmList, nodeList)
			}
		}
	}

	nm.nodeInfoLock.Unlock()

	return nil
}

func (nm *NodeManager) datacenterToNodeList(vmList map[string]*NodeInfo, nodeList *[]*pb.Node) {
	for _, node := range vmList {
		pbNode := &pb.Node{
			Vcenter:    node.vcServer,
			Datacenter: node.dataCenter.Name(),
			Name:       node.NodeName,
			Dnsnames:   make([]string, 0),
			Addresses:  make([]string, 0),
			Uuid:       node.UUID,
		}
		for _, address := range node.NodeAddresses {
			switch address.Type {
			case v1.NodeExternalIP:
				pbNode.Addresses = append(pbNode.Addresses, address.Address)
			case v1.NodeHostName:
				pbNode.Dnsnames = append(pbNode.Dnsnames, address.Address)
			default:
				klog.Warning("Unknown/unsupported address type:", address.Type)
			}
		}
		*nodeList = append(*nodeList, pbNode)
	}
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

// FindNodeInfoInVCList retrieves the NodeInfo from the tree
func (nm *NodeManager) FindNodeInfoInVCList(vcenter string, datacenter string, UUID string) (*NodeInfo, error) {
	dc, err := nm.FindDatacenterInfoInVCList(vcenter, datacenter)
	if err != nil {
		return nil, err
	}

	vm := dc.vmList[UUID]
	if vm == nil {
		return nil, ErrVMNotFound
	}

	return vm, nil
}
