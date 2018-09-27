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
	"sync"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	pb "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/proto"
	"k8s.io/cloud-provider-vsphere/pkg/vclib"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"

	"github.com/vmware/govmomi/vim25/mo"
)

type FindVM int

const (
	FindVMByUUID FindVM = iota // 0
	FindVMByName               // 1

	// Error Messages
	VCenterNotFoundErrMsg    = "vCenter not found"
	DatacenterNotFoundErrMsg = "Datacenter not found"
	VMNotFoundErrMsg         = "VM not found"
)

// Error constants
var (
	ErrVCenterNotFound    = errors.New(VCenterNotFoundErrMsg)
	ErrDatacenterNotFound = errors.New(DatacenterNotFoundErrMsg)
	ErrVMNotFound         = errors.New(VMNotFoundErrMsg)
)

// RegisterNode - Handler when node is removed from k8s cluster.
func (nm *NodeManager) RegisterNode(node *v1.Node) {
	glog.V(4).Info("RegisterNode ENTER: ", node.Name)
	nm.addNode(node)
	nm.DiscoverNode(nm.convertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID), FindVMByUUID)
	glog.V(4).Info("RegisterNode LEAVE: ", node.Name)
}

// UnregisterNode - Handler when node is removed from k8s cluster.
func (nm *NodeManager) UnregisterNode(node *v1.Node) {
	glog.V(4).Info("UnregisterNode ENTER: ", node.Name)
	nm.removeNode(node)
	glog.V(4).Info("UnregisterNode LEAVE: ", node.Name)
}

func (nm *NodeManager) addNodeInfo(node *NodeInfo) {
	nm.nodeInfoLock.Lock()
	glog.V(4).Info("addNodeInfo NodeName: ", node.NodeName, ", UUID: ", node.UUID)
	nm.nodeNameMap[node.NodeName] = node
	nm.nodeUUIDMap[node.UUID] = node
	nm.AddNodeInfoToVCList(node.vcServer, node.dataCenter.Name(), node)
	nm.nodeInfoLock.Unlock()
}

func (nm *NodeManager) addNode(node *v1.Node) {
	nm.nodeRegInfoLock.Lock()
	uuid := nm.convertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)
	glog.V(4).Info("addNode NodeName: ", node.GetName(), ", UID: ", uuid)
	nm.nodeRegUUIDMap[uuid] = node
	nm.nodeRegInfoLock.Unlock()
}

func (nm *NodeManager) removeNode(node *v1.Node) {
	nm.nodeRegInfoLock.Lock()
	uuid := nm.convertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)
	glog.V(4).Info("removeNode NodeName: ", node.GetName(), ", UID: ", uuid)
	delete(nm.nodeRegUUIDMap, uuid)
	nm.nodeRegInfoLock.Unlock()
}

