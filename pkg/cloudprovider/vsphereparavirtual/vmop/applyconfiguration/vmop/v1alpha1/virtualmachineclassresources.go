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

// VirtualMachineClassResourcesApplyConfiguration represents an declarative configuration of the VirtualMachineClassResources type for use
// with apply.
type VirtualMachineClassResourcesApplyConfiguration struct {
	Requests *VirtualMachineResourceSpecApplyConfiguration `json:"requests,omitempty"`
	Limits   *VirtualMachineResourceSpecApplyConfiguration `json:"limits,omitempty"`
}

// VirtualMachineClassResourcesApplyConfiguration constructs an declarative configuration of the VirtualMachineClassResources type for use with
// apply.
func VirtualMachineClassResources() *VirtualMachineClassResourcesApplyConfiguration {
	return &VirtualMachineClassResourcesApplyConfiguration{}
}

// WithRequests sets the Requests field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Requests field is set to the value of the last call.
func (b *VirtualMachineClassResourcesApplyConfiguration) WithRequests(value *VirtualMachineResourceSpecApplyConfiguration) *VirtualMachineClassResourcesApplyConfiguration {
	b.Requests = value
	return b
}

// WithLimits sets the Limits field in the declarative configuration to the given value
// and returns the receiver, so that objects can be built by chaining "With" function invocations.
// If called multiple times, the Limits field is set to the value of the last call.
func (b *VirtualMachineClassResourcesApplyConfiguration) WithLimits(value *VirtualMachineResourceSpecApplyConfiguration) *VirtualMachineClassResourcesApplyConfiguration {
	b.Limits = value
	return b
}