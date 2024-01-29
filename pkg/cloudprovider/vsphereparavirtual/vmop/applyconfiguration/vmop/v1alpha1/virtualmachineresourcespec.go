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

// Code generated by applyconfiguration-gen. DO NOT EDIT.

package v1alpha1

import (
	resource "k8s.io/apimachinery/pkg/api/resource"
)

// VirtualMachineResourceSpecApplyConfiguration represents an declarative configuration of the VirtualMachineResourceSpec type for use
// with apply.
type VirtualMachineResourceSpecApplyConfiguration struct {
	Cpu    *resource.Quantity `json:"cpu,omitempty"`
	Memory *resource.Quantity `json:"memory,omitempty"`
}

// VirtualMachineResourceSpecApplyConfiguration constructs an declarative configuration of the VirtualMachineResourceSpec type for use with
// apply.
func VirtualMachineResourceSpec() *VirtualMachineResourceSpecApplyConfiguration {
	return &VirtualMachineResourceSpecApplyConfiguration{}
}

// WithCpu sets the Cpu field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Cpu field is set to the value of the last call.
func (b *VirtualMachineResourceSpecApplyConfiguration) WithCpu(value resource.Quantity) *VirtualMachineResourceSpecApplyConfiguration {
	b.Cpu = &value
	return b
}

// WithMemory sets the Memory field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Memory field is set to the value of the last call.
func (b *VirtualMachineResourceSpecApplyConfiguration) WithMemory(value resource.Quantity) *VirtualMachineResourceSpecApplyConfiguration {
	b.Memory = &value
	return b
}