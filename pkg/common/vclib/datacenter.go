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
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
	"github.com/vmware/govmomi/vslm"
)

// Datacenter extends the govmomi Datacenter object
type Datacenter struct {
	*object.Datacenter
}

// GetDatacenter returns the DataCenter Object for the given datacenterPath
// If datacenter is located in a folder, include full path to datacenter else just provide the datacenter name
func GetDatacenter(ctx context.Context, connection *VSphereConnection, datacenterPath string) (*Datacenter, error) {
	finder := find.NewFinder(connection.Client, false)
	datacenter, err := finder.Datacenter(ctx, datacenterPath)
	if err != nil {
		glog.Errorf("Failed to find the datacenter: %s. err: %+v", datacenterPath, err)
		return nil, err
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
		glog.Errorf("Failed to find the datacenter. err: %+v", err)
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
		glog.Errorf("Failed to find the datacenter. err: %+v", err)
		return 0, err
	}
	return len(datacenters), nil
}

// GetVMByDNSName gets the VM object from the given dns name
func (dc *Datacenter) GetVMByDNSName(ctx context.Context, dnsName string) (*VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	dnsName = strings.ToLower(strings.TrimSpace(dnsName))
	svm, err := s.FindByDnsName(ctx, dc.Datacenter, dnsName, true)
	if err != nil {
		glog.Errorf("Failed to find VM by DNS Name. VM DNS Name: %s, err: %+v", dnsName, err)
		return nil, err
	}
	if svm == nil {
		glog.Errorf("Unable to find VM by DNS Name. VM DNS Name: %s", dnsName)
		return nil, ErrNoVMFound
	}
	virtualMachine := VirtualMachine{object.NewVirtualMachine(dc.Client(), svm.Reference()), dc}
	return &virtualMachine, nil
}

// GetVMByUUID gets the VM object from the given vmUUID
func (dc *Datacenter) GetVMByUUID(ctx context.Context, vmUUID string) (*VirtualMachine, error) {
	s := object.NewSearchIndex(dc.Client())
	vmUUID = strings.ToLower(strings.TrimSpace(vmUUID))
	svm, err := s.FindByUuid(ctx, dc.Datacenter, vmUUID, true, nil)
	if err != nil {
		glog.Errorf("Failed to find VM by UUID. VM UUID: %s, err: %+v", vmUUID, err)
		return nil, err
	}
	if svm == nil {
		glog.Errorf("Unable to find VM by UUID. VM UUID: %s", vmUUID)
		return nil, ErrNoVMFound
	}
	virtualMachine := VirtualMachine{object.NewVirtualMachine(dc.Client(), svm.Reference()), dc}
	return &virtualMachine, nil
}

