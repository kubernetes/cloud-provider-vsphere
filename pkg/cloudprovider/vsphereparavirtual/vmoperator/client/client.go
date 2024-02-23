/*
Copyright 2024 The Kubernetes Authors.

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

// Clientset contains the clients for groups. Each group has exactly one
// version included in a Clientset.
type Clientset struct {
	vmopv1alpha1 *VmoperatorV1alpha1Client
}

// V1alpha1 retrieves the VmoperatorV1alpha1Client
func (c *Clientset) V1alpha1() vmoperator.V1alpha1Interface {
	return c.vmopv1alpha1
}

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
func NewForConfig(c *rest.Config) (*Clientset, error) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)

	dynamicClient, err := dynamic.NewForConfig(c)
	if err != nil {
		return nil, err
	}

	clientSet := &Clientset{
		vmopv1alpha1: &VmoperatorV1alpha1Client{
			dynamicClient: dynamicClient,
		},
	}
	return clientSet, nil
}
