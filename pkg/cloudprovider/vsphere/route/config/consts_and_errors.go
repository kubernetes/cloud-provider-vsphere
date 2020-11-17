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
	"time"
)

const (
	// ClusterNameTagScope is the scope of clusterName tag
	// NSXT Tag includes scope and tag value, the format is a JSON map with name/value pair,
	// for example {"scope": "vsphere.k8s.io/cluster-name", "tag": "kubernetes-cluster-1"}
	ClusterNameTagScope = "vsphere.k8s.io/cluster-name"
	// NodeNameTagScope is the scope of nodeName tag
	// Node name tag will be used to identify static route belongs to which node,
	// for example {"scope": "vsphere.k8s.io/node-name", "tag": "worker-node-1"}
	NodeNameTagScope = "vsphere.k8s.io/node-name"

	// RealizedStateTimeout is the timeout duration for realized state check
	RealizedStateTimeout = 10 * time.Second
	// RealizedStateSleepTime is the interval between realized state check
	RealizedStateSleepTime = 1 * time.Second
	// RealizedState is the realized state
	RealizedState = "REALIZED"

	// DisplayNameMaxLength is the maximum length of static route display name
	DisplayNameMaxLength = 255
)
