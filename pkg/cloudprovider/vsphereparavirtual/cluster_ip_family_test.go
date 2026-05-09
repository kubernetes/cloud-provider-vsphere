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
	"github.com/stretchr/testify/require"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/factory"
)

// TestVmopSupportsDualStackVMServiceAPI must be updated when vmopAPILevel gains
// entries (e.g. a new VM Operator API with level >= 60 for dual-stack fields).
func TestVmopSupportsDualStackVMServiceAPI(t *testing.T) {
	testCases := []struct {
		name string
		ver  string
		want bool
	}{
		{name: "v1alpha2 below threshold", ver: factory.V1alpha2, want: false},
		{name: "v1alpha5 below threshold", ver: factory.V1alpha5, want: false},
		{name: "v1alpha6 meets threshold", ver: factory.V1alpha6, want: true},
		{name: "unknown version", ver: "unknown", want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, vmopSupportsDualStackVMServiceAPI(tc.ver))
		})
	}
}

func TestParseClusterIPFamily(t *testing.T) {
	testCases := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "ipv4 canonical", raw: "ipv4", want: ClusterIPFamilyIPv4},
		{name: "IPv4 case", raw: "IPv4", want: ClusterIPFamilyIPv4},
		{name: "ipv6 with spaces", raw: "  ipv6 ", want: ClusterIPFamilyIPv6},
		{name: "ipv4ipv6", raw: "ipv4ipv6", want: ClusterIPFamilyIPv4IPv6},
		{name: "IPv4IPv6 mixed case", raw: "IPv4IPv6", want: ClusterIPFamilyIPv4IPv6},
		{name: "ipv6ipv4", raw: "ipv6ipv4", want: ClusterIPFamilyIPv6IPv4},
		{name: "IPv6IPv4 mixed case", raw: "IPv6IPv4", want: ClusterIPFamilyIPv6IPv4},
		{name: "garbage", raw: "dual-stack", wantErr: true},
		{name: "empty after trim", raw: "   ", wantErr: true},
		{name: "typo v6", raw: "ip6", wantErr: true},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseClusterIPFamily(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// simulateInitializeIPFamilyChecks mirrors VSphereParavirtual.Initialize order:
// parse flag value first, then validate against vm-operator-api-version.
func simulateInitializeIPFamilyChecks(rawClusterIPFamily, vmopAPIVersion string) error {
	normalized, err := ParseClusterIPFamily(rawClusterIPFamily)
	if err != nil {
		return err
	}
	return validateIPFamilyConfig(normalized, vmopAPIVersion)
}

// The following tests exercise the combined parse+validate flow, not the individual functions.
func TestSimulateInitializeIPFamilyChecks_Order(t *testing.T) {
	t.Run("illegal family fails at parse not at vmop gate", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("not-a-family", factory.V1alpha2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --cluster-ip-family")
	})

	t.Run("ipv6 with v1alpha2 fails at validate after successful parse", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("IPv6", factory.V1alpha2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ">= "+factory.V1alpha6)
	})

	t.Run("ipv6ipv4 with v1alpha5 fails validate", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("ipv6ipv4", factory.V1alpha5)
		require.Error(t, err)
		assert.Contains(t, err.Error(), ">= "+factory.V1alpha6)
	})

	t.Run("ipv6ipv4 with v1alpha6 succeeds", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("IPv6IPv4", factory.V1alpha6)
		assert.NoError(t, err)
	})

	t.Run("ipv6 with v1alpha6 succeeds", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("ipv6", factory.V1alpha6)
		assert.NoError(t, err)
	})

	t.Run("ipv4 with v1alpha2 succeeds without needing v1alpha6", func(t *testing.T) {
		err := simulateInitializeIPFamilyChecks("ipv4", factory.V1alpha2)
		assert.NoError(t, err)
	})
}

func TestValidateIPFamilyConfig(t *testing.T) {
	testCases := []struct {
		name            string
		clusterIPFamily string
		vmopAPIVersion  string
		wantErr         bool
	}{
		{
			name:            "ipv4 + v1alpha2 is always valid (default path)",
			clusterIPFamily: ClusterIPFamilyIPv4,
			vmopAPIVersion:  factory.V1alpha2,
			wantErr:         false,
		},
		{
			name:            "ipv4 + v1alpha5 is valid",
			clusterIPFamily: ClusterIPFamilyIPv4,
			vmopAPIVersion:  factory.V1alpha5,
			wantErr:         false,
		},
		{
			name:            "ipv4 + v1alpha6 is valid",
			clusterIPFamily: ClusterIPFamilyIPv4,
			vmopAPIVersion:  factory.V1alpha6,
			wantErr:         false,
		},
		{
			name:            "ipv6 + v1alpha6 is valid",
			clusterIPFamily: ClusterIPFamilyIPv6,
			vmopAPIVersion:  factory.V1alpha6,
			wantErr:         false,
		},
		{
			name:            "ipv4ipv6 + v1alpha6 is valid",
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			vmopAPIVersion:  factory.V1alpha6,
			wantErr:         false,
		},
		{
			name:            "ipv6 + v1alpha2 is rejected",
			clusterIPFamily: ClusterIPFamilyIPv6,
			vmopAPIVersion:  factory.V1alpha2,
			wantErr:         true,
		},
		{
			name:            "ipv6 + v1alpha5 is rejected",
			clusterIPFamily: ClusterIPFamilyIPv6,
			vmopAPIVersion:  factory.V1alpha5,
			wantErr:         true,
		},
		{
			name:            "ipv4ipv6 + v1alpha2 is rejected",
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			vmopAPIVersion:  factory.V1alpha2,
			wantErr:         true,
		},
		{
			name:            "ipv4ipv6 + v1alpha5 is rejected",
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			vmopAPIVersion:  factory.V1alpha5,
			wantErr:         true,
		},
		{
			name:            "ipv6ipv4 + v1alpha6 is valid",
			clusterIPFamily: ClusterIPFamilyIPv6IPv4,
			vmopAPIVersion:  factory.V1alpha6,
			wantErr:         false,
		},
		{
			name:            "ipv6ipv4 + v1alpha2 is rejected",
			clusterIPFamily: ClusterIPFamilyIPv6IPv4,
			vmopAPIVersion:  factory.V1alpha2,
			wantErr:         true,
		},
		{
			name:            "unknown vmop version is treated as below v1alpha6 for dual-stack family",
			clusterIPFamily: ClusterIPFamilyIPv6IPv4,
			vmopAPIVersion:  "v1alpha99",
			wantErr:         true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateIPFamilyConfig(tc.clusterIPFamily, tc.vmopAPIVersion)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
