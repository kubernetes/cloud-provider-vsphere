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
	"net"
	"strings"

	"github.com/pkg/errors"
	klog "k8s.io/klog/v2"
)

const (
	// ProviderPrefix is the Kubernetes cloud provider prefix for this
	// cloud provider.
	ProviderPrefix = ProviderName + "://"

	// MinUUIDLen is the min length for a valid UUID
	MinUUIDLen int = 36
)

// GetUUIDFromProviderID returns a UUID from the supplied cloud provider ID.
func GetUUIDFromProviderID(providerID string) string {
	withoutPrefix := strings.TrimPrefix(providerID, ProviderPrefix)
	return strings.ToLower(strings.TrimSpace(withoutPrefix))
}

// ConvertK8sUUIDtoNormal reformats UUID to match VMware's format:
//
// Endian Safe : https://www.dmtf.org/standards/smbios/
//
//	8   -  4 -  4 - 4  -    12
//
// K8s:    56492e42-22ad-3911-6d72-59cc8f26bc90
// VMware: 422e4956-ad22-1139-6d72-59cc8f26bc90
func ConvertK8sUUIDtoNormal(k8sUUID string) string {
	if len(k8sUUID) < MinUUIDLen {
		klog.Errorf("The UUID length is invalid. Returning UUID=%s as is.", k8sUUID)
		return k8sUUID
	}
	uuid := fmt.Sprintf("%s%s%s%s-%s%s-%s%s-%s-%s",
		k8sUUID[6:8], k8sUUID[4:6], k8sUUID[2:4], k8sUUID[0:2],
		k8sUUID[11:13], k8sUUID[9:11],
		k8sUUID[16:18], k8sUUID[14:16],
		k8sUUID[19:23],
		k8sUUID[24:36])
	return strings.ToLower(strings.TrimSpace(uuid))
}

// ErrOnLocalOnlyIPAddr returns an error if the provided IP address is
// accessible only on the VM's guest OS.
func ErrOnLocalOnlyIPAddr(addr string) error {
	var reason string
	a := net.ParseIP(addr)
	if a == nil {
		reason = "invalid"
	} else if a.IsUnspecified() {
		reason = "unspecified"
	} else if a.IsLinkLocalMulticast() {
		reason = "link-local-mutlicast"
	} else if a.IsLinkLocalUnicast() {
		reason = "link-local-unicast"
	} else if a.IsLoopback() {
		reason = "loopback"
	}
	if reason != "" {
		return errors.Errorf("failed to validate ip addr=%v: %s", addr, reason)
	}
	return nil
}

// ArrayContainsCaseInsensitive detects whether a given array of string contains
// the given string, ignoring case.
func ArrayContainsCaseInsensitive(arr []string, str string) bool {
	for _, a := range arr {
		if strings.EqualFold(a, str) {
			return true
		}
	}
	return false
}
