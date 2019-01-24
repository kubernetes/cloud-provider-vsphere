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

package vsphere

import (
	"fmt"
	"strings"
)

const (
	ProviderPrefix = "vsphere://"
)

func GetUUIDFromProviderID(providerID string) string {
	return strings.TrimPrefix(providerID, ProviderPrefix)
}

// Reformats UUID to match vSphere format
// Endian Safe : https://www.dmtf.org/standards/smbios/
//            8   -  4 -  4 - 4  -    12
//K8s:    56492e42-22ad-3911-6d72-59cc8f26bc90
//VMware: 422e4956-ad22-1139-6d72-59cc8f26bc90
func ConvertK8sUUIDtoNormal(k8sUUID string) string {
	uuid := fmt.Sprintf("%s%s%s%s-%s%s-%s%s-%s-%s",
		k8sUUID[6:8], k8sUUID[4:6], k8sUUID[2:4], k8sUUID[0:2],
		k8sUUID[11:13], k8sUUID[9:11],
		k8sUUID[16:18], k8sUUID[14:16],
		k8sUUID[19:23],
		k8sUUID[24:36])
	return strings.ToLower(uuid)
}
