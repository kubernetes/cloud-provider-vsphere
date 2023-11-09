/*
Copyright 2021 The Kubernetes Authors.
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

package helper

import (
	"fmt"
	"time"
)

const (
	// DefaultResyncTime is the default period for ippool informer to do re-sync
	DefaultResyncTime time.Duration = time.Minute * 1
)

const (
	// IPFamilyDefault is default value of ipFamily in v1alpha1 ippool
	IPFamilyDefault = "ipv4"
	// IPFamilyDefaultV2 is default value of ipFamily in v1alpha2 ippool
	IPFamilyDefaultV2 = "IPv4"
	// PrefixLengthDefault is default value of prefixLength
	PrefixLengthDefault = 24
)

// NSXIPPool defines an interface that is used to represent different versions nsx.vmware.com ipppol
type NSXIPPool interface{}

// IppoolNameFromClusterName returns the ippool name constructed using the cluster name
func IppoolNameFromClusterName(clusterName string) string {
	return fmt.Sprintf("%s-ippool", clusterName)
}
