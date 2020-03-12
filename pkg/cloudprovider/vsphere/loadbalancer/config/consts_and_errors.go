/*
 Copyright 2020 The Kubernetes Authors.

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

package config

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

const (
	// DefaultLoadBalancerClass is the default load balancer class
	DefaultLoadBalancerClass = "default"
)

// LoadBalancerSizes contains the valid size names
var LoadBalancerSizes = sets.NewString(
	model.LBService_SIZE_SMALL,
	model.LBService_SIZE_MEDIUM,
	model.LBService_SIZE_LARGE,
	model.LBService_SIZE_XLARGE,
	model.LBService_SIZE_DLB,
)
