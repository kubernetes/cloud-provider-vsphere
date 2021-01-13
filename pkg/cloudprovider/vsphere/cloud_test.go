/*
Copyright 2021 The Kubernetes Authors.

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
	"os"
	"reflect"
	"testing"

	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
)

func Test_validateDualStack(t *testing.T) {
	testcases := []struct {
		name          string
		gateEnabled   bool
		cfg           *ccfg.CPIConfig
		expectedError error
	}{
		{
			name:        "config dual-stack, gate enabled",
			gateEnabled: true,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4", "v6"},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "config dual-stack, gate disabled",
			gateEnabled: false,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4", "v6"},
						},
					},
				},
			},
			expectedError: fmt.Errorf("mulitple IP families specified for virtual center %q but ENABLE_ALPHA_DUAL_STACK env var is not set", "vcenter.local"),
		},
		{
			name:        "config single-stack, gate enabled",
			gateEnabled: true,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4"},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "config single-stack, gate disabled",
			gateEnabled: false,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4"},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "config default, gate enabled",
			gateEnabled: true,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4"},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name:        "config default, gate disabled",
			gateEnabled: false,
			cfg: &ccfg.CPIConfig{
				Config: vcfg.Config{
					VirtualCenter: map[string]*vcfg.VirtualCenterConfig{
						"vcenter.local": {
							IPFamilyPriority: []string{"v4"},
						},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			if testcase.gateEnabled {
				if err := os.Setenv(dualStackFeatureGateEnv, "true"); err != nil {
					t.Fatalf("failed to set ENABLE_ALPHA_DUAL_STACK: %v", err)
				}

				defer os.Unsetenv(dualStackFeatureGateEnv)
			}

			err := validateDualStack(testcase.cfg)
			if !reflect.DeepEqual(err, testcase.expectedError) {
				t.Logf("actual error: %v", err)
				t.Logf("expected error: %v", err)
				t.Error("unexpected error")
			}
		})
	}
}