// GetVMByPath gets the VM object from the given vmPath
// vmPath should be the full path to VM and not just the name
func (dc *Datacenter) GetVMByPath(ctx context.Context, vmPath string) (*VirtualMachine, error) {
	finder := getFinder(dc)
	vm, err := finder.VirtualMachine(ctx, vmPath)
	if err != nil {
		glog.Errorf("Failed to find VM by Path. VM Path: %s, err: %+v", vmPath, err)
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
		glog.Errorf("Failed to get all the datastores. err: %+v", err)
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
		glog.Errorf("Failed to get Datastore managed objects from datastore objects."+
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
	glog.V(9).Infof("dsURLInfoMap : %+v", dsURLInfoMap)
	return dsURLInfoMap, nil
}

// GetDatastoreByPath gets the Datastore object from the given vmDiskPath
func (dc *Datacenter) GetDatastoreByPath(ctx context.Context, vmDiskPath string) (*DatastoreInfo, error) {
	datastorePathObj := new(object.DatastorePath)
	isSuccess := datastorePathObj.FromString(vmDiskPath)
	if !isSuccess {
		glog.Errorf("Failed to parse vmDiskPath: %s", vmDiskPath)
		return nil, errors.New("Failed to parse vmDiskPath")
	}

	return dc.GetDatastoreByName(ctx, datastorePathObj.Datastore)
}

// GetDatastoreByName gets the Datastore object for the given datastore name
func (dc *Datacenter) GetDatastoreByName(ctx context.Context, name string) (*DatastoreInfo, error) {
	finder := getFinder(dc)
	ds, err := finder.Datastore(ctx, name)
	if err != nil {
		glog.Errorf("Failed while searching for datastore: %s. err: %+v", name, err)
		return nil, err
	}

	var dsMo mo.Datastore
	pc := property.DefaultCollector(dc.Client())
	properties := []string{DatastoreInfoProperty}
	err = pc.RetrieveOne(ctx, ds.Reference(), properties, &dsMo)
	if err != nil {
		glog.Errorf("Failed to get Datastore managed objects from datastore objects."+
			" properties: %+v, err: %v", properties, err)
		return nil, err
	}

	return &DatastoreInfo{
		&Datastore{ds, dc},
		dsMo.Info.GetDatastoreInfo()}, nil
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
		glog.Errorf("Failed to get the ResourcePool for computePath '%s'. err: %+v", computePath, err)
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
		glog.Errorf("Failed to get the folder reference for %s. err: %+v", folderPath, err)
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
		glog.Error("VirtualMachine Object list is empty")
		return nil, fmt.Errorf("VirtualMachine Object list is empty")
	}

	for _, vmObj := range vmObjList {
		vmRefs = append(vmRefs, vmObj.Reference())
	}
	pc := property.DefaultCollector(dc.Client())
	err := pc.Retrieve(ctx, vmRefs, properties, &vmMoList)
	if err != nil {
		glog.Errorf("Failed to get VM managed objects from VM objects. vmObjList: %+v, properties: %+v, err: %v", vmObjList, properties, err)
		return nil, err
	}
	return vmMoList, nil
}

// GetVirtualDiskPage83Data gets the virtual disk UUID by diskPath
func (dc *Datacenter) GetVirtualDiskPage83Data(ctx context.Context, diskPath string) (string, error) {
	if len(diskPath) > 0 && filepath.Ext(diskPath) != ".vmdk" {
		diskPath += ".vmdk"
	}
	vdm := object.NewVirtualDiskManager(dc.Client())
	// Returns uuid of vmdk virtual disk
	diskUUID, err := vdm.QueryVirtualDiskUuid(ctx, diskPath, dc.Datacenter)

	if err != nil {
		glog.Warningf("QueryVirtualDiskUuid failed for diskPath: %q. err: %+v", diskPath, err)
		return "", err
	}
	diskUUID = formatVirtualDiskUUID(diskUUID)
	return diskUUID, nil
}

// GetDatastoreMoList gets the Datastore Managed Objects with the given properties from the datastore objects
func (dc *Datacenter) GetDatastoreMoList(ctx context.Context, dsObjList []*Datastore, properties []string) ([]mo.Datastore, error) {
	var dsMoList []mo.Datastore
	var dsRefs []types.ManagedObjectReference
	if len(dsObjList) < 1 {
		glog.Error("Datastore Object list is empty")
		return nil, fmt.Errorf("Datastore Object list is empty")
	}

	for _, dsObj := range dsObjList {
		dsRefs = append(dsRefs, dsObj.Reference())
	}
	pc := property.DefaultCollector(dc.Client())
	err := pc.Retrieve(ctx, dsRefs, properties, &dsMoList)
	if err != nil {
		glog.Errorf("Failed to get Datastore managed objects from datastore objects. dsObjList: %+v, properties: %+v, err: %v", dsObjList, properties, err)
		return nil, err
	}
	return dsMoList, nil
}

// CheckDisksAttached checks if the disk is attached to node.
// This is done by comparing the volume path with the backing.FilePath on the VM Virtual disk devices.
func (dc *Datacenter) CheckDisksAttached(ctx context.Context, nodeVolumes map[string][]string) (map[string]map[string]bool, error) {
	attached := make(map[string]map[string]bool)
	var vmList []*VirtualMachine
	for nodeName, volPaths := range nodeVolumes {
		for _, volPath := range volPaths {
			setNodeVolumeMap(attached, volPath, nodeName, false)
		}
		vm, err := dc.GetVMByPath(ctx, nodeName)
		if err != nil {
			if IsNotFound(err) {
				glog.Warningf("Node %q does not exist, vSphere CP will assume disks %v are not attached to it.", nodeName, volPaths)
			}
			continue
		}
		vmList = append(vmList, vm)
	}
	if len(vmList) == 0 {
		glog.V(2).Info("vSphere CP will assume no disks are attached to any node.")
		return attached, nil
	}
	vmMoList, err := dc.GetVMMoList(ctx, vmList, []string{"config.hardware.device", "name"})
	if err != nil {
		// When there is an error fetching instance information
		// it is safer to return nil and let volume information not be touched.
		glog.Errorf("Failed to get VM Managed object for nodes: %+v. err: +%v", vmList, err)
		return nil, err
	}

	for _, vmMo := range vmMoList {
		if vmMo.Config == nil {
			glog.Errorf("Config is not available for VM: %q", vmMo.Name)
			continue
		}
		for nodeName, volPaths := range nodeVolumes {
			if nodeName == vmMo.Name {
				verifyVolumePathsForVM(vmMo, volPaths, attached)
			}
		}
	}
	return attached, nil
}

// VerifyVolumePathsForVM verifies if the volume paths (volPaths) are attached to VM.
func verifyVolumePathsForVM(vmMo mo.VirtualMachine, volPaths []string, nodeVolumeMap map[string]map[string]bool) {
	// Verify if the volume paths are present on the VM backing virtual disk devices
	for _, volPath := range volPaths {
		vmDevices := object.VirtualDeviceList(vmMo.Config.Hardware.Device)
		for _, device := range vmDevices {
			if vmDevices.TypeName(device) == "VirtualDisk" {
				virtualDevice := device.GetVirtualDevice()
				if backing, ok := virtualDevice.Backing.(*types.VirtualDiskFlatVer2BackingInfo); ok {
					if backing.FileName == volPath {
						setNodeVolumeMap(nodeVolumeMap, volPath, vmMo.Name, true)
					}
				}
			}
		}
	}
}

func setNodeVolumeMap(
	nodeVolumeMap map[string]map[string]bool,
	volumePath string,
	nodeName string,
	check bool) {
	volumeMap := nodeVolumeMap[nodeName]
	if volumeMap == nil {
		volumeMap = make(map[string]bool)
		nodeVolumeMap[nodeName] = volumeMap
	}
	volumeMap[volumePath] = check
}

func (dc *Datacenter) GetAllDatastoreClusters(ctx context.Context, child bool) (map[string]*StoragePodInfo, error) {
	finder := getFinder(dc)
	storagePods, err := finder.DatastoreClusterList(ctx, "*")
	if err != nil {
		glog.Errorf("Failed to get all the datastore clusters. err: %+v", err)
		return nil, err
	}
	var spList []types.ManagedObjectReference
	for _, sp := range storagePods {
		spList = append(spList, sp.Reference())
	}

	var spMoList []mo.StoragePod
	pc := property.DefaultCollector(dc.Client())
	properties := []string{StoragePodDrsEntryProperty, StoragePodProperty}
	err = pc.Retrieve(ctx, spList, properties, &spMoList)
	if err != nil {
		glog.Errorf("Failed to get Datastore managed objects from datastore objects."+
			" dsObjList: %+v, properties: %+v, err: %v", spList, properties, err)
		return nil, err
	}

	spURLInfoMap := make(map[string]*StoragePodInfo)
	for _, spMo := range spMoList {
		spURLInfoMap[spMo.Summary.Name] = &StoragePodInfo{
			&StoragePod{
				dc,
				object.NewStoragePod(dc.Client(), spMo.Reference()),
				make([]*Datastore, 0),
			},
			spMo.Summary,
			&spMo.PodStorageDrsEntry.StorageDrsConfig,
			make([]*DatastoreInfo, 0),
		}

		if child {
			err := spURLInfoMap[spMo.Summary.Name].PopulateChildDatastoreInfos(ctx, false)
			if err != nil {
				glog.Warningf("PopulateChildDatastoreInfos Failed. Err: %v", err)
			}
		}
	}

	glog.V(9).Infof("spURLInfoMap : %+v", spURLInfoMap)
	return spURLInfoMap, nil
}

// GetDatastoreClusterByName gets the DatastoreCluster object for the given name
func (dc *Datacenter) GetDatastoreClusterByName(ctx context.Context, name string) (*StoragePodInfo, error) {
	finder := getFinder(dc)
	ds, err := finder.DatastoreCluster(ctx, name)
	if err != nil {
		glog.Errorf("Failed while searching for datastore cluster: %s. err: %+v", name, err)
		return nil, err
	}

	var spMo mo.StoragePod
	pc := property.DefaultCollector(dc.Client())
	properties := []string{StoragePodDrsEntryProperty, StoragePodProperty}
	err = pc.RetrieveOne(ctx, ds.Reference(), properties, &spMo)
	if err != nil {
		glog.Errorf("Failed to get Datastore managed objects from datastore objects."+
			" properties: %+v, err: %v", properties, err)
		return nil, err
	}

	return &StoragePodInfo{
		&StoragePod{
			dc,
			object.NewStoragePod(dc.Client(), spMo.Reference()),
			make([]*Datastore, 0),
		},
		spMo.Summary,
		&spMo.PodStorageDrsEntry.StorageDrsConfig,
		make([]*DatastoreInfo, 0),
	}, nil
}

func (dc *Datacenter) CreateFirstClassDisk(ctx context.Context,
	datastoreName string, datastoreType ParentDatastoreType,
	diskName string, diskSize int64) error {

	m := vslm.NewObjectManager(dc.Client())

	var pool *object.ResourcePool
	var ds types.ManagedObjectReference
	if datastoreType == TypeDatastoreCluster {
		storagePod, err := dc.GetDatastoreClusterByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreClusterByName failed. Err: %v", err)
			return err
		}
		ds = storagePod.Reference()

		pool, err = dc.GetResourcePool(ctx, "")
		if err != nil {
			glog.Errorf("GetResourcePool failed. Err: %v", err)
			return err
		}
	} else {
		datastore, err := dc.GetDatastoreByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreByName failed. Err: %v", err)
			return err
		}
		ds = datastore.Reference()
	}

	spec := types.VslmCreateSpec{
		Name:         diskName,
		CapacityInMB: diskSize,
		BackingSpec: &types.VslmCreateSpecDiskFileBackingSpec{
			VslmCreateSpecBackingSpec: types.VslmCreateSpecBackingSpec{
				Datastore: ds,
			},
			ProvisioningType: string(types.BaseConfigInfoDiskFileBackingInfoProvisioningTypeThin),
		},
	}

	if datastoreType == TypeDatastoreCluster {
		err := m.PlaceDisk(ctx, &spec, pool.Reference())
		if err != nil {
			glog.Errorf("PlaceDisk(%s) failed. Err: %v", diskName, err)
			return err
		}
	}

	task, err := m.CreateDisk(ctx, spec)
	if err != nil {
		glog.Errorf("CreateDisk(%s) failed. Err: %v", diskName, err)
		return err
	}

	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Wait(%s) failed. Err: %v", diskName, err)
		return err
	}

	return nil
}