func (nm *NodeManager) DiscoverNode(nodeID string, searchBy FindVM) error {
	if nodeID == "" {
		glog.V(3).Info("DiscoverNode called but nodeID is empty")
		return vclib.ErrNoVMFound
	}
	type vmSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *vmSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *vmSearch, QUEUE_SIZE)

	myNodeID := nodeID
	if searchBy == FindVMByUUID {
		glog.V(3).Info("DiscoverNode by UUID")
		myNodeID = strings.ToLower(nodeID)
	} else {
		glog.V(3).Info("DiscoverNode by Name")
	}
	glog.V(2).Info("DiscoverNode nodeID: ", myNodeID)

	vmFound := false
	globalErr = nil

	setGlobalErr := func(err error) {
		globalErrMutex.Lock()
		globalErr = &err
		globalErrMutex.Unlock()
	}

	setVMFound := func(found bool) {
		mutex.Lock()
		vmFound = found
		mutex.Unlock()
	}

	getVMFound := func() bool {
		mutex.Lock()
		found := vmFound
		mutex.Unlock()
		return found
	}

	go func() {
		var datacenterObjs []*vclib.Datacenter
		for vc, vsi := range nm.vsphereInstanceMap {

			found := getVMFound()
			if found == true {
				break
			}

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := nm.vcConnect(ctx, vsi)
			if err != nil {
				glog.Error("Discovering node error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.conn)
				if err != nil {
					glog.Error("Discovering node error dc:", err)
					setGlobalErr(err)
					continue
				}
			} else {
				datacenters := strings.Split(vsi.cfg.Datacenters, ",")
				for _, dc := range datacenters {
					dc = strings.TrimSpace(dc)
					if dc == "" {
						continue
					}
					datacenterObj, err := vclib.GetDatacenter(ctx, vsi.conn, dc)
					if err != nil {
						glog.Error("Discovering node error dc:", err)
						setGlobalErr(err)
						continue
					}
					datacenterObjs = append(datacenterObjs, datacenterObj)
				}
			}

			for _, datacenterObj := range datacenterObjs {
				found := getVMFound()
				if found == true {
					break
				}

				glog.V(4).Infof("Finding node %s in vc=%s and datacenter=%s", myNodeID, vc, datacenterObj.Name())
				queueChannel <- &vmSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	for i := 0; i < POOL_SIZE; i++ {
		go func() {
			for res := range queueChannel {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				var vm *vclib.VirtualMachine
				var err error
				if searchBy == FindVMByUUID {
					vm, err = res.datacenter.GetVMByUUID(ctx, myNodeID)
				} else {
					vm, err = res.datacenter.GetVMByDNSName(ctx, myNodeID)
				}

				if err != nil {
					glog.Error("Error while looking for vm=%+v in vc=%s and datacenter=%s: %v",
						vm, res.vc, res.datacenter.Name(), err)
					if err != vclib.ErrNoVMFound {
						setGlobalErr(err)
					} else {
						glog.V(2).Infof("Did not find node %s in vc=%s and datacenter=%s",
							myNodeID, res.vc, res.datacenter.Name())
					}
					continue
				}

				var oVM mo.VirtualMachine
				err = vm.Properties(ctx, vm.Reference(), []string{"config", "summary", "summary.config", "guest.net", "guest"}, &oVM)
				if err != nil {
					glog.Error("Error collecting properties for vm=%+v in vc=%s and datacenter=%s: %v",
						vm, res.vc, res.datacenter.Name(), err)
					continue
				}

				addrs := []v1.NodeAddress{}
				for _, v := range oVM.Guest.Net {
					for _, ip := range v.IpAddress {
						if net.ParseIP(ip).To4() != nil {
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
						}
					}
				}

				glog.V(2).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
					nodeID, vm, res.vc, res.datacenter.Name())
				glog.V(2).Info("Hostname: ", oVM.Guest.HostName, " UUID: ", oVM.Summary.Config.Uuid)

				nodeInfo := &NodeInfo{dataCenter: res.datacenter, vm: vm, vcServer: res.vc,
					UUID: oVM.Summary.Config.Uuid, NodeName: oVM.Guest.HostName, NodeAddresses: addrs}
				nm.addNodeInfo(nodeInfo)
				for range queueChannel {
				}
				setVMFound(true)
				break
			}
			wg.Done()
		}()
		wg.Add(1)
	}
	wg.Wait()
	if vmFound {
		return nil
	}
	if globalErr != nil {
		return *globalErr
	}

	glog.V(4).Infof("Discovery Node: %q vm not found", myNodeID)
	return vclib.ErrNoVMFound
}

// vcConnect connects to vCenter with existing credentials
// If credentials are invalid:
// 		1. It will fetch credentials from credentialManager
//      2. Update the credentials
//		3. Connects again to vCenter with fetched credentials
func (nm *NodeManager) vcConnect(ctx context.Context, vsphereInstance *VSphereInstance) error {
	err := vsphereInstance.conn.Connect(ctx)
	if err == nil {
		return nil
	}

	if !vclib.IsInvalidCredentialsError(err) || nm.credentialManager == nil {
		glog.Errorf("Cannot connect to vCenter with err: %v", err)
		return err
	}

	glog.V(2).Infof("Invalid credentials. Cannot connect to server %q. "+
		"Fetching credentials from secrets.", vsphereInstance.conn.Hostname)

	// Get latest credentials from SecretCredentialManager
	credentials, err := nm.credentialManager.GetCredential(vsphereInstance.conn.Hostname)
	if err != nil {
		glog.Error("Failed to get credentials from Secret Credential Manager with err:", err)
		return err
	}
	vsphereInstance.conn.UpdateCredentials(credentials.User, credentials.Password)
	return vsphereInstance.conn.Connect(ctx)
}

// Reformats UUID to match vSphere format
// Endian Safe : https://www.dmtf.org/standards/smbios/
//            8   -  4 -  4 - 4  -    12
//K8s:    56492e42-22ad-3911-6d72-59cc8f26bc90
//VMware: 422e4956-ad22-1139-6d72-59cc8f26bc90
func (nm *NodeManager) convertK8sUUIDtoNormal(k8sUUID string) string {
	uuid := fmt.Sprintf("%s%s%s%s-%s%s-%s%s-%s-%s",
		k8sUUID[6:8], k8sUUID[4:6], k8sUUID[2:4], k8sUUID[0:2],
		k8sUUID[11:13], k8sUUID[9:11],
		k8sUUID[16:18], k8sUUID[14:16],
		k8sUUID[19:23],
		k8sUUID[24:36])
	return strings.ToLower(uuid)
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
				glog.Warning("Unknown/unsupported address type:", address.Type)
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
