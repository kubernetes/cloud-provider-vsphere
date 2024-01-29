package client

import (
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

type FakeClient struct {
	DynamicClient *dynamicfake.FakeDynamicClient
}

// NewFakeClientWrapper creates a FakeClientWrapper
func NewFakeClient(fakeClient *dynamicfake.FakeDynamicClient) *FakeClient {
	fcw := FakeClient{}
	fcw.DynamicClient = fakeClient
	return &fcw
}

func (c *FakeClient) VirtualMachines(namespace string) vmoperator.VirtualMachineInterface {
	return newVirtualMachines(c, namespace)
}

func (c *FakeClient) VirtualMachineServices(namespace string) vmoperator.VirtualMachineServiceInterface {
	return newVirtualMachineServices(c, namespace)
}

func (c *FakeClient) Client() dynamic.Interface {
	if c == nil {
		return nil
	}
	return c.DynamicClient
}
