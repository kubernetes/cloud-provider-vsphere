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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	json "encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
	v1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmopv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/applyconfiguration/vmop/v1alpha1"
	scheme "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/clientset/versioned/scheme"
)

// VirtualMachineImagesGetter has a method to return a VirtualMachineImageInterface.
// A group's client should implement this interface.
type VirtualMachineImagesGetter interface {
	VirtualMachineImages() VirtualMachineImageInterface
}

// VirtualMachineImageInterface has methods to work with VirtualMachineImage resources.
type VirtualMachineImageInterface interface {
	Create(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.CreateOptions) (*v1alpha1.VirtualMachineImage, error)
	Update(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.UpdateOptions) (*v1alpha1.VirtualMachineImage, error)
	UpdateStatus(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.UpdateOptions) (*v1alpha1.VirtualMachineImage, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.VirtualMachineImage, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.VirtualMachineImageList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.VirtualMachineImage, err error)
	Apply(ctx context.Context, virtualMachineImage *vmopv1alpha1.VirtualMachineImageApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.VirtualMachineImage, err error)
	ApplyStatus(ctx context.Context, virtualMachineImage *vmopv1alpha1.VirtualMachineImageApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.VirtualMachineImage, err error)
	VirtualMachineImageExpansion
}

// virtualMachineImages implements VirtualMachineImageInterface
type virtualMachineImages struct {
	client rest.Interface
}

// newVirtualMachineImages returns a VirtualMachineImages
func newVirtualMachineImages(c *VmoperatorV1alpha1Client) *virtualMachineImages {
	return &virtualMachineImages{
		client: c.RESTClient(),
	}
}

// Get takes name of the virtualMachineImage, and returns the corresponding virtualMachineImage object, and an error if there is any.
func (c *virtualMachineImages) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Get().
		Resource("virtualmachineimages").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of VirtualMachineImages that match those selectors.
func (c *virtualMachineImages) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.VirtualMachineImageList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.VirtualMachineImageList{}
	err = c.client.Get().
		Resource("virtualmachineimages").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested virtualMachineImages.
func (c *virtualMachineImages) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("virtualmachineimages").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a virtualMachineImage and creates it.  Returns the server's representation of the virtualMachineImage, and an error, if there is any.
func (c *virtualMachineImages) Create(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.CreateOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Post().
		Resource("virtualmachineimages").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(virtualMachineImage).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a virtualMachineImage and updates it. Returns the server's representation of the virtualMachineImage, and an error, if there is any.
func (c *virtualMachineImages) Update(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.UpdateOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Put().
		Resource("virtualmachineimages").
		Name(virtualMachineImage.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(virtualMachineImage).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *virtualMachineImages) UpdateStatus(ctx context.Context, virtualMachineImage *v1alpha1.VirtualMachineImage, opts v1.UpdateOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Put().
		Resource("virtualmachineimages").
		Name(virtualMachineImage.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(virtualMachineImage).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the virtualMachineImage and deletes it. Returns an error if one occurs.
func (c *virtualMachineImages) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("virtualmachineimages").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *virtualMachineImages) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("virtualmachineimages").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched virtualMachineImage.
func (c *virtualMachineImages) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.VirtualMachineImage, err error) {
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Patch(pt).
		Resource("virtualmachineimages").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// Apply takes the given apply declarative configuration, applies it and returns the applied virtualMachineImage.
func (c *virtualMachineImages) Apply(ctx context.Context, virtualMachineImage *vmopv1alpha1.VirtualMachineImageApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	if virtualMachineImage == nil {
		return nil, fmt.Errorf("virtualMachineImage provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(virtualMachineImage)
	if err != nil {
		return nil, err
	}
	name := virtualMachineImage.Name
	if name == nil {
		return nil, fmt.Errorf("virtualMachineImage.Name must be provided to Apply")
	}
	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Patch(types.ApplyPatchType).
		Resource("virtualmachineimages").
		Name(*name).
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}

// ApplyStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating ApplyStatus().
func (c *virtualMachineImages) ApplyStatus(ctx context.Context, virtualMachineImage *vmopv1alpha1.VirtualMachineImageApplyConfiguration, opts v1.ApplyOptions) (result *v1alpha1.VirtualMachineImage, err error) {
	if virtualMachineImage == nil {
		return nil, fmt.Errorf("virtualMachineImage provided to Apply must not be nil")
	}
	patchOpts := opts.ToPatchOptions()
	data, err := json.Marshal(virtualMachineImage)
	if err != nil {
		return nil, err
	}

	name := virtualMachineImage.Name
	if name == nil {
		return nil, fmt.Errorf("virtualMachineImage.Name must be provided to Apply")
	}

	result = &v1alpha1.VirtualMachineImage{}
	err = c.client.Patch(types.ApplyPatchType).
		Resource("virtualmachineimages").
		Name(*name).
		SubResource("status").
		VersionedParams(&patchOpts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}