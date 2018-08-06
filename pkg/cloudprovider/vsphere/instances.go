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
	"net"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/cloudprovider"

	"k8s.io/cloud-provider-vsphere/pkg/vclib"
)

const (
	POOL_SIZE      int    = 8
	QUEUE_SIZE     int    = POOL_SIZE * 10
	ProviderPrefix string = "vsphere://"

	CredentialManagerErrMsg = "The Credential Manager is not initialized"
)

// Error constants
var (
	ErrCredentialManager = errors.New(CredentialManagerErrMsg)
)

func newInstances(nodeManager *NodeManager) cloudprovider.Instances {
	return &instances{nodeManager}
}

// NodeAddresses returns all the valid addresses of the instance identified by
// nodeName. Only the public/private IPv4 addresses are considered for now.
//
// When nodeName identifies more than one instance, only the first will be
// considered.
func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	glog.V(4).Info("vsphere.instances.NodeAddresses() called")

	// Check if node has been discovered already
	if node, ok := i.nodeManager.nodeInfoMap[string(nodeName)]; ok {
		return node.NodeAddresses, nil
	}

	if err := i.nodeDiscoveryByName(ctx, nodeName); err != nil {
		return []v1.NodeAddress{}, err
	}

	return i.nodeManager.nodeInfoMap[string(nodeName)].NodeAddresses, nil
}

// NodeAddressesByProviderID returns all the valid addresses of the instance
// identified by providerID. Only the public/private IPv4 addresses will be
// considered for now.
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	glog.V(4).Info("vsphere.instances.NodeAddressesByProviderID() called")
	return i.NodeAddresses(ctx, types.NodeName(providerID))
}

// ExternalID returns the cloud provider ID of the instance identified by
// nodeName. If the instance does not exist or is no longer running, the
// returned error will be cloudprovider.InstanceNotFound.
//
// When nodeName identifies more than one instance, only the first will be
// considered.
func (i *instances) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	glog.V(4).Info("vsphere.instances.ExternalID() called")
	return i.InstanceID(ctx, nodeName)
}

// InstanceID returns the cloud provider ID of the instance identified by nodeName.
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	glog.V(4).Info("vsphere.instances.InstanceID() called")

	// Check if node has been discovered already
	if node, ok := i.nodeManager.nodeInfoMap[string(nodeName)]; ok {
		return node.UUID, nil
	}

	if err := i.nodeDiscoveryByName(ctx, nodeName); err != nil {
		return "", err
	}

	return i.nodeManager.nodeInfoMap[string(nodeName)].UUID, nil

}

// InstanceType returns the type of the instance identified by name.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	glog.V(4).Info("vsphere.instances.InstanceType() called")
	return "vsphere-vm", nil
}

// InstanceTypeByProviderID returns the type of the instance identified by providerID.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	glog.V(4).Info("vsphere.instances.InstanceTypeByProviderID() called")
	return "vsphere-vm", nil
}

// AddSSHKeyToAllInstances is not implemented; it always returns an error.
func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	glog.V(4).Info("vsphere.instances.AddSSHKeyToAllInstances() called")
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns hostname as a NodeName value.
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	glog.V(4).Info("vsphere.instances.CurrentNodeName() called")
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance identified by
// providerID is running.
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	glog.V(4).Info("vsphere.instances.InstanceExistsByProviderID() called")

	// Check if node has been discovered already
	if _, ok := i.nodeManager.nodeInfoMap[providerID]; !ok {
		if err := i.nodeDiscoveryByProviderID(ctx, providerID); err != nil {
			return false, err
		}
		return true, nil
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is in safe state to detach volumes
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, cloudprovider.NotImplemented
}

// PRIVATE

