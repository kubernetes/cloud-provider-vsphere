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
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

func TestCreateNodeAddressesOrdersInterfaceFallback(t *testing.T) {
	tests := []struct {
		name       string
		family     string
		primaryIP4 string
		primaryIP6 string
		interfaces []string
		want       []string
	}{
		{
			name: "IPv6 preferred with only IPv4 primary", family: ClusterIPFamilyIPv6IPv4,
			primaryIP4: "10.0.0.1", interfaces: []string{"2001:db8::1"},
			want: []string{"2001:db8::1", "10.0.0.1"},
		},
		{
			name: "IPv6 preferred with ambiguous IPv6 primary", family: ClusterIPFamilyIPv6IPv4,
			primaryIP4: "10.0.0.1", interfaces: []string{"10.0.0.1", "2001:db8::a", "2001:db8::b"},
			want: []string{"2001:db8::a", "2001:db8::b", "10.0.0.1"},
		},
		{
			name: "IPv4 preferred with only IPv6 primary", family: ClusterIPFamilyIPv4IPv6,
			primaryIP6: "2001:db8::1", interfaces: []string{"10.0.0.1"},
			want: []string{"10.0.0.1", "2001:db8::1"},
		},
		{
			name: "IPv6 preferred with interleaved interfaces only", family: ClusterIPFamilyIPv6IPv4,
			interfaces: []string{"10.0.0.1", "2001:db8::2", "10.0.0.2", "2001:db8::1"},
			want:       []string{"2001:db8::2", "2001:db8::1", "10.0.0.1", "10.0.0.2"},
		},
		{
			name: "IPv4 preferred with interleaved interfaces only", family: ClusterIPFamilyIPv4IPv6,
			interfaces: []string{"2001:db8::1", "10.0.0.2", "2001:db8::2", "10.0.0.1"},
			want:       []string{"10.0.0.2", "10.0.0.1", "2001:db8::1", "2001:db8::2"},
		},
		{
			name: "IPv6 primary precedes additional IPv6 and IPv4", family: ClusterIPFamilyIPv6IPv4,
			primaryIP4: "10.0.0.1", primaryIP6: "2001:db8::1",
			interfaces: []string{"10.0.0.2", "2001:db8::2", "2001:0db8::1", "10.0.0.1", "2001:db8::2", "fe80::1", "169.254.1.1", "invalid"},
			want:       []string{"2001:db8::1", "2001:db8::2", "10.0.0.1", "10.0.0.2"},
		},
		{
			name: "IPv4 primary precedes additional IPv4 and IPv6", family: ClusterIPFamilyIPv4IPv6,
			primaryIP4: "10.0.0.1", primaryIP6: "2001:db8::1",
			interfaces: []string{"2001:db8::2", "10.0.0.2", "::ffff:10.0.0.1", "fe80::1", "invalid"},
			want:       []string{"10.0.0.1", "10.0.0.2", "2001:db8::1", "2001:db8::2"},
		},
		{
			name: "IPv6 single stack filters IPv4 primary and interfaces", family: ClusterIPFamilyIPv6,
			primaryIP4: "10.0.0.1", interfaces: []string{"10.0.0.2", "2001:db8::1"},
			want: []string{"2001:db8::1"},
		},
		{
			name: "IPv4 single stack filters IPv6 primary and interfaces", family: ClusterIPFamilyIPv4,
			primaryIP6: "2001:db8::1", interfaces: []string{"2001:db8::2", "10.0.0.1"},
			want: []string{"10.0.0.1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vm := &vmoptypes.VirtualMachineInfo{
				PrimaryIP4: tt.primaryIP4, PrimaryIP6: tt.primaryIP6,
				NetworkInterfaceAddresses: append([]string(nil), tt.interfaces...),
			}
			addresses := createNodeAddresses(vm, tt.family)
			assert.Equal(t, tt.want, internalIPs(addresses))
			assert.Equal(t, v1.NodeAddress{Type: v1.NodeHostName}, addresses[len(addresses)-1])
			assert.Len(t, addresses, len(tt.want)+1)
			assert.Equal(t, tt.interfaces, vm.NetworkInterfaceAddresses, "input interface order must not change")
		})
	}
}
