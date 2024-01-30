package client

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

var (
	// VirtualMachineServiceGVR has virtualmachineservice resource info.
	VirtualMachineServiceGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha1",
		Resource: "virtualmachineservices",
	}
	// VirtualMachineGVR has virtualmachine resource info.
	VirtualMachineGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha1",
		Resource: "virtualmachines",
	}
)

// VmoperatorV1alpha1Client contains the dynamic client for vm operator group
type VmoperatorV1alpha1Client struct {
	dynamicClient *dynamic.DynamicClient
}

// VirtualMachines retrieves the virtualmachine client
func (c *VmoperatorV1alpha1Client) VirtualMachines(namespace string) vmoperator.VirtualMachineInterface {
	return newVirtualMachines(c, namespace)
}

// VirtualMachineServices retrieves the virtualmachineservice client
func (c *VmoperatorV1alpha1Client) VirtualMachineServices(namespace string) vmoperator.VirtualMachineServiceInterface {
	return newVirtualMachineServices(c, namespace)
}

// Client retrieves the dynamic client
func (c *VmoperatorV1alpha1Client) Client() dynamic.Interface {
	if c == nil {
		return nil
	}
	return c.dynamicClient
}

// NewForConfig creates a new client for the given config.
func NewForConfig(c *rest.Config) (*VmoperatorV1alpha1Client, error) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)

	dynamicClient, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	client := &VmoperatorV1alpha1Client{
		dynamicClient: dynamicClient,
	}
	return client, nil
}
