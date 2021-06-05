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

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	v1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	client "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	fake "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	nsxv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/typed/nsxnetworking/v1alpha1"
)

// FakeRouteSetClientWrapper allows functions to be replaced for fault injection
type FakeRouteSetClientWrapper struct {
	fakeClient client.Interface
	// Set these functions if you want to override the default fakeClient behavior
	GetFunc    func(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.RouteSet, err error)
	CreateFunc func(ctx context.Context, routeSet *v1alpha1.RouteSet, opts v1.CreateOptions) (result *v1alpha1.RouteSet, err error)
	DeleteFunc func(ctx context.Context, name string, opts v1.DeleteOptions) error
	ListFunc   func(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.RouteSetList, err error)
}

// NewFakeRouteSetClientWrapper creates a FakeClientWrapper
func NewFakeRouteSetClientWrapper(fakeClient *fake.Clientset) *FakeRouteSetClientWrapper {
	fcw := FakeRouteSetClientWrapper{}
	fcw.fakeClient = fakeClient
	return &fcw
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (w *FakeRouteSetClientWrapper) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.RouteSet, err error) {
	if w.GetFunc != nil {
		return w.GetFunc(ctx, name, options)
	}
	return w.fakeClient.NsxV1alpha1().RouteSets("test-ns").Get(ctx, name, options)
}

// List retrieves list of objects for a given namespace and list options.
func (w *FakeRouteSetClientWrapper) List(ctx context.Context, options v1.ListOptions) (result *v1alpha1.RouteSetList, err error) {
	if w.ListFunc != nil {
		return w.ListFunc(ctx, options)
	}
	return w.fakeClient.NsxV1alpha1().RouteSets("test-ns").List(ctx, options)
}

// Create saves the object obj in the Kubernetes cluster.
func (w *FakeRouteSetClientWrapper) Create(ctx context.Context, route *v1alpha1.RouteSet, options v1.CreateOptions) (result *v1alpha1.RouteSet, err error) {
	if w.CreateFunc != nil {
		return w.CreateFunc(ctx, route, options)
	}
	return w.fakeClient.NsxV1alpha1().RouteSets("test-ns").Create(ctx, route, options)
}

// Delete deletes the given obj from Kubernetes cluster.
func (w *FakeRouteSetClientWrapper) Delete(ctx context.Context, name string, options v1.DeleteOptions) error {
	if w.DeleteFunc != nil {
		return w.DeleteFunc(ctx, name, options)
	}
	return w.fakeClient.NsxV1alpha1().RouteSets("test-ns").Delete(ctx, name, options)
}

// Discovery retrieves the DiscoveryClient
func (w *FakeRouteSetClientWrapper) Discovery() discovery.DiscoveryInterface {
	return w.fakeClient.Discovery()
}

// NsxV1alpha1 retrieves the NsxV1alpha1Client
func (w *FakeRouteSetClientWrapper) NsxV1alpha1() nsxv1alpha1.NsxV1alpha1Interface {
	return w.fakeClient.NsxV1alpha1()
}
