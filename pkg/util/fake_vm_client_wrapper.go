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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmopclient "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/clientset/versioned"
	fake "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/clientset/versioned/fake"
	vmoperatorv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/clientset/versioned/typed/vmop/v1alpha1"
)

// FakeClientWrapper allows functions to be replaced for fault injection
type FakeVMClientWrapper struct {
	fakeClient vmopclient.Interface
	// Set these functions if you want to override the default fakeClient behavior
	GetFunc    func(ctx context.Context, namespace, name string, opts metav1.GetOptions) (result *vmopv1alpha1.VirtualMachine, err error)
	ListFunc   func(ctx context.Context, namespace string, opts metav1.ListOptions) (result *vmopv1alpha1.VirtualMachineList, err error)
	CreateFunc func(ctx context.Context, vm *vmopv1alpha1.VirtualMachine, opts metav1.CreateOptions) (result *vmopv1alpha1.VirtualMachine, err error)
	UpdateFunc func(ctx context.Context, vm *vmopv1alpha1.VirtualMachine, opts metav1.UpdateOptions) (result *vmopv1alpha1.VirtualMachine, err error)
	DeleteFunc func(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error
}

// NewFakeVMClientWrapper creates a FakeClientWrapper
func NewFakeVMClientWrapper(fakeClient *fake.Clientset) *FakeVMClientWrapper {
	fcw := FakeVMClientWrapper{}
	fcw.fakeClient = fakeClient
	return &fcw
}

// Get retrieves an obj for the given object key from the Kubernetes Cluster.
func (w *FakeVMClientWrapper) Get(ctx context.Context, namespace, name string, opts metav1.GetOptions) (result *vmopv1alpha1.VirtualMachine, err error) {
	if w.GetFunc != nil {
		return w.GetFunc(ctx, namespace, name, opts)
	}
	return w.fakeClient.VmoperatorV1alpha1().VirtualMachines(namespace).Get(ctx, name, opts)
}

// List retrieves list of objects for a given namespace and list options.
func (w *FakeVMClientWrapper) List(ctx context.Context, namespace string, opts metav1.ListOptions) (result *vmopv1alpha1.VirtualMachineList, err error) {
	if w.ListFunc != nil {
		return w.ListFunc(ctx, namespace, opts)
	}
	return w.fakeClient.VmoperatorV1alpha1().VirtualMachines(namespace).List(ctx, opts)
}

// Create saves the object obj in the Kubernetes cluster.
func (w *FakeVMClientWrapper) Create(ctx context.Context, vm *vmopv1alpha1.VirtualMachine, opts metav1.CreateOptions) (result *vmopv1alpha1.VirtualMachine, err error) {
	if w.CreateFunc != nil {
		return w.CreateFunc(ctx, vm, opts)
	}
	return w.fakeClient.VmoperatorV1alpha1().VirtualMachines(vm.Namespace).Create(ctx, vm, opts)
}

// Update updates the given obj in the Kubernetes cluster.
func (w *FakeVMClientWrapper) Update(ctx context.Context, vm *vmopv1alpha1.VirtualMachine, opts metav1.UpdateOptions) (result *vmopv1alpha1.VirtualMachine, err error) {
	if w.UpdateFunc != nil {
		return w.UpdateFunc(ctx, vm, opts)
	}
	return w.fakeClient.VmoperatorV1alpha1().VirtualMachines(vm.Namespace).Update(ctx, vm, opts)
}

// Delete deletes the given obj from Kubernetes cluster.
func (w *FakeVMClientWrapper) Delete(ctx context.Context, namespace, name string, opts metav1.DeleteOptions) error {
	if w.DeleteFunc != nil {
		return w.DeleteFunc(ctx, namespace, name, opts)
	}
	return w.fakeClient.VmoperatorV1alpha1().VirtualMachines(namespace).Delete(ctx, name, opts)
}

// Discovery retrieves the DiscoveryClient
func (w *FakeVMClientWrapper) Discovery() discovery.DiscoveryInterface {
	return w.fakeClient.Discovery()
}

// VmoperatorV1alpha1 retrieves the VmoperatorV1alpha1Client
func (w *FakeVMClientWrapper) VmoperatorV1alpha1() vmoperatorv1alpha1.VmoperatorV1alpha1Interface {
	return w.fakeClient.VmoperatorV1alpha1()
}
