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

// Package factory constructs the vmoperator.Interface adapter for a given API version.
// The version is supplied via the --vm-operator-api-version flag at startup.
package factory

import (
	"fmt"
	"strings"

	"k8s.io/client-go/rest"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	adapterv2 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha2"
	adapterv5 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha5"
)

// Supported VM Operator API version strings for the --vm-operator-api-version flag.
const (
	V1alpha2 = "v1alpha2"
	V1alpha5 = "v1alpha5"
)

var supportedVersions = []string{V1alpha2, V1alpha5}

// NewAdapter returns a vmoperator.Interface for the requested API version.
func NewAdapter(version string, cfg *rest.Config) (vmop.Interface, error) {
	switch version {
	case V1alpha2:
		return adapterv2.New(cfg)
	case V1alpha5:
		return adapterv5.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported vm-operator-api-version %q: supported versions are %s",
			version, strings.Join(supportedVersions, ", "))
	}
}
