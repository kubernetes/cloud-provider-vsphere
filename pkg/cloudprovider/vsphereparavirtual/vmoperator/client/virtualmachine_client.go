/*
Copyright The Kubernetes Authors.

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
package client

import (
	"context"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

// virtualMachines implements VirtualMachineInterface
type virtualMachines struct {
	client dynamic.Interface
	ns     string
}

func newVirtualMachines(c vmoperator.V1alpha2Interface, namespace string) *virtualMachines {
	return &virtualMachines{
		client: c.Client(),
		ns:     namespace,
	}
}

func (v *virtualMachines) Create(ctx context.Context, virtualMachine *vmopv1.VirtualMachine, opts v1.CreateOptions) (*vmopv1.VirtualMachine, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(virtualMachine)
	if err != nil {
		return nil, err
	}

	obj, err := v.client.Resource(VirtualMachineGVR).Namespace(v.ns).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, opts)
	if err != nil {
		return nil, err
	}

	createdVirtualMachine := &vmopv1.VirtualMachine{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), createdVirtualMachine); err != nil {
		return nil, err
	}
	return createdVirtualMachine, nil
}

func (v *virtualMachines) Update(ctx context.Context, virtualMachine *vmopv1.VirtualMachine, opts v1.UpdateOptions) (*vmopv1.VirtualMachine, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(virtualMachine)
	if err != nil {
		return nil, err
	}

	obj, err := v.client.Resource(VirtualMachineGVR).Namespace(v.ns).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, opts)
	if err != nil {
		return nil, err
	}

	updatedVirtualMachine := &vmopv1.VirtualMachine{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), updatedVirtualMachine); err != nil {
		return nil, err
	}
	return updatedVirtualMachine, nil
}

func (v *virtualMachines) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return v.client.Resource(VirtualMachineGVR).Namespace(v.ns).Delete(ctx, name, opts)
}

func (v *virtualMachines) Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1.VirtualMachine, error) {
	virtualMachine := &vmopv1.VirtualMachine{}
	if obj, err := v.client.Resource(VirtualMachineGVR).Namespace(v.ns).Get(ctx, name, opts); err != nil {
		return nil, err
	} else if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), virtualMachine); err != nil {
		return nil, err
	}
	return virtualMachine, nil
}

func (v *virtualMachines) List(ctx context.Context, opts v1.ListOptions) (*vmopv1.VirtualMachineList, error) {
	virtualMachineList := &vmopv1.VirtualMachineList{}
	if obj, err := v.client.Resource(VirtualMachineGVR).Namespace(v.ns).List(ctx, opts); err != nil {
		return nil, err
	} else if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), virtualMachineList); err != nil {
		return nil, err
	}
	return virtualMachineList, nil
}
