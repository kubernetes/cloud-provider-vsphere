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

package connectionmanager

import (
	"context"
	"math/rand"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

func TestWhichVCandDCByNodeIdByUUID(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(&config, nil)
	defer connMgr.Logout()

	// setup
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	vm.Guest.HostName = strings.ToLower(name)
	UUID := vm.Config.Uuid

	// context
	ctx := context.Background()

	info, err := connMgr.WhichVCandDCByNodeId(ctx, UUID, FindVMByUUID)
	if err != nil {
		t.Fatalf("WhichVCandDCByNodeId err=%v", err)
	}
	if info == nil {
		t.Fatalf("WhichVCandDCByNodeId info=nil")
	}

	if !strings.EqualFold(name, info.NodeName) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
	if !strings.EqualFold(UUID, info.UUID) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
}

func TestWhichVCandDCByNodeIdByName(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(&config, nil)
	defer connMgr.Logout()

	// setup
	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	vm.Guest.HostName = strings.ToLower(name)
	UUID := vm.Config.Uuid

	// context
	ctx := context.Background()

	info, err := connMgr.WhichVCandDCByNodeId(ctx, name, FindVMByName)
	if err != nil {
		t.Fatalf("WhichVCandDCByNodeId err=%v", err)
	}
	if info == nil {
		t.Fatalf("WhichVCandDCByNodeId info=nil")
	}

	if !strings.EqualFold(name, info.NodeName) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
	if !strings.EqualFold(UUID, info.UUID) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
}

func TestWhichVCandDCByFCDId(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(&config, nil)
	defer connMgr.Logout()

	// context
	ctx := context.Background()

	/*
	 * Setup
	 */
	// Get a simulator DS
	myds := simulator.Map.Any("Datastore").(*simulator.Datastore)

	items, err := connMgr.ListAllVCandDCPairs(ctx)
	if err != nil {
		t.Fatalf("ListAllVCandDCPairs err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListAllVCandDCPairs items should be 2 but count=%d", len(items))
	}

	randDC := items[rand.Intn(len(items))]

	datastoreName := myds.Name
	datastoreType := vclib.TypeDatastore
	volName := "myfcd"
	volSizeMB := int64(1024) //1GB

	err = randDC.DataCenter.CreateFirstClassDisk(ctx, datastoreName, datastoreType, volName, volSizeMB)
	if err != nil {
		t.Fatalf("CreateFirstClassDisk err=%v", err)
	}

	firstClassDisk, err := randDC.DataCenter.GetFirstClassDisk(
		ctx, datastoreName, datastoreType, volName, vclib.FindFCDByName)
	if err != nil {
		t.Fatalf("GetFirstClassDisk err=%v", err)
	}

	fcdID := firstClassDisk.Config.Id.Id
	/*
	 * Setup
	 */

	// call WhichVCandDCByFCDId
	fcdObj, err := connMgr.WhichVCandDCByFCDId(ctx, fcdID)
	if err != nil {
		t.Fatalf("WhichVCandDCByFCDId err=%v", err)
	}
	if fcdObj == nil {
		t.Fatalf("WhichVCandDCByFCDId fcdObj=nil")
	}

	if !strings.EqualFold(fcdID, fcdObj.FCDInfo.Config.Id.Id) {
		t.Errorf("FCD ID mismatch %s=%s", fcdID, fcdObj.FCDInfo.Config.Id.Id)
	}
	if datastoreType != fcdObj.FCDInfo.ParentType {
		t.Errorf("FCD DatastoreType mismatch %v=%v", datastoreType, fcdObj.FCDInfo.ParentType)
	}
	if !strings.EqualFold(datastoreName, fcdObj.FCDInfo.DatastoreInfo.Info.Name) {
		t.Errorf("FCD Datastore mismatch %s=%s", datastoreName, fcdObj.FCDInfo.DatastoreInfo.Info.Name)
	}
	if volSizeMB != fcdObj.FCDInfo.Config.CapacityInMB {
		t.Errorf("FCD Size mismatch %d=%d", volSizeMB, fcdObj.FCDInfo.Config.CapacityInMB)
	}
}
