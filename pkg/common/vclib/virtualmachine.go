/*
Copyright 2018 The Kubernetes Authors.

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

package vclib

import (
	"context"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	klog "k8s.io/klog/v2"
)

// VirtualMachine extends the govmomi VirtualMachine object
type VirtualMachine struct {
	*object.VirtualMachine
	Datacenter *Datacenter
}

// IsActive checks if the VM is active.
// Returns true if VM is in poweredOn state.
func (vm *VirtualMachine) IsActive(ctx context.Context) (bool, error) {
	vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*VirtualMachine{vm}, []string{"summary"})
	if err != nil {
		klog.Errorf("Failed to get VM Managed object with property summary. err: +%v", err)
		return false, err
	}
	if vmMoList[0].Summary.Runtime.PowerState == ActivePowerState {
		return true, nil
	}

	return false, nil
}

// Exists chech whether VM exist by searching by managed object reference
func (vm *VirtualMachine) Exists(ctx context.Context) (bool, error) {
	vmMoList, err := vm.Datacenter.GetVMMoList(ctx, []*VirtualMachine{vm}, []string{"summary.runtime.powerState"})
	if err != nil {
		if IsManagedObjectNotFoundError(err) {
			klog.Errorf("VM's ManagedObject is not found, assume it's already deleted, err: +%v", err)
			return false, nil
		}
		klog.Errorf("Failed to get VM Managed object with property summary. err: +%v", err)
		return false, err
	}
	// We check for VMs which are still available in vcenter and has not been terminated/removed from
	// disk and hence we consider PoweredOn,PoweredOff and Suspended as alive states.
	aliveStates := []types.VirtualMachinePowerState{
		types.VirtualMachinePowerStatePoweredOff,
		types.VirtualMachinePowerStatePoweredOn,
		types.VirtualMachinePowerStateSuspended,
	}
	currentState := vmMoList[0].Summary.Runtime.PowerState
	for _, state := range aliveStates {
		if state == currentState {
			return true, nil
		}
	}
	return false, nil
}
