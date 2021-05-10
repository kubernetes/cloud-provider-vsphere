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

package util

import (
	"context"

	runtime "k8s.io/apimachinery/pkg/runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeClientWrapper allows functions to be replaced for fault injection
type FakeClientWrapper struct {
	fakeClient client.Client
	// Set these functions if you want to override the default fakeClient behavior
	GetFunc    func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
	CreateFunc func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error
	UpdateFunc func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error
	DeleteFunc func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error
	ListFunc   func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error
}

// NewFakeClientWrapper creates a FakeClientWrapper
func NewFakeClientWrapper(fakeClient client.Client) *FakeClientWrapper {
	fcw := FakeClientWrapper{}
	fcw.fakeClient = fakeClient
	return &fcw
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (w *FakeClientWrapper) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	if w.GetFunc != nil {
		return w.GetFunc(ctx, key, obj)
	}
	return w.fakeClient.Get(ctx, key, obj)
}

// List retrieves list of objects for a given namespace and list options.
func (w *FakeClientWrapper) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	if w.ListFunc != nil {
		return w.ListFunc(ctx, list, opts...)
	}
	return w.fakeClient.List(ctx, list, opts...)
}

// Create saves the object obj in the Kubernetes cluster.
func (w *FakeClientWrapper) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	if w.CreateFunc != nil {
		return w.CreateFunc(ctx, obj, opts...)
	}
	return w.fakeClient.Create(ctx, obj, opts...)
}

// Delete deletes the given obj from Kubernetes cluster.
func (w *FakeClientWrapper) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	if w.DeleteFunc != nil {
		return w.DeleteFunc(ctx, obj, opts...)
	}
	return w.fakeClient.Delete(ctx, obj, opts...)
}

// Update updates the given obj in the Kubernetes cluster.
func (w *FakeClientWrapper) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	if w.UpdateFunc != nil {
		return w.UpdateFunc(ctx, obj, opts...)
	}
	return w.fakeClient.Update(ctx, obj, opts...)
}

// Patch patches the given obj in the Kubernetes cluster.
func (w *FakeClientWrapper) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return w.fakeClient.Patch(ctx, obj, patch, opts...)
}

// DeleteAllOf deletes all objects of the given type matching the given options.
func (w *FakeClientWrapper) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return w.fakeClient.DeleteAllOf(ctx, obj, opts...)
}

// Status returns a StatusWriter which knows how to update status subresource of a Kubernetes object.
func (w *FakeClientWrapper) Status() client.StatusWriter {
	return w.fakeClient.Status()
}
