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
	"strings"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	klog "k8s.io/klog/v2"
)

// Datacenter extends the govmomi Datacenter object
type Datacenter struct {
	*object.Datacenter
}

// GetDatacenter returns the DataCenter Object for the given datacenterPath or datacenter MOID.
// If datacenter is located in a folder, include full path to datacenter else just provide the datacenter name
func GetDatacenter(ctx context.Context, connection *VSphereConnection, datacenterPath string) (*Datacenter, error) {
	var datacenter *object.Datacenter
	var err error

	// Try to get an object reference based on the requested datacenter name.
	// eg.: if datacenterPath == Datacenter:datacenter-3, this is a valid MOID
	// so dcRef will not be null.
	dcRef := object.ReferenceFromString(datacenterPath)
	if dcRef != nil {
		datacenter = object.NewDatacenter(connection.Client, *dcRef)
		datacenter.InventoryPath, err = find.InventoryPath(ctx, connection.Client, dcRef.Reference())
		if err != nil {
			klog.Errorf("Failed to find datacenter by MOID: %s. err: %+v", datacenterPath, err)
			return nil, err
		}
		klog.Infof("Datacenter found by Moid: %s", datacenter.InventoryPath)
	} else {
		finder := find.NewFinder(connection.Client, false)
		datacenter, err = finder.Datacenter(ctx, datacenterPath)
		if err != nil {
			klog.Errorf("Failed to find the datacenter: %s. err: %+v", datacenterPath, err)
			return nil, err
		}
	}

	dc := Datacenter{datacenter}
	return &dc, nil
}

// GetAllDatacenter returns all the DataCenter Objects
func GetAllDatacenter(ctx context.Context, connection *VSphereConnection) ([]*Datacenter, error) {
	var dc []*Datacenter
	finder := find.NewFinder(connection.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		klog.Errorf("Failed to find the datacenter. err: %+v", err)
		return nil, err
	}
	for _, datacenter := range datacenters {
		dc = append(dc, &(Datacenter{datacenter}))
	}

	return dc, nil
}

// GetNumberOfDatacenters returns the number of DataCenters in this vCenter
func GetNumberOfDatacenters(ctx context.Context, connection *VSphereConnection) (int, error) {
	finder := find.NewFinder(connection.Client, false)
	datacenters, err := finder.DatacenterList(ctx, "*")
	if err != nil {
		klog.Errorf("Failed to find the datacenter. err: %+v", err)
		return 0, err
	}
	return len(datacenters), nil
}

// GetVMByIP gets the VM object from the given IP address
func (dc *Datacenter) GetVMByIP(ctx context.Context, ipAddy string) (*VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	ipAddy = strings.ToLower(strings.TrimSpace(ipAddy))
	svm, err := s.FindByIp(ctx, dc.Datacenter, ipAddy, true)
	if err != nil {
		klog.Errorf("Failed to find VM by IP. VM IP: %s, err: %+v", ipAddy, err)
		return nil, err
	}
	if svm == nil {
		klog.Errorf("Unable to find VM by IP. VM IP: %s", ipAddy)
		return nil, ErrNoVMFound
	}
	virtualMachine := VirtualMachine{svm.(*object.VirtualMachine), dc}
	return &virtualMachine, nil
}

// GetVMByDNSName gets the VM object from the given dns name
func (dc *Datacenter) GetVMByDNSName(ctx context.Context, dnsName string) (*VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	dnsName = strings.ToLower(strings.TrimSpace(dnsName))
	svms, err := s.FindAllByDnsName(ctx, dc.Datacenter, dnsName, true)
	if err != nil {
		klog.Errorf("Failed to find VM by DNS Name. VM DNS Name: %s, err: %+v", dnsName, err)
		return nil, err
	}
	if len(svms) == 0 {
		klog.Errorf("Unable to find VM by DNS Name. VM DNS Name: %s", dnsName)
		return nil, ErrNoVMFound
	}
	if len(svms) > 1 {
		klog.Errorf("Multiple vms found VM by DNS Name. DNS Name: %s", dnsName)
		return nil, ErrMultipleVMsFound
	}
	virtualMachine := VirtualMachine{svms[0].(*object.VirtualMachine), dc}
	return &virtualMachine, nil
}

