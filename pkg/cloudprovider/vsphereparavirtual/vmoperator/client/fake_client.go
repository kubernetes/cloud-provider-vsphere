package client

import (
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

// FakeClient contains the fake dynamic client for vm operator group
type FakeClient struct {
	DynamicClient *dynamicfake.FakeDynamicClient
}

// NewFakeClient creates a FakeClientWrapper
func NewFakeClient(fakeClient *dynamicfake.FakeDynamicClient) *FakeClient {
	fcw := FakeClient{}
	fcw.DynamicClient = fakeClient
	return &fcw
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
