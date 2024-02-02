package client

import (
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

// FakeClientSet contains the fake clients for groups. Each group has exactly one
// version included in a Clientset.
type FakeClientSet struct {
	FakeClient *FakeClient
}

// V1alpha1 retrieves the fake VmoperatorV1alpha1Client
func (c *FakeClientSet) V1alpha1() vmoperator.V1alpha1Interface {
	return c.FakeClient
}

// NewFakeClientSet creates a FakeClientWrapper
func NewFakeClientSet(fakeClient *dynamicfake.FakeDynamicClient) *FakeClientSet {
	fcw := &FakeClientSet{
		FakeClient: &FakeClient{
			DynamicClient: fakeClient,
		},
	}
	return fcw
}

// FakeClient contains the fake dynamic client for vm operator group
type FakeClient struct {
	DynamicClient *dynamicfake.FakeDynamicClient
}

// VirtualMachines retrieves the virtualmachine client
func (c *FakeClient) VirtualMachines(namespace string) vmoperator.VirtualMachineInterface {
	return newVirtualMachines(c, namespace)
}

// VirtualMachineServices retrieves the virtualmachineservice client
func (c *FakeClient) VirtualMachineServices(namespace string) vmoperator.VirtualMachineServiceInterface {
	return newVirtualMachineServices(c, namespace)
}

// Client retrieves the dynamic client
func (c *FakeClient) Client() dynamic.Interface {
	if c == nil {
		return nil
	}
	return c.DynamicClient
}
