/*
Copyright 2026 The Kubernetes Authors.

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

// Package types defines version-agnostic hub types for VM Operator resources.
// The CPI only needs a small subset of the VM Operator API; these types capture
// exactly that subset as a union across all supported API versions.
//
// Fields must never be removed or renamed — older adapters depend on them.
// When a new API version introduces fields the CPI needs, add them here, update
// the relevant adapter's conversion functions, and leave older adapters unchanged.
package types

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PowerState represents the power state of a VirtualMachine.
type PowerState string

// Power state values mirror the VM Operator API and are kept stable across versions.
const (
	PowerStatePoweredOn  PowerState = "PoweredOn"
	PowerStatePoweredOff PowerState = "PoweredOff"
	PowerStateSuspended  PowerState = "Suspended"
)

// VirtualMachineServiceType represents the type of a VirtualMachineService.
type VirtualMachineServiceType string

// Service type values mirror the VM Operator API.
const (
	VirtualMachineServiceTypeLoadBalancer VirtualMachineServiceType = "LoadBalancer"
	VirtualMachineServiceTypeClusterIP    VirtualMachineServiceType = "ClusterIP"
	VirtualMachineServiceTypeExternalName VirtualMachineServiceType = "ExternalName"
)

// VirtualMachineInfo holds the VM fields required by the CPI for node discovery.
type VirtualMachineInfo struct {
	Name       string
	Namespace  string
	Labels     map[string]string
	BiosUUID   string
	PowerState PowerState
	PrimaryIP4 string
	PrimaryIP6 string
}

// VirtualMachineServicePort describes a single port exposed by a VirtualMachineService.
// Protocol is a plain string ("TCP", "UDP", "SCTP") passed through from the
// Kubernetes Service without interpretation.
type VirtualMachineServicePort struct {
	Name       string
	Protocol   string
	Port       int32
	TargetPort int32
}

// VirtualMachineServiceSpec is the hub spec for a VirtualMachineService.
type VirtualMachineServiceSpec struct {
	Type                     VirtualMachineServiceType
	Ports                    []VirtualMachineServicePort
	Selector                 map[string]string
	LoadBalancerIP           string
	LoadBalancerSourceRanges []string
}

// LoadBalancerIngress represents a single ingress point for a load balancer.
type LoadBalancerIngress struct {
	IP       string
	Hostname string
}

// VirtualMachineServiceStatus is the hub status for a VirtualMachineService.
type VirtualMachineServiceStatus struct {
	LoadBalancerIngress []LoadBalancerIngress
}

// VirtualMachineServiceInfo holds the VirtualMachineService fields required by the CPI.
// ResourceVersion is populated by adapters from the underlying API object and is
// required for optimistic concurrency on Update.
type VirtualMachineServiceInfo struct {
	Name            string
	Namespace       string
	ResourceVersion string
	Labels          map[string]string
	Annotations     map[string]string
	OwnerReferences []metav1.OwnerReference
	Spec            VirtualMachineServiceSpec
	Status          VirtualMachineServiceStatus
}

// ListOptions contains options for listing resources.
type ListOptions struct {
	LabelSelector string
}
