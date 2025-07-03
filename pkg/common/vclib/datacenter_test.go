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
	"strings"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/simulator"
)

func TestDatacenter(t *testing.T) {
	ctx := context.Background()

	// vCenter model + initial set of objects (cluster, hosts, VMs, network, datastore, etc)
	model := simulator.VPX()

	defer model.Remove()
	err := model.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	avm := model.Map().Any(VirtualMachineType).(*simulator.VirtualMachine)

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vc := &VSphereConnection{Client: c.Client}

	runDatacenterTest := func(t *testing.T, dc *Datacenter) {
		_, err = dc.GetVMByUUID(ctx, testNameNotFound)
		if err == nil || err != ErrNoVMFound {
			t.Error("expected error")
		}

		_, err = dc.GetVMByUUID(ctx, avm.Summary.Config.Uuid)
		if err != nil {
			t.Error(err)
		}

		_, err = dc.GetVMByPath(ctx, testNameNotFound)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Error("expected error")
		}

		vm, err := dc.GetVMByPath(ctx, TestDefaultDatacenter+"/vm/"+avm.Name)
		if err != nil {
			t.Error(err)
		}

		_, err = dc.GetFolderByPath(ctx, testNameNotFound)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Error("expected error")
		}

		_, err = dc.GetFolderByPath(ctx, TestDefaultDatacenter+"/vm")
		if err != nil {
			t.Error(err)
		}

		_, err = dc.GetVMMoList(ctx, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "VirtualMachine Object list is empty") {
			t.Error("expected error")
		}

		_, err = dc.GetVMMoList(ctx, []*VirtualMachine{vm}, []string{testNameNotFound}) // invalid property
		if err == nil || !strings.Contains(err.Error(), "InvalidProperty") {
			t.Error("expected error")
		}

		_, err = dc.GetVMMoList(ctx, []*VirtualMachine{vm}, []string{"summary"})
		if err != nil {
			t.Error(err)
		}
	}

	_, err = GetDatacenter(ctx, vc, testNameNotFound)
	if err == nil {
		t.Error("expected error")
	}

	t.Run("should get objects using Datacenter path", func(t *testing.T) {
		dc, err := GetDatacenter(ctx, vc, TestDefaultDatacenter)
		if err != nil {
			t.Error(err)
		}
		runDatacenterTest(t, dc)
	})

	t.Run("should get objects using Datacenter MOID", func(t *testing.T) {
		dcRef := model.Map().Any("Datacenter")
		dc, err := GetDatacenter(ctx, vc, dcRef.Reference().String())
		if err != nil {
			t.Error(err)
		}
		runDatacenterTest(t, dc)
	})

}
