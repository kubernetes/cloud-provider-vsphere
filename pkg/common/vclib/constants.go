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

// FindFCD is the type that represents the types of searches used to
// discover FCDs.
type FindFCD int

const (
	// FindFCDByID finds FCDs with the provided ID.
	FindFCDByID FindFCD = iota // 0

	// FindFCDByName finds FCDs with the provided name.
	FindFCDByName // 1
)

// Volume Constnts
const (
	// ThinDiskType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	ThinDiskType = "thin"
	// PreallocatedDiskType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	PreallocatedDiskType = "preallocated"
	// EagerZeroedThickDiskType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	EagerZeroedThickDiskType = "eagerZeroedThick"
	// ZeroedThickDiskType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	ZeroedThickDiskType = "zeroedThick"
)

// Controller Constants
const (
	// SCSIControllerLimit is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	SCSIControllerLimit = 4
	// SCSIControllerDeviceLimit is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	SCSIControllerDeviceLimit = 15
	// SCSIDeviceSlots is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	SCSIDeviceSlots = 16
	// SCSIReservedSlot is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	SCSIReservedSlot = 7

	// SCSIControllerType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	SCSIControllerType = "scsi"
	// LSILogicControllerType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	LSILogicControllerType = "lsiLogic"
	// BusLogicControllerType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	BusLogicControllerType = "busLogic"
	// LSILogicSASControllerType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	LSILogicSASControllerType = "lsiLogic-sas"
	// PVSCSIControllerType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	PVSCSIControllerType = "pvscsi"
)

// Other Constants
const (
	// LogLevel is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	LogLevel = 4
	// DatastoreProperty is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	DatastoreProperty = "datastore"
	// ResourcePoolProperty is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	ResourcePoolProperty = "resourcePool"
	// DatastoreInfoProperty is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	DatastoreInfoProperty = "info"
	// StoragePodDrsEntryProperty is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	StoragePodDrsEntryProperty = "podStorageDrsEntry"
	// StoragePodProperty is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	StoragePodProperty = "summary"
	// VirtualMachineType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	VirtualMachineType = "VirtualMachine"
	// RoundTripperDefaultCount is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	RoundTripperDefaultCount = 3
	// VSANDatastoreType is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	VSANDatastoreType = "vsan"
	// DummyVMPrefixName is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	DummyVMPrefixName = "vsphere-k8s"
	// ActivePowerState is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	ActivePowerState = "poweredOn"
)

// Test Constants
const (
	// TestDefaultDatacenter is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	TestDefaultDatacenter = "DC0"
	// TestDefaultDatastore is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	TestDefaultDatastore = "LocalDS_0"
	// TestDefaultNetwork is a good constant, yes it is!
	// TODO(?) Provide better documentation.
	TestDefaultNetwork = "VM Network"

	testNameNotFound = "enoent"
)
