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

// Package v1alpha5 provides a vmoperator.Interface adapter for the VM Operator v1alpha5 API.
// To expose a new v1alpha5 field, add it to the hub types package and update the
// conversion helpers in this file; the v1alpha2 adapter does not need changes.
package v1alpha5

import (
	"context"

	vmopv5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/networkutil"
	clientv5 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha5"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

// Adapter implements vmoperator.Interface for the v1alpha5 API version.
type Adapter struct {
	vms  *virtualMachines
	vmss *virtualMachineServices
}

// Compile-time assertion that Adapter implements vmoperator.Interface.
var _ vmop.Interface = &Adapter{}

// New creates a new v1alpha5 Adapter using the provided REST config.
func New(cfg *rest.Config) (*Adapter, error) {
	c, err := clientv5.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newAdapter(c), nil
}

// NewWithFakeClient creates a v1alpha5 Adapter backed by a fake client for testing.
func NewWithFakeClient(fakeClient *clientv5.Client) *Adapter {
	return newAdapter(fakeClient)
}

func newAdapter(c *clientv5.Client) *Adapter {
	return &Adapter{
		vms:  &virtualMachines{client: c},
		vmss: &virtualMachineServices{client: c},
	}
}

// VirtualMachines returns the VirtualMachineInterface for this adapter.
func (a *Adapter) VirtualMachines() vmop.VirtualMachineInterface {
	return a.vms
}

// VirtualMachineServices returns the VirtualMachineServiceInterface for this adapter.
func (a *Adapter) VirtualMachineServices() vmop.VirtualMachineServiceInterface {
	return a.vmss
}

// virtualMachines implements vmop.VirtualMachineInterface for the v1alpha5 API.
type virtualMachines struct {
	client *clientv5.Client
}

// Compile-time assertion that virtualMachines implements vmop.VirtualMachineInterface.
var _ vmop.VirtualMachineInterface = &virtualMachines{}

func (v *virtualMachines) Get(ctx context.Context, namespace, name string) (*types.VirtualMachineInfo, error) {
	vm, err := v.client.GetVirtualMachine(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return vmToInfo(vm), nil
}

func (v *virtualMachines) List(ctx context.Context, namespace string, opts types.ListOptions) ([]*types.VirtualMachineInfo, error) {
	list, err := v.client.ListVirtualMachines(ctx, namespace, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*types.VirtualMachineInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, vmToInfo(&list.Items[i]))
	}
	return result, nil
}

// GetByBiosUUID returns the VM whose BiosUUID matches, scanning in memory.
// FieldSelector is not supported on status subresources, so a full list is needed.
// An empty biosUUID returns nil immediately to avoid false matches.
func (v *virtualMachines) GetByBiosUUID(ctx context.Context, namespace, biosUUID string) (*types.VirtualMachineInfo, error) {
	if biosUUID == "" {
		return nil, nil
	}
	list, err := v.client.ListVirtualMachines(ctx, namespace, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].Status.BiosUUID == biosUUID {
			return vmToInfo(&list.Items[i]), nil
		}
	}
	return nil, nil
}

// virtualMachineServices implements vmop.VirtualMachineServiceInterface for the v1alpha5 API.
type virtualMachineServices struct {
	client *clientv5.Client
}

// Compile-time assertion that virtualMachineServices implements vmop.VirtualMachineServiceInterface.
var _ vmop.VirtualMachineServiceInterface = &virtualMachineServices{}

func (v *virtualMachineServices) Get(ctx context.Context, namespace, name string) (*types.VirtualMachineServiceInfo, error) {
	vmService, err := v.client.GetVirtualMachineService(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return vmServiceToInfo(vmService), nil
}

func (v *virtualMachineServices) List(ctx context.Context, namespace string, opts types.ListOptions) ([]*types.VirtualMachineServiceInfo, error) {
	list, err := v.client.ListVirtualMachineServices(ctx, namespace, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	})
	if err != nil {
		return nil, err
	}
	result := make([]*types.VirtualMachineServiceInfo, 0, len(list.Items))
	for i := range list.Items {
		result = append(result, vmServiceToInfo(&list.Items[i]))
	}
	return result, nil
}

func (v *virtualMachineServices) Create(ctx context.Context, info *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	vmService := infoToVMService(info)
	created, err := v.client.CreateVirtualMachineService(ctx, vmService)
	if err != nil {
		return nil, err
	}
	return vmServiceToInfo(created), nil
}

