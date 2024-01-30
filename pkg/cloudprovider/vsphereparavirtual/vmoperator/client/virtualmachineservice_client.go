package client

import (
	"context"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

// virtualMachineServices implements VirtualMachineServiceInterface
type virtualMachineServices struct {
	client dynamic.Interface
	ns     string
}

// newVirtualMachineServices returns a VirtualMachineServices
func newVirtualMachineServices(c vmoperator.V1alpha1Interface, namespace string) *virtualMachineServices {
	return &virtualMachineServices{
		client: c.Client(),
		ns:     namespace,
	}
}

func (v *virtualMachineServices) Create(ctx context.Context, virtualMachineService *vmopv1alpha1.VirtualMachineService, opts v1.CreateOptions) (*vmopv1alpha1.VirtualMachineService, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(virtualMachineService)
	if err != nil {
		return nil, err
	}

	obj, err := v.client.Resource(VirtualMachineServiceGVR).Namespace(v.ns).Create(ctx, &unstructured.Unstructured{Object: unstructuredObj}, opts)
	if err != nil {
		return nil, err
	}

	createdVirtualMachineService := &vmopv1alpha1.VirtualMachineService{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), createdVirtualMachineService); err != nil {
		return nil, err
	}
	return createdVirtualMachineService, nil
}

func (v *virtualMachineServices) Update(ctx context.Context, virtualMachineService *vmopv1alpha1.VirtualMachineService, opts v1.UpdateOptions) (*vmopv1alpha1.VirtualMachineService, error) {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(virtualMachineService)
	if err != nil {
		return nil, err
	}

	obj, err := v.client.Resource(VirtualMachineServiceGVR).Namespace(v.ns).Update(ctx, &unstructured.Unstructured{Object: unstructuredObj}, opts)
	if err != nil {
		return nil, err
	}

	updatedVirtualMachineService := &vmopv1alpha1.VirtualMachineService{}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), updatedVirtualMachineService); err != nil {
		return nil, err
	}
	return updatedVirtualMachineService, nil
}

func (v *virtualMachineServices) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return v.client.Resource(VirtualMachineServiceGVR).Namespace(v.ns).Delete(ctx, name, opts)
}

func (v *virtualMachineServices) Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1alpha1.VirtualMachineService, error) {
	virtualMachineService := &vmopv1alpha1.VirtualMachineService{}
	if obj, err := v.client.Resource(VirtualMachineServiceGVR).Namespace(v.ns).Get(ctx, name, opts); err != nil {
		return nil, err
	} else if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), virtualMachineService); err != nil {
		return nil, err
	}
	return virtualMachineService, nil
}

func (v *virtualMachineServices) List(ctx context.Context, opts v1.ListOptions) (*vmopv1alpha1.VirtualMachineServiceList, error) {
	virtualMachineServiceList := &vmopv1alpha1.VirtualMachineServiceList{}
	if obj, err := v.client.Resource(VirtualMachineServiceGVR).Namespace(v.ns).List(ctx, opts); err != nil {
		return nil, err
	} else if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), virtualMachineServiceList); err != nil {
		return nil, err
	}
	return virtualMachineServiceList, nil
}
