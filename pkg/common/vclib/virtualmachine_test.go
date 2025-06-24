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

package vclib

import (
	"context"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/simulator"
	klog "k8s.io/klog/v2"
)

func TestVirtualMachine(t *testing.T) {
	ctx := context.Background()

	model := simulator.VPX()

	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vc := &VSphereConnection{Client: c.Client}

	dc, err := GetDatacenter(ctx, vc, TestDefaultDatacenter)
	if err != nil {
		t.Error(err)
	}

	folders, err := dc.Folders(ctx)
	if err != nil {
		t.Fatal(err)
	}

	folder, err := dc.GetFolderByPath(ctx, folders.VmFolder.InventoryPath)
	if err != nil {
		t.Fatal(err)
	}

	vms, err := getVirtualMachines(ctx, folder.Folder, dc)
	if err != nil {
		t.Fatal(err)
	}

	if len(vms) == 0 {
		t.Fatal("no VMs")
	}

	for _, vm := range vms {
		active, err := vm.IsActive(ctx)
		if err != nil {
			t.Error(err)
		}

		if !active {
			t.Errorf("active=%t, expected=%t", active, true)
		}
	}
}

func getVirtualMachines(ctx context.Context, folder *object.Folder, dc *Datacenter) ([]*VirtualMachine, error) {
	vmFolders, err := folder.Children(ctx)
	if err != nil {
		klog.Errorf("Failed to get children from Folder: %s. err: %+v", folder.InventoryPath, err)
		return nil, err
	}
	var vmObjList []*VirtualMachine
	for _, vmFolder := range vmFolders {
		if vmFolder.Reference().Type == VirtualMachineType {
			vmObj := VirtualMachine{object.NewVirtualMachine(folder.Client(), vmFolder.Reference()), dc}
			vmObjList = append(vmObjList, &vmObj)
		}
	}
	return vmObjList, nil
}
