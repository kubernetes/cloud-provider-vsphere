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
	"fmt"
	"log"
	"testing"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vslm"
)

func TestDatastoreAndClusters(t *testing.T) {
	ctx := context.Background()
	model := simulator.VPX()
	defer model.Remove()

	model.Datacenter = 2
	model.Pod = 1
	model.Datastore = 4

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

	finder := find.NewFinder(c.Client, false)

	dc, err := finder.Datacenter(ctx, "DC0")
	if err != nil {
		t.Fatal(err)
	}

	finder.SetDatacenter(dc)

	pools, err := finder.ResourcePoolList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	stores, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	pod, err := finder.DatastoreCluster(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	// Move half the datastores into the datastore cluster
	var objs []types.ManagedObjectReference
	for i := 0; i < len(stores)/2; i++ {
		objs = append(objs, stores[i].Reference())
	}

	_, err = pod.MoveInto(ctx, objs)
	if err != nil {
		t.Fatal(err)
	}

	m := vslm.NewObjectManager(c.Client)
	ndisks := 4

	// Other half of the datastores
	standaloneDS := stores[len(stores)/2:]
	objs = []types.ManagedObjectReference{pod.Reference()}
	for i := 0; i < len(standaloneDS); i++ {
		objs = append(objs, standaloneDS[i].Reference())
	}

	// Create disks in the datatore cluster and the standalone datastores
	for _, store := range objs {
		for i := 0; i < ndisks; i++ {
			spec := types.VslmCreateSpec{
				Name:         fmt.Sprintf("test-disk-%d", i+1),
				CapacityInMB: 10,
				BackingSpec: &types.VslmCreateSpecDiskFileBackingSpec{
					VslmCreateSpecBackingSpec: types.VslmCreateSpecBackingSpec{
						Datastore: store.Reference(),
					},
					ProvisioningType: string(types.BaseConfigInfoDiskFileBackingInfoProvisioningTypeThin),
				},
			}

			if store.Type == "StoragePod" {
				if err = m.PlaceDisk(ctx, &spec, pools[0].Reference()); err != nil {
					t.Fatal(err)
				}
			}

			task, err := m.CreateDisk(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			err = task.Wait(ctx)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	allDS, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	for _, ds := range allDS {
		ids, err := m.List(ctx, ds)
		if err != nil {
			t.Fatal(err)
		}

		log.Printf("Datastore %s:", ds.InventoryPath)
		for i, id := range ids {
			log.Printf("  %d: %s", i, id.Id)
		}
	}
}
