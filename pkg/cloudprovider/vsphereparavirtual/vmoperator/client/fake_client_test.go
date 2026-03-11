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

// White-box test helpers for the client package.
// External test packages should use the fake sub-package instead.
package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

// NewFakeClient wraps fakeClient in a Client for use in this package's own tests.
func NewFakeClient(fakeClient *dynamicfake.FakeDynamicClient) *Client {
	return NewWithDynamicClient(fakeClient)
}

// listOptsSpy is a minimal dynamic.Interface that records the ResourceVersion
// from the most recent List call. Methods other than Resource/Namespace/List
// panic because the tests that use this spy only exercise List paths.
type listOptsSpy struct {
	capturedRV string
}

func (s *listOptsSpy) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &listOptsSpyResource{spy: s}
}

type listOptsSpyResource struct {
	spy       *listOptsSpy
	namespace string
}

func (r *listOptsSpyResource) Namespace(ns string) dynamic.ResourceInterface {
	return &listOptsSpyResource{spy: r.spy, namespace: ns}
}

func (r *listOptsSpyResource) List(ctx context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.spy.capturedRV = opts.ResourceVersion
	return &unstructured.UnstructuredList{}, nil
}

func (r *listOptsSpyResource) Create(_ context.Context, _ *unstructured.Unstructured, _ metav1.CreateOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) Update(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) UpdateStatus(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) Delete(_ context.Context, _ string, _ metav1.DeleteOptions, _ ...string) error {
	panic("not implemented")
}
func (r *listOptsSpyResource) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	panic("not implemented")
}
func (r *listOptsSpyResource) Get(_ context.Context, _ string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) Apply(_ context.Context, _ string, _ *unstructured.Unstructured, _ metav1.ApplyOptions, _ ...string) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
func (r *listOptsSpyResource) ApplyStatus(_ context.Context, _ string, _ *unstructured.Unstructured, _ metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	panic("not implemented")
}
