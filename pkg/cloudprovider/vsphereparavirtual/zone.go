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

package vsphereparavirtual

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

type zones struct {
	vmClient  vmop.Interface
	namespace string
}

// Compile-time assertion that zones implements cloudprovider.Zones.
var _ cloudprovider.Zones = &zones{}

func (z zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	zone := cloudprovider.Zone{}
	return zone, cloudprovider.NotImplemented
}

func (z zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	zone := cloudprovider.Zone{}

	vm, err := z.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find vm :  %v", err)
		return zone, err
	}

	if vm == nil {
		klog.V(4).Info("instances.GetZoneByProviderID() InstanceNotFound ", providerID)
		return zone, cloudprovider.InstanceNotFound
	}

	if val, ok := vm.Labels["topology.kubernetes.io/zone"]; ok {
		klog.V(4).Info("retrieved zone", val)
		zone = cloudprovider.Zone{
			FailureDomain: val,
		}
	}

	return zone, nil
}

func (z zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	zone := cloudprovider.Zone{}

	vm, err := z.discoverNodeByName(ctx, nodeName)
	if err != nil {
		klog.Errorf("Error trying to find vm :  %v", err)
		return zone, err
	}

	if vm == nil {
		klog.V(4).Info("zones.GetZoneByNodeName() InstanceNotFound ", nodeName)
		return zone, cloudprovider.InstanceNotFound
	}

	if val, ok := vm.Labels["topology.kubernetes.io/zone"]; ok {
		klog.V(4).Info("retrieved zone", val)
		zone = cloudprovider.Zone{
			FailureDomain: val,
		}
	}

	return zone, nil
}

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachineInfo if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (z zones) discoverNodeByProviderID(ctx context.Context, providerID string) (*vmoptypes.VirtualMachineInfo, error) {
	return discoverNodeByProviderID(ctx, providerID, z.namespace, z.vmClient)
}

// discoverNodeByName takes a node name and returns a VirtualMachineInfo if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (z zones) discoverNodeByName(ctx context.Context, name types.NodeName) (*vmoptypes.VirtualMachineInfo, error) {
	return discoverNodeByName(ctx, name, z.namespace, z.vmClient)
}

// NewZones returns an implementation of cloudprovider.Zones
func NewZones(namespace string, vmClient vmop.Interface) (cloudprovider.Zones, error) {
	return &zones{
		vmClient:  vmClient,
		namespace: namespace,
	}, nil
}
