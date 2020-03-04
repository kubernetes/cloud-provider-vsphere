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

package loadbalancer

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	vapi_errors "github.com/vmware/vsphere-automation-sdk-go/lib/vapi/std/errors"
)

func namespacedNameFromService(service *corev1.Service) types.NamespacedName {
	return types.NamespacedName{Namespace: service.Namespace, Name: service.Name}
}

func parseNamespacedName(name string) types.NamespacedName {
	parts := strings.Split(name, "/")
	return types.NamespacedName{Namespace: parts[0], Name: parts[1]}
}

func collectNodeInternalAddresses(nodes []*corev1.Node) map[string]string {
	set := map[string]string{}
	for _, node := range nodes {
		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				set[addr.Address] = node.Name
				break
			}
		}
	}
	return set
}

func strptr(s string) *string {
	return &s
}

func isNotFoundError(err error) bool {
	_, ok := err.(vapi_errors.NotFound)
	return ok
}

func boolptr(b bool) *bool {
	return &b
}

func int64ptr(i int64) *int64 {
	return &i
}

func safeEquals(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
