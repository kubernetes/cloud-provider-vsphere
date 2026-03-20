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

// Package vmoperator defines the version-agnostic interfaces used by CPI business
// logic to interact with VM Operator resources. The concrete implementation is
// chosen at startup by the factory package based on the --vm-operator-api-version flag.
package vmoperator

import (
	"context"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

// Interface is the top-level entry point for VM Operator operations.
// All CPI business logic must depend only on this interface and must not
// import versioned VM Operator API packages directly.
type Interface interface {
	VirtualMachines() VirtualMachineInterface
	VirtualMachineServices() VirtualMachineServiceInterface
}

// VirtualMachineInterface provides read access to VirtualMachine resources.
// The CPI never writes VirtualMachine objects; it only reads them for node discovery.
type VirtualMachineInterface interface {
	Get(ctx context.Context, namespace, name string) (*types.VirtualMachineInfo, error)
	List(ctx context.Context, namespace string, opts types.ListOptions) ([]*types.VirtualMachineInfo, error)
	// GetByBiosUUID returns the VM whose BiosUUID matches, or nil if none is found.
	// BiosUUID is a status field and cannot be filtered server-side, so implementations
	// must list all VMs in the namespace and scan in memory.
	// Returns (nil, nil) immediately when biosUUID is empty to avoid false matches
	// against VMs that have not yet been assigned a UUID by the hypervisor.
	GetByBiosUUID(ctx context.Context, namespace, biosUUID string) (*types.VirtualMachineInfo, error)
}

// VirtualMachineServiceInterface provides CRUD access to VirtualMachineService resources.
type VirtualMachineServiceInterface interface {
	Get(ctx context.Context, namespace, name string) (*types.VirtualMachineServiceInfo, error)
	List(ctx context.Context, namespace string, opts types.ListOptions) ([]*types.VirtualMachineServiceInfo, error)
	Create(ctx context.Context, vms *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error)
	// Update applies the mutable fields from update to the existing object identified
	// by namespace/name. It is a read-modify-write operation; callers should handle
	// 409 Conflict by retrying on the next reconcile cycle.
	Update(ctx context.Context, namespace, name string, update *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error)
	Delete(ctx context.Context, namespace, name string) error
}
