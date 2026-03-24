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

// Package fake provides test helpers for the v1alpha5 adapter.
// It is intended for use in tests only and must not be imported by production code.
package fake

import (
	vmopv5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	adapterv5 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha5"
	clientv5 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha5"
)

// NewAdapter creates a v1alpha5 Adapter backed by a new fake dynamic client
// with the v1alpha5 scheme registered. It returns the adapter and the underlying
// fake dynamic client so tests can prepend reactors or seed objects.
func NewAdapter() (*adapterv5.Adapter, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv5.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	return adapterv5.NewWithFakeClient(clientv5.NewWithDynamicClient(fc)), fc
}
