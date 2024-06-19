package vmoperator

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
)

// Interface has methods to work with Vmoperator resources.
type Interface interface {
	V1alpha2() V1alpha2Interface
}

// V1alpha2Interface has methods to work with Vmoperator V1alpha2 resources.
type V1alpha2Interface interface {
	Client() dynamic.Interface
	VirtualMachines(namespace string) VirtualMachineInterface
	VirtualMachineServices(namespace string) VirtualMachineServiceInterface
}

// VirtualMachineInterface has methods to work with VirtualMachineService resources.
type VirtualMachineInterface interface {
	Create(ctx context.Context, virtualMachine *vmopv1.VirtualMachine, opts v1.CreateOptions) (*vmopv1.VirtualMachine, error)
	Update(ctx context.Context, virtualMachine *vmopv1.VirtualMachine, opts v1.UpdateOptions) (*vmopv1.VirtualMachine, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1.VirtualMachine, error)
	List(ctx context.Context, opts v1.ListOptions) (*vmopv1.VirtualMachineList, error)
}

// VirtualMachineServiceInterface has methods to work with VirtualMachineService resources.
type VirtualMachineServiceInterface interface {
	Create(ctx context.Context, virtualMachineService *vmopv1.VirtualMachineService, opts v1.CreateOptions) (*vmopv1.VirtualMachineService, error)
	Update(ctx context.Context, virtualMachineService *vmopv1.VirtualMachineService, opts v1.UpdateOptions) (*vmopv1.VirtualMachineService, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*vmopv1.VirtualMachineService, error)
	List(ctx context.Context, opts v1.ListOptions) (*vmopv1.VirtualMachineServiceList, error)
}
