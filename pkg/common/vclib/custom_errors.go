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

import "errors"

// Error Messages
const (
	FileAlreadyExistErrMsg     = "File requested already exist"
	NoDevicesFoundErrMsg       = "No devices found"
	DiskNotFoundErrMsg         = "No vSphere disk ID/Name found"
	InvalidVolumeOptionsErrMsg = "VolumeOptions verification failed"
	NoVMFoundErrMsg            = "No VM found"
	NoZoneRegionFoundErrMsg    = "Unable to find the Zone/Region pair"
	NoDatastoreFoundErrMsg     = "Datastore not found"
	NoDatacenterFoundErrMsg    = "Datacenter not found"
)

// Error constants
var (
	ErrFileAlreadyExist     = errors.New(FileAlreadyExistErrMsg)
	ErrNoDevicesFound       = errors.New(NoDevicesFoundErrMsg)
	ErrNoDiskIDFound        = errors.New(DiskNotFoundErrMsg)
	ErrInvalidVolumeOptions = errors.New(InvalidVolumeOptionsErrMsg)
	ErrNoVMFound            = errors.New(NoVMFoundErrMsg)
	ErrNoZoneRegionFound    = errors.New(NoZoneRegionFoundErrMsg)
	ErrNoDatastoreFound     = errors.New(NoDatastoreFoundErrMsg)
	ErrNoDatacenterFound    = errors.New(NoDatacenterFoundErrMsg)
)
