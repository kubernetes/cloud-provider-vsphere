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

	"github.com/golang/glog"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vslm"
)

// StoragePod extends the govmomi StoragePod object
type StoragePod struct {
	Datacenter *Datacenter
	*object.StoragePod
	Datastores []*Datastore
}

// StoragePodInfo is a structure to store the StoragePod and it's Info.
type StoragePodInfo struct {
	*StoragePod
	Summary        *types.StoragePodSummary
	Config         *types.StorageDrsConfigInfo
	DatastoreInfos []*DatastoreInfo
}

// PopulateChildDatastoreInfos discovers the child DatastoreInfos backed by this StoragePodInfo
func (spi *StoragePodInfo) PopulateChildDatastoreInfos(ctx context.Context, refresh bool) error {
	if refresh {
		glog.Infof("Re-discover datastore infos")
	}
	if len(spi.DatastoreInfos) > 0 && !refresh {
		glog.Infof("Already discovered datastore infos")
		return nil
	}

	err := spi.PopulateChildDatastores(ctx, false)
	if err != nil {
		glog.Errorf("PopulateChildDatastores failed. Err: %v", err)
		return err
	}

	var dsList []types.ManagedObjectReference
	for _, ds := range spi.Datastores {
		dsList = append(dsList, ds.Reference())
	}

	var dsMoList []mo.Datastore
	pc := property.DefaultCollector(spi.Datacenter.Client())
	properties := []string{DatastoreInfoProperty}
	err = pc.Retrieve(ctx, dsList, properties, &dsMoList)
	if err != nil {
		glog.Errorf("Failed to get Datastore managed objects from datastore objects."+
			" dsObjList: %+v, properties: %+v, err: %v", dsList, properties, err)
		return err
	}

	spi.DatastoreInfos = make([]*DatastoreInfo, 0)
	for _, dsMo := range dsMoList {
		spi.DatastoreInfos = append(spi.DatastoreInfos, &DatastoreInfo{
			&Datastore{
				object.NewDatastore(spi.Datacenter.Client(), dsMo.Reference()),
				spi.Datacenter,
			},
			dsMo.Info.GetDatastoreInfo(),
		})
	}

	return nil
}

// ListFirstClassDisksInfo gets a list of first class disks (FCD) on this datastore backed by this StoragePodInfo
func (spi *StoragePodInfo) ListFirstClassDisksInfo(ctx context.Context) ([]*FirstClassDiskInfo, error) {
	err := spi.PopulateChildDatastoreInfos(ctx, false)
	if err != nil {
		glog.Errorf("PopulateChildDatastoreInfos failed. Err: %v", err)
		return nil, err
	}

	m := vslm.NewObjectManager(spi.Datacenter.Client())

	var objs []*FirstClassDiskInfo
	for _, child := range spi.DatastoreInfos {
		oids, err := m.List(ctx, child)
		if err != nil {
			glog.Errorf("Failed to list disks. Err: %v", err)
			return nil, err
		}

		var ids []string
		for _, id := range oids {
			ids = append(ids, id.Id)
		}

		for _, id := range ids {
			o, err := m.Retrieve(ctx, child, id)
			if err != nil {
				return nil, err
			}

			objs = append(objs, &FirstClassDiskInfo{
				&FirstClassDisk{
					spi.Datacenter,
					o,
					TypeDatastoreCluster,
					child.Datastore,
					spi.StoragePod,
				},
				child,
				spi,
			})
		}
	}

	return objs, nil
}

// PopulateChildDatastores discovers the child Datastores backed by this StoragePod
func (sp *StoragePod) PopulateChildDatastores(ctx context.Context, refresh bool) error {
	if refresh {
		glog.Infof("Re-discover datastores")
	}
	if len(sp.Datastores) > 0 && !refresh {
		glog.Infof("Already discovered datastores")
		return nil
	}

	children, err := sp.Children(ctx)
	if err != nil {
		glog.Errorf("Failed to list disks. Err: %v", err)
		return err
	}

	sp.Datastores = make([]*Datastore, 0)
	for _, child := range children {
		sp.Datastores = append(sp.Datastores, &Datastore{
			object.NewDatastore(sp.Datacenter.Client(), child.Reference()),
			sp.Datacenter,
		})
	}

	return nil
}

// ListFirstClassDisks gets a list of first class disks (FCD) on this datastore backed by this StoragePod
func (sp *StoragePod) ListFirstClassDisks(ctx context.Context) ([]*FirstClassDisk, error) {
	err := sp.PopulateChildDatastores(ctx, false)
	if err != nil {
		glog.Errorf("PopulateChildDatastores failed. Err: %v", err)
		return nil, err
	}

	m := vslm.NewObjectManager(sp.Datacenter.Client())

	var objs []*FirstClassDisk
	for _, child := range sp.Datastores {
		oids, err := m.List(ctx, child)
		if err != nil {
			glog.Errorf("Failed to list disks. Err: %v", err)
			return nil, err
		}

		var ids []string
		for _, id := range oids {
			ids = append(ids, id.Id)
		}

		for _, id := range ids {
			o, err := m.Retrieve(ctx, child, id)
			if err != nil {
				return nil, err
			}

			objs = append(objs, &FirstClassDisk{
				sp.Datacenter,
				o,
				TypeDatastoreCluster,
				child,
				sp,
			})
		}
	}

	return objs, nil
}
