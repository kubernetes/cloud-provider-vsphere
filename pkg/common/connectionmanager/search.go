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

package connectionmanager

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/klog"

	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

func (f FindVM) String() string {
	switch f {
	case FindVMByUUID:
		return "byUUID"
	case FindVMByName:
		return "byName"
	default:
		return "byUnknown"
	}
}

// WhichVCandDCByNodeId finds the VC/DC combo that owns a particular VM
func (cm *ConnectionManager) WhichVCandDCByNodeId(ctx context.Context, nodeID string, searchBy FindVM) (*VmDiscoveryInfo, error) {
	if nodeID == "" {
		klog.V(3).Info("WhichVCandDCByNodeId called but nodeID is empty")
		return nil, vclib.ErrNoVMFound
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
		klog.V(3).Info("WhichVCandDCByNodeId by UUID")
		myNodeID = strings.ToLower(nodeID)
	} else {
		klog.V(3).Info("WhichVCandDCByNodeId by Name")
	}
	klog.V(2).Info("WhichVCandDCByNodeId nodeID: ", myNodeID)

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
		for vc, vsi := range cm.VsphereInstanceMap {
			var datacenterObjs []*vclib.Datacenter

			found := getVMFound()
			if found == true {
				break
			}

			var err error
			for i := 0; i < NUM_OF_CONNECTION_ATTEMPTS; i++ {
				err = cm.Connect(ctx, vc)
				if err == nil {
					break
				}
				time.Sleep(time.Duration(RETRY_ATTEMPT_DELAY_IN_SECONDS) * time.Second)
			}

			if err != nil {
				klog.Error("WhichVCandDCByNodeId error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.Cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.Conn)
				if err != nil {
					klog.Error("WhichVCandDCByNodeId error dc:", err)
					setGlobalErr(err)
					continue
				}
			} else {
				datacenters := strings.Split(vsi.Cfg.Datacenters, ",")
				for _, dc := range datacenters {
					dc = strings.TrimSpace(dc)
					if dc == "" {
						continue
					}
					datacenterObj, err := vclib.GetDatacenter(ctx, vsi.Conn, dc)
					if err != nil {
						klog.Error("WhichVCandDCByNodeId error dc:", err)
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

				klog.V(4).Infof("Finding node %s in vc=%s and datacenter=%s", myNodeID, vc, datacenterObj.Name())
				queueChannel <- &vmSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	var vmInfo *VmDiscoveryInfo
	for i := 0; i < POOL_SIZE; i++ {
		wg.Add(1)
		go func() {
			for res := range queueChannel {
				var vm *vclib.VirtualMachine
				var err error
				if searchBy == FindVMByUUID {
					vm, err = res.datacenter.GetVMByUUID(ctx, myNodeID)
				} else {
					vm, err = res.datacenter.GetVMByDNSName(ctx, myNodeID)
				}

				if err != nil {
					klog.Errorf("Error while looking for vm=%s(%s) in vc=%s and datacenter=%s: %v",
						myNodeID, searchBy, res.vc, res.datacenter.Name(), err)
					if err != vclib.ErrNoVMFound {
						setGlobalErr(err)
					} else {
						klog.V(2).Infof("Did not find node %s in vc=%s and datacenter=%s",
							myNodeID, res.vc, res.datacenter.Name())
					}
					continue
				}

				var oVM mo.VirtualMachine
				err = vm.Properties(ctx, vm.Reference(), []string{"config", "summary", "guest"}, &oVM)
				if err != nil {
					klog.Errorf("Error collecting properties for vm=%+v in vc=%s and datacenter=%s: %v",
						vm, res.vc, res.datacenter.Name(), err)
					continue
				}

				klog.V(2).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
					nodeID, vm, res.vc, res.datacenter.Name())
				klog.V(2).Info("Hostname: ", oVM.Guest.HostName, " UUID: ", oVM.Summary.Config.Uuid)

				vmInfo = &VmDiscoveryInfo{DataCenter: res.datacenter, VM: vm, VcServer: res.vc,
					UUID: oVM.Summary.Config.Uuid, NodeName: oVM.Guest.HostName}
				setVMFound(true)
				break
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if vmFound {
		return vmInfo, nil
	}
	if globalErr != nil {
		return nil, *globalErr
	}

	klog.V(4).Infof("WhichVCandDCByNodeId: %q vm not found", myNodeID)
	return nil, vclib.ErrNoVMFound
}

func (cm *ConnectionManager) WhichVCandDCByFCDId(ctx context.Context, fcdID string) (*FcdDiscoveryInfo, error) {
	if fcdID == "" {
		klog.V(3).Info("WhichVCandDCByFCDId called but fcdID is empty")
		return nil, vclib.ErrNoDiskIDFound
	}
	klog.V(2).Info("WhichVCandDCByFCDId fcdID: ", fcdID)

	type fcdSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *fcdSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *fcdSearch, QUEUE_SIZE)

	fcdFound := false
	globalErr = nil

	setGlobalErr := func(err error) {
		globalErrMutex.Lock()
		globalErr = &err
		globalErrMutex.Unlock()
	}

	setFCDFound := func(found bool) {
		mutex.Lock()
		fcdFound = found
		mutex.Unlock()
	}

	getFCDFound := func() bool {
		mutex.Lock()
		found := fcdFound
		mutex.Unlock()
		return found
	}

	go func() {
		for vc, vsi := range cm.VsphereInstanceMap {
			var datacenterObjs []*vclib.Datacenter

			found := getFCDFound()
			if found == true {
				break
			}

			var err error
			for i := 0; i < NUM_OF_CONNECTION_ATTEMPTS; i++ {
				err = cm.Connect(ctx, vc)
				if err == nil {
					break
				}
				time.Sleep(time.Duration(RETRY_ATTEMPT_DELAY_IN_SECONDS) * time.Second)
			}

			if err != nil {
				klog.Error("WhichVCandDCByFCDId error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.Cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.Conn)
				if err != nil {
					klog.Error("WhichVCandDCByFCDId error dc:", err)
					setGlobalErr(err)
					continue
				}
			} else {
				datacenters := strings.Split(vsi.Cfg.Datacenters, ",")
				for _, dc := range datacenters {
					dc = strings.TrimSpace(dc)
					if dc == "" {
						continue
					}
					datacenterObj, err := vclib.GetDatacenter(ctx, vsi.Conn, dc)
					if err != nil {
						klog.Error("WhichVCandDCByFCDId error dc:", err)
						setGlobalErr(err)
						continue
					}
					datacenterObjs = append(datacenterObjs, datacenterObj)
				}
			}

			for _, datacenterObj := range datacenterObjs {
				found := getFCDFound()
				if found == true {
					break
				}

				klog.V(4).Infof("Finding FCD %s in vc=%s and datacenter=%s", fcdID, vc, datacenterObj.Name())
				queueChannel <- &fcdSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	var fcdInfo *FcdDiscoveryInfo
	for i := 0; i < POOL_SIZE; i++ {
		wg.Add(1)
		go func() {
			for res := range queueChannel {

				fcd, err := res.datacenter.DoesFirstClassDiskExist(ctx, fcdID)
				if err != nil {
					klog.Errorf("Error while looking for FCD=%+v in vc=%s and datacenter=%s: %v",
						fcd, res.vc, res.datacenter.Name(), err)
					if err != vclib.ErrNoDiskIDFound {
						setGlobalErr(err)
					} else {
						klog.V(2).Infof("Did not find FCD %s in vc=%s and datacenter=%s",
							fcdID, res.vc, res.datacenter.Name())
					}
					continue
				}

				klog.V(2).Infof("Found FCD %s as vm=%+v in vc=%s and datacenter=%s",
					fcdID, fcd, res.vc, res.datacenter.Name())

				fcdInfo = &FcdDiscoveryInfo{DataCenter: res.datacenter, FCDInfo: fcd, VcServer: res.vc}
				setFCDFound(true)
				break
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if fcdFound {
		return fcdInfo, nil
	}
	if globalErr != nil {
		return nil, *globalErr
	}

	klog.V(4).Infof("WhichVCandDCByFCDId: %q FCD not found", fcdID)
	return nil, vclib.ErrNoDiskIDFound
}
