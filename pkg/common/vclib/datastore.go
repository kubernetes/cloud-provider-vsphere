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

	"k8s.io/klog"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vslm"
)

// Datastore extends the govmomi Datastore object
type Datastore struct {
	*object.Datastore
	Datacenter *Datacenter
}

// DatastoreInfo is a structure to store the Datastore and it's Info.
type DatastoreInfo struct {
	*Datastore
	Info *types.DatastoreInfo
}

func (di DatastoreInfo) String() string {
	return fmt.Sprintf("Datastore: %+v, datastore URL: %s", di.Datastore, di.Info.Url)
}

// CreateDirectory creates the directory at location specified by directoryPath.
// If the intermediate level folders do not exist, and the parameter createParents is true, all the non-existent folders are created.
// directoryPath must be in the format "[vsanDatastore] kubevols"
func (ds *Datastore) CreateDirectory(ctx context.Context, directoryPath string, createParents bool) error {
	fileManager := object.NewFileManager(ds.Client())
	err := fileManager.MakeDirectory(ctx, directoryPath, ds.Datacenter.Datacenter, createParents)
	if err != nil {
		if soap.IsSoapFault(err) {
			soapFault := soap.ToSoapFault(err)
			if _, ok := soapFault.VimFault().(types.FileAlreadyExists); ok {
				return ErrFileAlreadyExist
			}
		}
		return err
	}
	klog.V(LogLevel).Infof("Created dir with path as %+q", directoryPath)
	return nil
}

// GetType returns the type of datastore
func (ds *Datastore) GetType(ctx context.Context) (string, error) {
	var dsMo mo.Datastore
	pc := property.DefaultCollector(ds.Client())
	err := pc.RetrieveOne(ctx, ds.Datastore.Reference(), []string{"summary"}, &dsMo)
	if err != nil {
		klog.Errorf("Failed to retrieve datastore summary property. err: %v", err)
		return "", err
	}
	return dsMo.Summary.Type, nil
}

// GetName returns the type of datastore
func (ds *Datastore) GetName(ctx context.Context) (string, error) {
	var dsMo mo.Datastore
	pc := property.DefaultCollector(ds.Client())
	err := pc.RetrieveOne(ctx, ds.Datastore.Reference(), []string{DatastoreInfoProperty}, &dsMo)
	if err != nil {
		klog.Errorf("Failed to retrieve datastore info property. err: %v", err)
		return "", err
	}
	return dsMo.Info.GetDatastoreInfo().Name, nil
}

// IsCompatibleWithStoragePolicy returns true if datastore is compatible with given storage policy else return false
// for not compatible datastore, fault message is also returned
func (ds *Datastore) IsCompatibleWithStoragePolicy(ctx context.Context, storagePolicyID string) (bool, string, error) {
	pbmClient, err := NewPbmClient(ctx, ds.Client())
	if err != nil {
		klog.Errorf("Failed to get new PbmClient Object. err: %v", err)
		return false, "", err
	}
	return pbmClient.IsDatastoreCompatible(ctx, storagePolicyID, ds)
}

// ListFirstClassDisks gets a list of first class disks (FCD) on this datastore
func (ds *Datastore) ListFirstClassDisks(ctx context.Context) ([]*FirstClassDisk, error) {
	m := vslm.NewObjectManager(ds.Client())

	oids, err := m.List(ctx, ds.Reference())
	if err != nil {
		klog.Errorf("Failed to list disks. Err: %v", err)
		return nil, err
	}

	var objs []*FirstClassDisk
	for _, id := range oids {
		o, err := m.Retrieve(ctx, ds.Reference(), id.Id)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &FirstClassDisk{
			ds.Datacenter,
			o,
			TypeDatastore,
			ds,
			nil,
		})
	}

	return objs, nil
}

// GetFirstClassDisk gets a specific first class disks (FCD) on this datastore
func (ds *Datastore) GetFirstClassDisk(ctx context.Context, diskID string, findBy FindFCD) (*FirstClassDisk, error) {
	m := vslm.NewObjectManager(ds.Client())

	oids, err := m.List(ctx, ds.Reference())
	if err != nil {
		klog.Errorf("Failed to list disks. Err: %v", err)
		return nil, err
	}

	for _, id := range oids {
		o, err := m.Retrieve(ctx, ds.Reference(), id.Id)
		if err != nil {
			return nil, err
		}

		if (findBy == FindFCDByName && o.Config.Name == diskID) ||
			(findBy == FindFCDByID && o.Config.Id.Id == diskID) {
			return &FirstClassDisk{
				ds.Datacenter,
				o,
				TypeDatastore,
				ds,
				nil,
			}, nil
		}
	}

	return nil, ErrNoDiskIDFound
}

// ListFirstClassDiskInfos gets a list of first class disks (FCD) on this datastore
func (dsi *DatastoreInfo) ListFirstClassDiskInfos(ctx context.Context) ([]*FirstClassDiskInfo, error) {
	m := vslm.NewObjectManager(dsi.Datacenter.Client())

	oids, err := m.List(ctx, dsi.Reference())
	if err != nil {
		klog.Errorf("Failed to list disks. Err: %v", err)
		return nil, err
	}

	var objs []*FirstClassDiskInfo
	for _, id := range oids {
		o, err := m.Retrieve(ctx, dsi.Reference(), id.Id)
		if err != nil {
			return nil, err
		}

		objs = append(objs, &FirstClassDiskInfo{
			&FirstClassDisk{
				dsi.Datacenter,
				o,
				TypeDatastore,
				dsi.Datastore,
				nil,
			},
			dsi,
			nil,
		})
	}

	return objs, nil
}

// GetFirstClassDiskInfo gets a specific first class disks (FCD) on this datastore
func (dsi *DatastoreInfo) GetFirstClassDiskInfo(ctx context.Context, diskID string, findBy FindFCD) (*FirstClassDiskInfo, error) {
	m := vslm.NewObjectManager(dsi.Datacenter.Client())

	oids, err := m.List(ctx, dsi.Reference())
	if err != nil {
		klog.Errorf("Failed to list disks. Err: %v", err)
		return nil, err
	}

	for _, id := range oids {
		o, err := m.Retrieve(ctx, dsi.Reference(), id.Id)
		if err != nil {
			return nil, err
		}

		if (findBy == FindFCDByName && o.Config.Name == diskID) ||
			(findBy == FindFCDByID && o.Config.Id.Id == diskID) {
			return &FirstClassDiskInfo{
				&FirstClassDisk{
					dsi.Datacenter,
					o,
					TypeDatastore,
					dsi.Datastore,
					nil,
				},
				dsi,
				nil,
			}, nil
		}
	}

	return nil, ErrNoDiskIDFound
}
