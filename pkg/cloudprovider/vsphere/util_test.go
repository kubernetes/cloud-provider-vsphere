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
	"testing"
)

func TestUUIDConvert1(t *testing.T) {
	k8sUUID := "56492e42-22ad-3911-6d72-59cc8f26bc90"

	biosUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	if biosUUID != "422e4956-ad22-1139-6d72-59cc8f26bc90" {
		t.Errorf("Failed to translate UUID")
	}
}

func TestUUIDConvert2(t *testing.T) {
	k8sUUID := "422e4956-ad22-1139-6d72-59cc8f26bc90"

	biosUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	if biosUUID != "56492e42-22ad-3911-6d72-59cc8f26bc90" {
		t.Errorf("Failed to translate UUID")
	}
}
