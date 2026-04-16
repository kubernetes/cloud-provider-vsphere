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

// Package v1alpha6 provides a dynamic client for the VM Operator v1alpha6 API.
package v1alpha6

import (
	"context"

	vmopv6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// GVRs for the VM Operator v1alpha6 resources used by the CPI.
var (
	VirtualMachineGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha6",
		Resource: "virtualmachines",
	}
	VirtualMachineServiceGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha6",
		Resource: "virtualmachineservices",
	}
)

// Client wraps a dynamic client for the v1alpha6 API group.
type Client struct {
	dynamicClient dynamic.Interface
}

// NewForConfig creates a new v1alpha6 Client from the given REST config.
func NewForConfig(cfg *rest.Config) (*Client, error) {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{dynamicClient: dc}, nil
}

// NewWithDynamicClient creates a Client from an existing dynamic.Interface.
// Intended for testing with a fake dynamic client.
func NewWithDynamicClient(dc dynamic.Interface) *Client {
	return &Client{dynamicClient: dc}
}

// GetVirtualMachine fetches a VirtualMachine by namespace and name.
func (c *Client) GetVirtualMachine(ctx context.Context, namespace, name string) (*vmopv6.VirtualMachine, error) {
	obj, err := c.dynamicClient.Resource(VirtualMachineGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	vm := &vmopv6.VirtualMachine{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), vm); err != nil {
		return nil, err
	}
	return vm, nil
}

// ListVirtualMachines lists VirtualMachines in the given namespace.
// ResourceVersion="0" defaults to reading from the API server's watch cache.
func (c *Client) ListVirtualMachines(ctx context.Context, namespace string, opts metav1.ListOptions) (*vmopv6.VirtualMachineList, error) {
	if opts.ResourceVersion == "" {
		opts.ResourceVersion = "0"
	}
	obj, err := c.dynamicClient.Resource(VirtualMachineGVR).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	list := &vmopv6.VirtualMachineList{
		Items: make([]vmopv6.VirtualMachine, 0, len(obj.Items)),
	}
	for i := range obj.Items {
		vm := &vmopv6.VirtualMachine{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Items[i].UnstructuredContent(), vm); err != nil {
			return nil, err
		}
		list.Items = append(list.Items, *vm)
	}
	return list, nil
}

// GetVirtualMachineService fetches a VirtualMachineService by namespace and name.
func (c *Client) GetVirtualMachineService(ctx context.Context, namespace, name string) (*vmopv6.VirtualMachineService, error) {
	obj, err := c.dynamicClient.Resource(VirtualMachineServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	vmService := &vmopv6.VirtualMachineService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), vmService); err != nil {
		return nil, err
	}
	return vmService, nil
}

// Dynamic returns the underlying dynamic client for unstructured access (used by the v1alpha6 adapter for dual-stack fields).
func (c *Client) Dynamic() dynamic.Interface {
	return c.dynamicClient
}

// ListVirtualMachineServices lists VirtualMachineServices in the given namespace.
// ResourceVersion="0" defaults to reading from the API server's watch cache.
func (c *Client) ListVirtualMachineServices(ctx context.Context, namespace string, opts metav1.ListOptions) (*vmopv6.VirtualMachineServiceList, error) {
	if opts.ResourceVersion == "" {
		opts.ResourceVersion = "0"
	}
	obj, err := c.dynamicClient.Resource(VirtualMachineServiceGVR).Namespace(namespace).List(ctx, opts)
	if err != nil {
		return nil, err
	}
	list := &vmopv6.VirtualMachineServiceList{
		Items: make([]vmopv6.VirtualMachineService, 0, len(obj.Items)),
	}
	for i := range obj.Items {
		vmService := &vmopv6.VirtualMachineService{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Items[i].UnstructuredContent(), vmService); err != nil {
			return nil, err
		}
		list.Items = append(list.Items, *vmService)
	}
	return list, nil
}

// CreateVirtualMachineService creates a VirtualMachineService.
// apiVersion and kind are set explicitly because ToUnstructured does not populate
// them when TypeMeta is unset, which would cause the API server to reject the request.
func (c *Client) CreateVirtualMachineService(ctx context.Context, vmService *vmopv6.VirtualMachineService) (*vmopv6.VirtualMachineService, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vmService)
	if err != nil {
		return nil, err
	}
	unstructuredObj["apiVersion"] = VirtualMachineServiceGVR.Group + "/" + VirtualMachineServiceGVR.Version
	unstructuredObj["kind"] = "VirtualMachineService"
	obj, err := c.dynamicClient.Resource(VirtualMachineServiceGVR).Namespace(vmService.Namespace).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	created := &vmopv6.VirtualMachineService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), created); err != nil {
		return nil, err
	}
	return created, nil
}

// UpdateVirtualMachineService updates a VirtualMachineService.
// apiVersion and kind are set explicitly for the same reason as in Create.
func (c *Client) UpdateVirtualMachineService(ctx context.Context, vmService *vmopv6.VirtualMachineService) (*vmopv6.VirtualMachineService, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vmService)
	if err != nil {
		return nil, err
	}
	unstructuredObj["apiVersion"] = VirtualMachineServiceGVR.Group + "/" + VirtualMachineServiceGVR.Version
	unstructuredObj["kind"] = "VirtualMachineService"
	obj, err := c.dynamicClient.Resource(VirtualMachineServiceGVR).Namespace(vmService.Namespace).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	updated := &vmopv6.VirtualMachineService{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), updated); err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteVirtualMachineService deletes a VirtualMachineService by namespace and name.
func (c *Client) DeleteVirtualMachineService(ctx context.Context, namespace, name string) error {
	return c.dynamicClient.Resource(VirtualMachineServiceGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}
