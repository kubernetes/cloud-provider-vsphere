package vmoperator

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
)

// VmoperatorV1alpha1Interface has methods to work with Vmoperator V1alpha1 resources.
type VmoperatorV1alpha1Interface interface {
	Client() dynamic.Interface
	VirtualMachines(namespace string) VirtualMachineInterface
	VirtualMachineServices(namespace string) VirtualMachineServiceInterface
}

// VirtualMachineInterface has methods to work with VirtualMachineService resources.
type VirtualMachineInterface interface {
	Create(ctx context.Context, virtualMachine *vmopv1alpha1.VirtualMachine, opts v1.CreateOptions) (*vmopv1alpha1.VirtualMachine, error)
	Update(ctx context.Context, virtualMachine *vmopv1alpha1.VirtualMachine, opts v1.UpdateOptions) (*vmopv1alpha1.VirtualMachine, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1alpha1.VirtualMachine, error)
	List(ctx context.Context, opts v1.ListOptions) (*vmopv1alpha1.VirtualMachineList, error)
}

// VirtualMachineServiceInterface has methods to work with VirtualMachineService resources.
type VirtualMachineServiceInterface interface {
	Create(ctx context.Context, virtualMachineService *vmopv1alpha1.VirtualMachineService, opts v1.CreateOptions) (*vmopv1alpha1.VirtualMachineService, error)
	Update(ctx context.Context, virtualMachineService *vmopv1alpha1.VirtualMachineService, opts v1.UpdateOptions) (*vmopv1alpha1.VirtualMachineService, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1alpha1.VirtualMachineService, error)
	List(ctx context.Context, opts v1.ListOptions) (*vmopv1alpha1.VirtualMachineServiceList, error)
}