func (dc *Datacenter) GetFirstClassDisk(ctx context.Context,
	datastoreName string, datastoreType ParentDatastoreType,
	diskID string, findBy FindFCD) (*FirstClassDiskInfo, error) {

	var fcd *FirstClassDiskInfo
	if datastoreType == TypeDatastoreCluster {
		storagePod, err := dc.GetDatastoreClusterByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreClusterByName failed. Err: %v", err)
			return nil, err
		}

		fcd, err = storagePod.GetFirstClassDiskInfo(ctx, diskID, findBy)
		if err != nil {
			glog.Errorf("GetFirstClassDiskByName failed. Err: %v", err)
			return nil, err
		}
	} else {
		datastore, err := dc.GetDatastoreByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreByName failed. Err: %v", err)
			return nil, err
		}

		fcd, err = datastore.GetFirstClassDiskInfo(ctx, diskID, findBy)
		if err != nil {
			glog.Errorf("GetFirstClassDiskByName failed. Err: %v", err)
			return nil, err
		}
	}

	return fcd, nil
}

func (dc *Datacenter) GetAllFirstClassDisks(ctx context.Context) ([]*FirstClassDiskInfo, error) {
	storagePods, err := dc.GetAllDatastoreClusters(ctx, true)
	if err != nil {
		glog.Errorf("GetAllDatastoreClusters failed. Err: %v", err)
		return nil, err
	}

	datastores, err := dc.GetAllDatastores(ctx)
	if err != nil {
		glog.Errorf("GetAllDatastores failed. Err: %v", err)
		return nil, err
	}

	alreadyVisited := make([]string, 0)
	firstClassDisks := make([]*FirstClassDiskInfo, 0)

	for _, storagePod := range storagePods {
		err := storagePod.PopulateChildDatastoreInfos(ctx, false)
		if err != nil {
			glog.Warningf("PopulateChildDatastores failed. Err: %v", err)
			continue
		}
		for _, datastore := range storagePod.DatastoreInfos {
			alreadyVisited = append(alreadyVisited, datastore.Info.Name)
		}

		disks, err := storagePod.ListFirstClassDisksInfo(ctx)
		if err != nil {
			glog.Warningf("ListFirstClassDisks failed for %s. Err: %v", storagePod.Name(), err)
			continue
		}

		firstClassDisks = append(firstClassDisks, disks...)
	}

	for _, datastore := range datastores {
		if ExistsInList(datastore.Info.Name, alreadyVisited, false) {
			continue
		}
		alreadyVisited = append(alreadyVisited, datastore.Info.Name)

		disks, err := datastore.ListFirstClassDiskInfos(ctx)
		if err != nil {
			glog.Warningf("ListFirstClassDisks failed for %s. Err: %v", datastore.Info.Name, err)
			continue
		}

		firstClassDisks = append(firstClassDisks, disks...)
	}

	return firstClassDisks, nil
}

