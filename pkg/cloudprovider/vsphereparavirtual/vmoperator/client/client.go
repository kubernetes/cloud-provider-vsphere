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

// Package client provides a dynamic client for the VM Operator v1alpha2 API.
// It is used internally by the v1alpha2 adapter and should not be imported
// directly by business logic. Business logic should use vmoperator.Interface.
package client

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var (
	VirtualMachineServiceGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha2",
		Resource: "virtualmachineservices",
	}
	VirtualMachineGVR = schema.GroupVersionResource{
		Group:    "vmoperator.vmware.com",
		Version:  "v1alpha2",
		Resource: "virtualmachines",
	}
)

// Client wraps a dynamic client for the v1alpha2 API group.
type Client struct {
	dynamicClient dynamic.Interface
}

// NewForConfig creates a new Client from the given REST config.
func NewForConfig(cfg *rest.Config) (*Client, error) {
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &Client{dynamicClient: dc}, nil
}

// NewWithDynamicClient creates a Client from an existing dynamic.Interface.
// This is intended for testing with a fake dynamic client.
func NewWithDynamicClient(dc dynamic.Interface) *Client {
	return &Client{dynamicClient: dc}
}
