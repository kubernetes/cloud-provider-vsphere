/*
Copyright 2026 The Kubernetes Authors.

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

// Package ipfamily defines the IPFamily type and its four canonical values
// (IPv4, IPv6, IPv4IPv6, IPv6IPv4), along with the helper methods derived from
// them. It lives in a standalone package so that both the top-level
// vsphereparavirtual package and its sub-packages (e.g. routablepod) can
// reference it without creating import cycles.
package ipfamily

import (
	"fmt"
	"strings"
)

// IPFamily represents the cluster IP-address family configuration. The four
// canonical values map directly to the --cluster-ip-family flag accepted by
// the cloud-controller-manager.
type IPFamily string

const (
	// IPv4 is the default: single-stack IPv4. NodeInternalIPs are reported
	// IPv4-first. Does not require dual-stack VM Operator API fields.
	IPv4 IPFamily = "ipv4"

	// IPv6 is single-stack IPv6. NodeInternalIPs are reported IPv6-first.
	// Requires --vm-operator-api-version >= v1alpha6.
	IPv6 IPFamily = "ipv6"

	// IPv4IPv6 is dual-stack with IPv4 as the primary (first) family.
	// Requires --vm-operator-api-version >= v1alpha6 and --enable-vpc-mode=true
	// when the route controller is enabled.
	IPv4IPv6 IPFamily = "ipv4ipv6"

	// IPv6IPv4 is dual-stack with IPv6 as the primary (first) family.
	// Requires --vm-operator-api-version >= v1alpha6 and --enable-vpc-mode=true
	// when the route controller is enabled.
	IPv6IPv4 IPFamily = "ipv6ipv4"
)

// Parse validates a raw --cluster-ip-family flag value (case-insensitive,
// whitespace-trimmed) and returns the canonical IPFamily constant.
func Parse(raw string) (IPFamily, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(IPv4):
		return IPv4, nil
	case string(IPv6):
		return IPv6, nil
	case string(IPv4IPv6):
		return IPv4IPv6, nil
	case string(IPv6IPv4):
		return IPv6IPv4, nil
	default:
		return "", fmt.Errorf("invalid --cluster-ip-family %q: must be one of %s, %s, %s, %s",
			raw, IPv4, IPv6, IPv4IPv6, IPv6IPv4)
	}
}

// IPv4Enabled reports whether the IPv4 family is active (true for IPv4,
// IPv4IPv6, and IPv6IPv4).
func (f IPFamily) IPv4Enabled() bool {
	return f == IPv4 || f == IPv4IPv6 || f == IPv6IPv4
}

// IPv6Enabled reports whether the IPv6 family is active (true for IPv6,
// IPv4IPv6, and IPv6IPv4).
func (f IPFamily) IPv6Enabled() bool {
	return f == IPv6 || f == IPv4IPv6 || f == IPv6IPv4
}

// DualStack reports whether both IPv4 and IPv6 are active.
func (f IPFamily) DualStack() bool {
	return f.IPv4Enabled() && f.IPv6Enabled()
}

// PrimaryIPv4 reports whether IPv4 is the primary (first-listed) family. It
// is true for IPv4 and IPv4IPv6, false for IPv6 and IPv6IPv4.
func (f IPFamily) PrimaryIPv4() bool {
	return f == IPv4 || f == IPv4IPv6
}

// FamilyCount returns 1 for single-stack families and 2 for dual-stack.
func (f IPFamily) FamilyCount() int {
	if f.DualStack() {
		return 2
	}
	return 1
}