func (dc *Datacenter) DoesFirstClassDiskExist(ctx context.Context, fcdID string) (*FirstClassDiskInfo, error) {
	datastores, err := dc.GetAllDatastores(ctx)
	if err != nil {
		glog.Errorf("GetAllDatastores failed. Err: %v", err)
		return nil, err
	}

	for _, datastore := range datastores {
		fcd, err := datastore.GetFirstClassDiskInfo(ctx, fcdID, FindFCDByID)
		if err == nil {
			glog.Infof("DoesFirstClassDiskExist(%s): FOUND", fcdID)
			return fcd, nil
		}
	}

	glog.Infof("DoesFirstClassDiskExist(%s): NOT FOUND", fcdID)
	return nil, ErrNoDiskIDFound
}

func (dc *Datacenter) DeleteFirstClassDisk(ctx context.Context,
	datastoreName string, datastoreType ParentDatastoreType, diskID string) error {

	var ds types.ManagedObjectReference
	if datastoreType == TypeDatastoreCluster {
		storagePod, err := dc.GetDatastoreClusterByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreClusterByName failed. Err: %v", err)
			return err
		}

		datastore, err := storagePod.GetDatastoreThatOwnsFCD(ctx, diskID)
		if err != nil {
			glog.Errorf("GetDatastoreThatOwnsFCD failed. Err: %v", err)
			return err
		}
		ds = datastore.Reference()
	} else {
		datastore, err := dc.GetDatastoreByName(ctx, datastoreName)
		if err != nil {
			glog.Errorf("GetDatastoreByName failed. Err: %v", err)
			return err
		}
		ds = datastore.Reference()
	}

	m := vslm.NewObjectManager(dc.Client())

	task, err := m.Delete(ctx, ds, diskID)
	if err != nil {
		glog.Errorf("Delete(%s) failed. Err: %v", diskID, err)
		return err
	}

	err = task.Wait(ctx)
	if err != nil {
		glog.Errorf("Wait(%s) failed. Err: %v", diskID, err)
		return err
	}

	return nil
}
