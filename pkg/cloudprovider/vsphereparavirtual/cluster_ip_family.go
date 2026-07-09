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

package vsphereparavirtual

import (
	"fmt"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ipfamily"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/factory"
)

// The four ClusterIPFamily constants are aliases for the canonical values in
// the ipfamily package. Keeping them here preserves backward compatibility for
// any callers in this package (instances.go, cloud.go, tests) that already
// reference them by the ClusterIPFamily* name.
const (
	ClusterIPFamilyIPv4     = string(ipfamily.IPv4)
	ClusterIPFamilyIPv6     = string(ipfamily.IPv6)
	ClusterIPFamilyIPv4IPv6 = string(ipfamily.IPv4IPv6)
	ClusterIPFamilyIPv6IPv4 = string(ipfamily.IPv6IPv4)
)

// ParseClusterIPFamily validates the --cluster-ip-family flag and returns a
// canonical IPFamily value. It is a thin wrapper around ipfamily.Parse.
func ParseClusterIPFamily(raw string) (ipfamily.IPFamily, error) {
	return ipfamily.Parse(raw)
}

// supportsDualStackVMService records, per --vm-operator-api-version, whether
// that API persists the dual-stack fields on VirtualMachineService
// (ipFamilies, ipFamilyPolicy). It must stay in sync with factory.NewAdapter:
// every version accepted there must appear here. A new API version that
// supports dual-stack VirtualMachineService fields should be added with the
// value true; older or partial APIs should be added with false.
// When adding a new supported version, add a case to TestVmopSupportsDualStackVMServiceAPI.
var supportsDualStackVMService = map[string]bool{
	factory.V1alpha2: false,
	factory.V1alpha5: false,
	factory.V1alpha6: true,
}

// vmopSupportsDualStackVMServiceAPI reports whether vmopAPIVersion is known
// and persists VirtualMachineService dual-stack fields. An unknown version
// returns false; add new versions to supportsDualStackVMService before
// relying on this function.
func vmopSupportsDualStackVMServiceAPI(version string) bool {
	return supportsDualStackVMService[version]
}

// validateIPFamilyConfig ensures ipv6 / ipv4ipv6 / ipv6ipv4 are only used with
// a VM Operator API version that persists VirtualMachineService dual-stack
// fields (see supportsDualStackVMService). ipv4 does not require dual-stack
// VirtualMachineService fields.
func validateIPFamilyConfig(f ipfamily.IPFamily, vmopAPIVersion string) error {
	if !f.IPv6Enabled() {
		return nil
	}
	if !vmopSupportsDualStackVMServiceAPI(vmopAPIVersion) {
		return fmt.Errorf(
			"--cluster-ip-family=%s requires --vm-operator-api-version >= %s (dual-stack VirtualMachineService fields), got %q; "+
				"earlier API versions omit ipFamilies/ipFamilyPolicy and can silently provision IPv4-only load balancers",
			f, factory.V1alpha6, vmopAPIVersion,
		)
	}
	return nil
}
