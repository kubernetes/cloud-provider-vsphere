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
	"github.com/vmware/govmomi/vim25/types"
)

// ParentDatastoreType represents the possible parent types of a datastore.
type ParentDatastoreType string

const (
	// TypeDatastore is a datastore parent that's another datastore.
	TypeDatastore ParentDatastoreType = "Datastore"

	// TypeDatastoreCluster is a datastore parent that's a cluster.
	TypeDatastoreCluster ParentDatastoreType = "DatastoreCluster"
)

// FirstClassDisk extends the govmomi FirstClassDisk object
type FirstClassDisk struct {
	Datacenter *Datacenter
	*types.VStorageObject
	ParentType ParentDatastoreType

	Datastore  *Datastore
	StoragePod *StoragePod
}

// FirstClassDiskInfo extends the govmomi FirstClassDisk object
type FirstClassDiskInfo struct {
	*FirstClassDisk

	DatastoreInfo  *DatastoreInfo
	StoragePodInfo *StoragePodInfo
}