func (v *virtualMachineServices) Update(ctx context.Context, namespace, name string, update *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	// Always re-fetch to obtain the current ResourceVersion and to preserve
	// immutable fields the caller doesn't supply (Selector, Labels,
	// OwnerReferences). NotFound is returned as-is; the caller will create on
	// the next reconcile.
	existing, err := v.client.GetVirtualMachineService(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	updated := existing.DeepCopy()
	applyVMServiceUpdate(updated, update)
	// TODO(https://github.com/kubernetes/cloud-provider-vsphere/issues/1130):
	// The re-fetch above does not eliminate the race: a concurrent writer can
	// change the object between our Get and Update, causing a 409 Conflict.
	// Until automatic retry is implemented the conflict is propagated to the
	// caller, which will retry on the next reconcile cycle.
	result, err := v.client.UpdateVirtualMachineService(ctx, updated)
	if err != nil {
		return nil, err
	}
	return vmServiceToInfo(result), nil
}

func (v *virtualMachineServices) Delete(ctx context.Context, namespace, name string) error {
	return v.client.DeleteVirtualMachineService(ctx, namespace, name)
}

// --- conversion helpers ---

// vmToInfo converts a v1alpha5 VirtualMachine to the version-agnostic hub type.
func vmToInfo(vm *vmopv5.VirtualMachine) *types.VirtualMachineInfo {
	info := &types.VirtualMachineInfo{
		Name:       vm.Name,
		Namespace:  vm.Namespace,
		Labels:     vm.Labels,
		BiosUUID:   vm.Status.BiosUUID,
		PowerState: types.PowerState(vm.Status.PowerState),
	}
	if vm.Status.Network != nil {
		info.PrimaryIP4 = vm.Status.Network.PrimaryIP4
		info.PrimaryIP6 = vm.Status.Network.PrimaryIP6
		if vm.Status.Network.Interfaces != nil {
			for _, iface := range vm.Status.Network.Interfaces {
				if iface.IP == nil || iface.IP.Addresses == nil {
					continue
				}
				for _, ipAddr := range iface.IP.Addresses {
					info.NetworkInterfaceAddresses = append(info.NetworkInterfaceAddresses, networkutil.StripCIDRPrefix(ipAddr.Address))
				}
			}
		}
	}
	return info
}

// vmServiceToInfo converts a v1alpha5 VirtualMachineService to the version-agnostic hub type.
func vmServiceToInfo(vmService *vmopv5.VirtualMachineService) *types.VirtualMachineServiceInfo {
	info := &types.VirtualMachineServiceInfo{
		Name:            vmService.Name,
		Namespace:       vmService.Namespace,
		ResourceVersion: vmService.ResourceVersion,
		Labels:          vmService.Labels,
		Annotations:     vmService.Annotations,
		OwnerReferences: vmService.OwnerReferences,
		Spec: types.VirtualMachineServiceSpec{
			Type:                     types.VirtualMachineServiceType(vmService.Spec.Type),
			Selector:                 vmService.Spec.Selector,
			LoadBalancerIP:           vmService.Spec.LoadBalancerIP,
			LoadBalancerSourceRanges: vmService.Spec.LoadBalancerSourceRanges,
			Ports:                    make([]types.VirtualMachineServicePort, 0, len(vmService.Spec.Ports)),
		},
		Status: types.VirtualMachineServiceStatus{
			LoadBalancerIngress: make([]types.LoadBalancerIngress, 0, len(vmService.Status.LoadBalancer.Ingress)),
		},
	}
	for _, p := range vmService.Spec.Ports {
		info.Spec.Ports = append(info.Spec.Ports, types.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
	for _, ing := range vmService.Status.LoadBalancer.Ingress {
		info.Status.LoadBalancerIngress = append(info.Status.LoadBalancerIngress, types.LoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
		})
	}
	return info
}

// infoToVMService converts a hub VirtualMachineServiceInfo to a v1alpha5 VirtualMachineService.
// Only fields that are set on Create are populated; status is not carried over.
// ResourceVersion is copied from info; callers must ensure it is empty on Create
// (the API server rejects a Create with a non-empty ResourceVersion).
func infoToVMService(info *types.VirtualMachineServiceInfo) *vmopv5.VirtualMachineService {
	vmService := &vmopv5.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            info.Name,
			Namespace:       info.Namespace,
			ResourceVersion: info.ResourceVersion,
			Labels:          info.Labels,
			Annotations:     info.Annotations,
			OwnerReferences: info.OwnerReferences,
		},
		Spec: vmopv5.VirtualMachineServiceSpec{
			Type:                     vmopv5.VirtualMachineServiceType(string(info.Spec.Type)),
			Selector:                 info.Spec.Selector,
			LoadBalancerIP:           info.Spec.LoadBalancerIP,
			LoadBalancerSourceRanges: info.Spec.LoadBalancerSourceRanges,
			Ports:                    make([]vmopv5.VirtualMachineServicePort, 0, len(info.Spec.Ports)),
		},
	}
	for _, p := range info.Spec.Ports {
		vmService.Spec.Ports = append(vmService.Spec.Ports, vmopv5.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
	return vmService
}

// applyVMServiceUpdate copies mutable fields from update into dst.
func applyVMServiceUpdate(dst *vmopv5.VirtualMachineService, update *types.VirtualMachineServiceInfo) {
	dst.Annotations = update.Annotations
	dst.Spec.LoadBalancerIP = update.Spec.LoadBalancerIP
	dst.Spec.LoadBalancerSourceRanges = update.Spec.LoadBalancerSourceRanges
	dst.Spec.Ports = make([]vmopv5.VirtualMachineServicePort, 0, len(update.Spec.Ports))
	for _, p := range update.Spec.Ports {
		dst.Spec.Ports = append(dst.Spec.Ports, vmopv5.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
}
