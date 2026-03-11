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

package vsphereparavirtual

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
	"k8s.io/cloud-provider-vsphere/pkg/util"
)

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachineInfo if one exists, or nil otherwise.
// VirtualMachine not found is not an error.
//
// NOTE: This performs a namespace-scoped List of all VMs followed by an
// in-memory BiosUUID scan because BiosUUID is a status field and the
// kube-apiserver does not support FieldSelector on status subresources.
// This is an inherent API limitation. Callers that have the node name should
// use discoverNodeByName instead to avoid this O(n) scan.
func discoverNodeByProviderID(ctx context.Context, providerID string, namespace string, vmClient vmop.Interface) (*vmoptypes.VirtualMachineInfo, error) {
	// Parse the UUID once; it is derived from the immutable providerID and
	// does not change between retry attempts.
	uuid := GetUUIDFromProviderID(providerID)

	var discoveredNode *vmoptypes.VirtualMachineInfo

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			// GetByBiosUUID returns (nil, nil) for an empty UUID per its contract.
			vm, err := vmClient.VirtualMachines().GetByBiosUUID(ctx, namespace, uuid)
			if err != nil {
				return err
			}
			discoveredNode = vm
			return nil
		})

	return discoveredNode, err
}

// discoverNodeByName takes a node name and returns a VirtualMachineInfo if one exists, or nil otherwise.
// VirtualMachine not found is not an error.
func discoverNodeByName(ctx context.Context, name types.NodeName, namespace string, vmClient vmop.Interface) (*vmoptypes.VirtualMachineInfo, error) {
	var discoveredNode *vmoptypes.VirtualMachineInfo

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			vm, err := vmClient.VirtualMachines().Get(ctx, namespace, string(name))
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			discoveredNode = vm
			return nil
		})

	return discoveredNode, err
}
