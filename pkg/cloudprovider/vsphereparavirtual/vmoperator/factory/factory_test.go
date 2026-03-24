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

package factory_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/factory"
)

func TestNewAdapter_SupportedVersions(t *testing.T) {
	// A real rest.Config is used here intentionally. dynamic.NewForConfig only
	// parses the config struct; it does not dial the server or perform any
	// network I/O, so this test remains hermetic and requires no fake server.
	cfg := &rest.Config{Host: "https://fake-host"}
	for _, version := range []string{factory.V1alpha2, factory.V1alpha5} {
		t.Run(version, func(t *testing.T) {
			adapter, err := factory.NewAdapter(version, cfg)
			assert.NoError(t, err)
			assert.NotNil(t, adapter)
			assert.NotNil(t, adapter.VirtualMachines())
			assert.NotNil(t, adapter.VirtualMachineServices())
		})
	}
}

func TestNewAdapter_UnsupportedVersion(t *testing.T) {
	cfg := &rest.Config{Host: "https://fake-host"}
	testCases := []struct {
		name    string
		version string
	}{
		{name: "unknown version string", version: "v1alpha6"},
		{name: "empty version string", version: ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := factory.NewAdapter(tc.version, cfg)
			assert.Error(t, err)
			assert.Nil(t, adapter)
			assert.Contains(t, err.Error(), "unsupported vm-operator-api-version")
			assert.Contains(t, err.Error(), factory.V1alpha2)
			assert.Contains(t, err.Error(), factory.V1alpha5)
		})
	}
}
