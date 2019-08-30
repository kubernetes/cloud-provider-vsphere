/*
Copyright 2019 The Kubernetes Authors.

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

package connectionmanager

import (
	"errors"
)

// FindVM is the type that represents the types of searches used to
// discover VMs.
type FindVM int

const (
	// FindVMByUUID finds VMs with the provided UUID.
	FindVMByUUID FindVM = iota // 0
	// FindVMByName finds VMs with the provided name.
	FindVMByName // 1
	// FindVMByIP finds VMs with the provided IP adress.
	FindVMByIP // 2

	// PoolSize is the number of goroutines used in parallel to find a VM.
	PoolSize int = 8

	// QueueSize is the size of the channel buffer used to find objects.
	// Only QueueSize objects may be placed into the queue before blocking.
	QueueSize int = PoolSize * 10

	// NumConnectionAttempts is the number of allowed connection attempts
	// before an error is returned.
	NumConnectionAttempts int = 3

	// RetryAttemptDelaySecs is the number of seconds to wait between
	// connection attempts.
	RetryAttemptDelaySecs int = 1
)

// Error Messages
const (
	ConnectionNotFoundErrMsg       = "vCenter not found"
	MustHaveAtLeastOneVCDCErrMsg   = "Must have at least one vCenter/Datacenter configured"
	MultiVCRequiresZonesErrMsg     = "The use of multiple vCenters require the use of zones"
	MultiDCRequiresZonesErrMsg     = "The use of multiple Datacenters within a vCenter require the use of zones"
	UnsupportedConfigurationErrMsg = "Unsupported configuration"
	UnableToFindCredentialManager  = "Unable to find Credential Manager"
)

// Error constants
var (
	ErrConnectionNotFound            = errors.New(ConnectionNotFoundErrMsg)
	ErrMustHaveAtLeastOneVCDC        = errors.New(MustHaveAtLeastOneVCDCErrMsg)
	ErrMultiVCRequiresZones          = errors.New(MultiVCRequiresZonesErrMsg)
	ErrMultiDCRequiresZones          = errors.New(MultiDCRequiresZonesErrMsg)
	ErrUnsupportedConfiguration      = errors.New(UnsupportedConfigurationErrMsg)
	ErrUnableToFindCredentialManager = errors.New(UnableToFindCredentialManager)
)