// GetVMByUUID gets the VM object from the given vmUUID
func (dc *Datacenter) GetVMByUUID(ctx context.Context, vmUUID string) (*VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	vmUUID = strings.ToLower(strings.TrimSpace(vmUUID))
	svm, err := s.FindByUuid(ctx, dc.Datacenter, vmUUID, true, nil)
	if err != nil {
		klog.Errorf("Failed to find VM by UUID. VM UUID: %s, err: %+v", vmUUID, err)
		return nil, err
	}
	if svm == nil {
		klog.Errorf("Unable to find VM by UUID. VM UUID: %s", vmUUID)
		return nil, ErrNoVMFound
	}
	virtualMachine := VirtualMachine{svm.(*object.VirtualMachine), dc}
	return &virtualMachine, nil
}

// GetVMByPath gets the VM object from the given vmPath
// vmPath should be the full path to VM and not just the name
func (dc *Datacenter) GetVMByPath(ctx context.Context, vmPath string) (*VirtualMachine, error) {
	finder := getFinder(dc)
	vm, err := finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		klog.Errorf("Failed to find VM by Path. VM Path: %s, err: %+v", vmPath, err)
		return nil, err
	}
	virtualMachine := VirtualMachine{vm, dc}
	return &virtualMachine, nil
}

// GetAllDatastores gets the datastore URL to DatastoreInfo map for all the datastores in
// the datacenter.
func (dc *Datacenter) GetAllDatastores(ctx context.Context) (map[string]*DatastoreInfo, error) {
	finder := getFinder(dc)
	datastores, err := finder.DatastoreList(ctx, "*")
	if err != nil {
		klog.Errorf("Failed to get all the datastores. err: %+v", err)
		return nil, err
	}
	var dsList []types.ManagedObjectReference
	for _, ds := range datastores {
		dsList = append(dsList, ds.Reference())
	}

	var dsMoList []mo.Datastore
	pc := property.DefaultCollector(dc.Client())
	properties := []string{DatastoreInfoProperty}
	err = pc.Retrieve(ctx, dsList, properties, &dsMoList)
	if err != nil {
		klog.Errorf("Failed to get Datastore managed objects from datastore objects."+
			" dsObjList: %+v, properties: %+v, err: %v", dsList, properties, err)
		return nil, err
	}

	dsURLInfoMap := make(map[string]*DatastoreInfo)
	for _, dsMo := range dsMoList {
		dsURLInfoMap[dsMo.Info.GetDatastoreInfo().Url] = &DatastoreInfo{
			&Datastore{object.NewDatastore(dc.Client(), dsMo.Reference()),
				dc},
			dsMo.Info.GetDatastoreInfo()}
	}
	klog.V(9).Infof("dsURLInfoMap : %+v", dsURLInfoMap)
	return dsURLInfoMap, nil
}

// GetResourcePool gets the resource pool for the given path
func (dc *Datacenter) GetResourcePool(ctx context.Context, computePath string) (*object.ResourcePool, error) {
	finder := getFinder(dc)
	var computeResource *object.ComputeResource
	var err error
	if computePath == "" {
		computeResource, err = finder.DefaultComputeResource(ctx)
	} else {
		computeResource, err = finder.ComputeResource(ctx, computePath)
	}
	if err != nil {
		klog.Errorf("Failed to get the ResourcePool for computePath '%s'. err: %+v", computePath, err)
		return nil, err
	}
	return computeResource.ResourcePool(ctx)
}

// GetFolderByPath gets the Folder Object from the given folder path
// folderPath should be the full path to folder
func (dc *Datacenter) GetFolderByPath(ctx context.Context, folderPath string) (*Folder, error) {
	finder := getFinder(dc)
	vmFolder, err := finder.Folder(ctx, folderPath)
	if err != nil {
		klog.Errorf("Failed to get the folder reference for %s. err: %+v", folderPath, err)
		return nil, err
	}
	folder := Folder{vmFolder, dc}
	return &folder, nil
}

// GetVMMoList gets the VM Managed Objects with the given properties from the VM object
func (dc *Datacenter) GetVMMoList(ctx context.Context, vmObjList []*VirtualMachine, properties []string) ([]mo.VirtualMachine, error) {
	var vmMoList []mo.VirtualMachine
	var vmRefs []types.ManagedObjectReference
	if len(vmObjList) < 1 {
		klog.Error("VirtualMachine Object list is empty")
		return nil, fmt.Errorf("VirtualMachine Object list is empty")
	}

	for _, vmObj := range vmObjList {
		vmRefs = append(vmRefs, vmObj.Reference())
	}
	pc := property.DefaultCollector(dc.Client())
	err := pc.Retrieve(ctx, vmRefs, properties, &vmMoList)
	if err != nil {
		klog.Errorf("Failed to get VM managed objects from VM objects. vmObjList: %+v, properties: %+v, err: %v", vmObjList, properties, err)
		return nil, err
	}
	return vmMoList, nil
}
