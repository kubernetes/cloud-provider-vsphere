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

package connectionmanager

import (
	"errors"
)

// Error Messages
const (
	ConnectionNotFoundErrMsg       = "vCenter not found"
	MustHaveAtLeastOneVCDCErrMsg   = "Must have at least one vCenter/Datacenter configured"
	MultiVCRequiresZonesErrMsg     = "The use of multiple vCenters require the use of zones"
	MultiDCRequiresZonesErrMsg     = "The use of multiple Datacenters within a vCenter require the use of zones"
	UnsupportedConfigurationErrMsg = "Unsupported configuration"
)

// Error constants
var (
	ErrConnectionNotFound       = errors.New(ConnectionNotFoundErrMsg)
	ErrMustHaveAtLeastOneVCDC   = errors.New(MustHaveAtLeastOneVCDCErrMsg)
	ErrMultiVCRequiresZones     = errors.New(MultiVCRequiresZonesErrMsg)
	ErrMultiDCRequiresZones     = errors.New(MultiDCRequiresZonesErrMsg)
	ErrUnsupportedConfiguration = errors.New(UnsupportedConfigurationErrMsg)
)
