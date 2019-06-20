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
	"strings"
	"testing"
)

func TestInvalidProviderID(t *testing.T) {
	providerID := ""

	UUID := GetUUIDFromProviderID(providerID)

	if UUID != "" {
		t.Errorf("Should return an empty string")
	}
}

func TestUpperUUIDFromProviderID(t *testing.T) {
	tmpUUID := strings.ToUpper("423740e7-c66e-05e3-9d0b-9e1205b24d43")
	providerID := ProviderPrefix + tmpUUID

	UUID := GetUUIDFromProviderID(providerID)

	if UUID != "423740e7-c66e-05e3-9d0b-9e1205b24d43" {
		t.Errorf("Failed to extract UUID")
	}
}

func TestUUIDFromProviderID(t *testing.T) {
	providerID := "vsphere://423740e7-c66e-05e3-9d0b-9e1205b24d43"

	UUID := GetUUIDFromProviderID(providerID)

	if UUID != "423740e7-c66e-05e3-9d0b-9e1205b24d43" {
		t.Errorf("Failed to extract UUID")
	}
}

func TestUUIDFromUUID(t *testing.T) {
	UUIDOrg := "423740e7-c66e-05e3-9d0b-9e1205b24d43"

	UUIDNew := GetUUIDFromProviderID(UUIDOrg)

	if UUIDOrg != UUIDNew {
		t.Errorf("Failed to just return the UUID")
	}
}

func TestUUIDConvertInvalid(t *testing.T) {
	k8sUUID := ""

	biosUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	if biosUUID != "" {
		t.Errorf("Should return empty string")
	}
}

func TestUUIDConvert(t *testing.T) {
	k8sUUID := "56492e42-22ad-3911-6d72-59cc8f26bc90"

	biosUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	if biosUUID != "422e4956-ad22-1139-6d72-59cc8f26bc90" {
		t.Errorf("Failed to translate UUID")
	}
}

func TestUpperUUIDConvert(t *testing.T) {
	k8sUUID := strings.ToUpper("422e4956-ad22-1139-6d72-59cc8f26bc90")

	biosUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	if biosUUID != "56492e42-22ad-3911-6d72-59cc8f26bc90" {
		t.Errorf("Failed to translate UUID")
	}
}

func TestUUIDConvertAndRevert(t *testing.T) {
	k8sUUID := "42278c9d-79fb-f2af-b060-d7f167fa261c"

	//converts
	tmpUUID := ConvertK8sUUIDtoNormal(k8sUUID)

	//reverts to original
	orgUUID := ConvertK8sUUIDtoNormal(tmpUUID)

	if orgUUID != "42278c9d-79fb-f2af-b060-d7f167fa261c" {
		t.Errorf("Failed to revert UUID")
	}
}
