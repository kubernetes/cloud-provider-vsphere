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

// Package fake provides a test helper for constructing a v1alpha2 Client backed
// by a fake dynamic client. Import this package from external test packages;
// tests inside the client package itself use the package-private NewFakeClient.
package fake

import (
	dynamicfake "k8s.io/client-go/dynamic/fake"

	vmopclient "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/client"
)

// NewClient wraps fakeClient in a v1alpha2 Client for use in tests.
func NewClient(fakeClient *dynamicfake.FakeDynamicClient) *vmopclient.Client {
	return vmopclient.NewWithDynamicClient(fakeClient)
}