func (i *instances) nodeDiscoveryByName(ctx context.Context, nodeName types.NodeName) error {
	glog.V(4).Info("vsphere.instances.nodeDiscovery() called")

	type VmSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *VmSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *VmSearch, QUEUE_SIZE)

	glog.V(4).Infof("Discovering node %s with name %s", string(nodeName), string(nodeName))

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
		for vc, vsi := range i.nodeManager.vsphereInstanceMap {

			found := getVMFound()
			if found == true {
				break
			}

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := i.vcConnect(ctx, vsi)
			if err != nil {
				glog.V(4).Info("Discovering node error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.conn)
				if err != nil {
					glog.V(4).Info("Discovering node error dc:", err)
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
						glog.V(4).Info("Discovering node error dc:", err)
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

				glog.V(4).Infof("Finding node %s in vc=%s and datacenter=%s", nodeName, vc, datacenterObj.Name())
				queueChannel <- &VmSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	for j := 0; j < POOL_SIZE; j++ {
		go func() {
			for res := range queueChannel {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				// vm, err := res.datacenter.GetVMByUUID(ctx, nodeUUID)
				vm, err := res.datacenter.GetVMByDNSName(ctx, string(nodeName))
				if err != nil {
					glog.V(4).Infof("Error %q while looking for vm=%+v in vc=%s and datacenter=%s",
						err, nodeName, vm, res.vc, res.datacenter.Name())
					if err != vclib.ErrNoVMFound {
						setGlobalErr(err)
					} else {
						glog.V(4).Infof("Did not find node %s in vc=%s and datacenter=%s",
							nodeName, res.vc, res.datacenter.Name(), err)
					}
					continue
				}
				if vm != nil {
					glog.V(4).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
						nodeName, vm, res.vc, res.datacenter.Name())

					nodeUUID, err := vm.GetVMUUID()
					if err != nil {
						glog.V(4).Infof("Did not find UUID node %s in vc=%s and datacenter=%s",
							nodeName, res.vc, res.datacenter.Name(), err)
					}

					addrs := []v1.NodeAddress{}
					vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*vclib.VirtualMachine{vm}, []string{"guest.net"})
					if err != nil {
						glog.Errorf("Failed to get VM Managed object with property guest.net for node: %q. err: +%v", string(nodeName), err)
						// return nil, err
					}
					// retrieve VM's ip(s)
					for _, v := range vmMoList[0].Guest.Net {
						// if vsi.cfg.Network.PublicNetwork == v.Network {
						for _, ip := range v.IpAddress {
							if net.ParseIP(ip).To4() != nil {
								v1helper.AddToNodeAddresses(&addrs,
									v1.NodeAddress{
										Type:    v1.NodeExternalIP,
										Address: ip,
									}, v1.NodeAddress{
										Type:    v1.NodeInternalIP,
										Address: ip,
									},
								)
							}
						}
						// }
					}

					nodeInfo := &NodeInfo{
						dataCenter:    res.datacenter,
						vm:            vm,
						vcServer:      res.vc,
						UUID:          nodeUUID,
						NodeName:      string(nodeName),
						NodeAddresses: addrs,
					}

					i.nodeManager.nodeInfoLock.Lock()
					i.nodeManager.nodeInfoMap[string(nodeName)] = nodeInfo
					i.nodeManager.nodeInfoMap[ProviderPrefix+nodeUUID] = nodeInfo
					i.nodeManager.nodeInfoLock.Unlock()

					for range queueChannel {
					}
					setVMFound(true)
					break
				}
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

	glog.V(4).Infof("Discovery Node: %q vm not found", nodeName)
	return vclib.ErrNoVMFound

}

// TODO(frapposelli): FIX THIS CODE DUPLICATION
// TODO(frapposelli): THIS SHOULD BE DONE WITH PROPERTYCOLLECTOR
func (i *instances) nodeDiscoveryByProviderID(ctx context.Context, providerID string) error {
	glog.V(4).Info("vsphere.instances.nodeDiscovery() called")

	type VmSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *VmSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *VmSearch, QUEUE_SIZE)

	glog.V(4).Infof("Discovering node with providerID %s", providerID)

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
		for vc, vsi := range i.nodeManager.vsphereInstanceMap {

			found := getVMFound()
			if found == true {
				break
			}

			// Create context
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := i.vcConnect(ctx, vsi)
			if err != nil {
				glog.V(4).Info("Discovering node error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.conn)
				if err != nil {
					glog.V(4).Info("Discovering node error dc:", err)
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
						glog.V(4).Info("Discovering node error dc:", err)
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

				glog.V(4).Infof("Finding providerID %s in vc=%s and datacenter=%s", providerID, vc, datacenterObj.Name())
				queueChannel <- &VmSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	for j := 0; j < POOL_SIZE; j++ {
		go func() {
			for res := range queueChannel {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				nodeUUID := i.GetUUIDFromProviderID(providerID)
				vm, err := res.datacenter.GetVMByUUID(ctx, nodeUUID)
				if err != nil {
					glog.V(4).Infof("Error %q while looking for vm=%+v in vc=%s and datacenter=%s",
						err, nodeUUID, vm, res.vc, res.datacenter.Name())
					if err != vclib.ErrNoVMFound {
						setGlobalErr(err)
					} else {
						glog.V(4).Infof("Did not find node %s in vc=%s and datacenter=%s",
							nodeUUID, res.vc, res.datacenter.Name(), err)
					}
					continue
				}
				if vm != nil {
					nodeName, err := vm.GetVMNodeName()
					if err != nil {
						glog.V(4).Infof("Did not find NodeName node %s in vc=%s and datacenter=%s",
							nodeName, res.vc, res.datacenter.Name(), err)
					}

					glog.V(4).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
						nodeName, vm, res.vc, res.datacenter.Name())

					addrs := []v1.NodeAddress{}
					vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*vclib.VirtualMachine{vm}, []string{"guest.net"})
					if err != nil {
						glog.Errorf("Failed to get VM Managed object with property guest.net for node: %q. err: +%v", string(nodeName), err)
						// return nil, err
					}
					// retrieve VM's ip(s)
					for _, v := range vmMoList[0].Guest.Net {
						// if vsi.cfg.Network.PublicNetwork == v.Network {
						for _, ip := range v.IpAddress {
							if net.ParseIP(ip).To4() != nil {
								v1helper.AddToNodeAddresses(&addrs,
									v1.NodeAddress{
										Type:    v1.NodeExternalIP,
										Address: ip,
									}, v1.NodeAddress{
										Type:    v1.NodeInternalIP,
										Address: ip,
									},
								)
							}
						}
						// }
					}

					nodeInfo := &NodeInfo{
						dataCenter:    res.datacenter,
						vm:            vm,
						vcServer:      res.vc,
						UUID:          nodeUUID,
						NodeName:      string(nodeName),
						NodeAddresses: addrs,
					}

					i.nodeManager.nodeInfoLock.Lock()
					i.nodeManager.nodeInfoMap[string(nodeName)] = nodeInfo
					i.nodeManager.nodeInfoMap[ProviderPrefix+nodeUUID] = nodeInfo
					i.nodeManager.nodeInfoLock.Unlock()

					for range queueChannel {
					}
					setVMFound(true)
					break
				}
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

	glog.V(4).Infof("Discovery providerID: %q vm not found", providerID)
	return vclib.ErrNoVMFound

}

func (i *instances) vcConnect(ctx context.Context, vsphereInstance *VSphereInstance) error {
	credentialManager := i.CredentialManager()
	if credentialManager == nil {
		err := ErrCredentialManager
		glog.Errorf("%v", err)
		return err
	}

	// Get latest credentials from SecretCredentialManager
	credentials, err := credentialManager.GetCredential(vsphereInstance.conn.Hostname)
	if err == nil {
		glog.V(4).Infof("Secret for server %q found. Attempting connection from secret.",
			vsphereInstance.conn.Hostname)

		//save username/password from config
		tmpUsername := vsphereInstance.conn.Username
		tmpPassword := vsphereInstance.conn.Password

		vsphereInstance.conn.UpdateCredentials(credentials.User, credentials.Password)
		err := vsphereInstance.conn.Connect(ctx)
		if err == nil {
			glog.V(4).Infof("Successfully connected to %q using credentials from secret.",
				vsphereInstance.conn.Hostname)
			return nil
		} else {
			glog.V(4).Infof("Failed to connected to %q using credentials from secret.",
				vsphereInstance.conn.Hostname)
		}

		//revert username/password
		vsphereInstance.conn.UpdateCredentials(tmpUsername, tmpPassword)

		glog.V(4).Infof("Unable to connect to %q using credentials from secret.",
			vsphereInstance.conn.Hostname)
	} else {
		glog.V(4).Infof("Unable to find secret for server %q. Using credentials from configuration.",
			vsphereInstance.conn.Hostname)
	}

	glog.V(4).Infof("Attempting connection on %q using credentials from config.",
		vsphereInstance.conn.Hostname)

	err = vsphereInstance.conn.Connect(ctx)
	if err == nil {
		glog.V(4).Infof("Successfully connected to %q using credentials from config.",
			vsphereInstance.conn.Hostname)
	} else {
		glog.V(4).Infof("Failed to connected to %q using credentials from secret.",
			vsphereInstance.conn.Hostname)
	}

	return err
}

func (i *instances) CredentialManager() *SecretCredentialManager {
	i.nodeManager.credentialManagerLock.Lock()
	defer i.nodeManager.credentialManagerLock.Unlock()
	return i.nodeManager.credentialManager
}

func (i *instances) UpdateCredentialManager(credentialManager *SecretCredentialManager) {
	i.nodeManager.credentialManagerLock.Lock()
	defer i.nodeManager.credentialManagerLock.Unlock()
	i.nodeManager.credentialManager = credentialManager
}

func (i *instances) GetUUIDFromProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderPrefix)
}
