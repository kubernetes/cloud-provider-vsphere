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

// Package fake provides test helpers for the v1alpha6 adapter.
// It is intended for use in tests only and must not be imported by production code.
package fake

import (
	vmopv6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	adapterv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha6"
	clientv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha6"
)

// NewAdapter creates a v1alpha6 Adapter backed by a new fake dynamic client
// with the v1alpha6 scheme registered. It returns the adapter and the underlying
// fake dynamic client so tests can prepend reactors or seed objects.
func NewAdapter() (*adapterv6.Adapter, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv6.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	return adapterv6.NewWithFakeClient(clientv6.NewWithDynamicClient(fc)), fc
}
