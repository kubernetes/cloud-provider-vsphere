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

// Package v1alpha5 provides a vmoperator.Interface adapter that translates
// between the hub types and the VM Operator v1alpha5 API.
//
// When a new VM Operator v1alpha5 field needs to be propagated by the CPI,
// add it to the hub types in the types package, then update vmsToInfo(),
// infoToVMS(), and applyVMSUpdate() in this file. The v1alpha2 adapter
// does not need changes.
package v1alpha5

import (
	"context"

	vmopv5 "github.com/vmware-tanzu/vm-operator/api/v1alpha5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	providerv5 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha5"
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
	c, err := providerv5.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newAdapter(c), nil
}

// NewWithFakeClient creates a v1alpha5 Adapter backed by a fake client for testing.
func NewWithFakeClient(fakeClient *providerv5.Client) *Adapter {
	return newAdapter(fakeClient)
}

func newAdapter(c *providerv5.Client) *Adapter {
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
	client *providerv5.Client
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

// GetByBiosUUID lists all VMs in the namespace and returns the one whose
// BiosUUID matches. BiosUUID is a status field; the kube-apiserver does not
// support FieldSelector on status subresources, so this is an in-memory scan.
// Returns nil without error when biosUUID is empty, because an empty UUID would
// match any VM whose BiosUUID has not yet been assigned, producing a false match.
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
	client *providerv5.Client
}

// Compile-time assertion that virtualMachineServices implements vmop.VirtualMachineServiceInterface.
var _ vmop.VirtualMachineServiceInterface = &virtualMachineServices{}

func (v *virtualMachineServices) Get(ctx context.Context, namespace, name string) (*types.VirtualMachineServiceInfo, error) {
	vms, err := v.client.GetVirtualMachineService(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	return vmsToInfo(vms), nil
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
		result = append(result, vmsToInfo(&list.Items[i]))
	}
	return result, nil
}

func (v *virtualMachineServices) Create(ctx context.Context, info *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	vms := infoToVMS(info)
	created, err := v.client.CreateVirtualMachineService(ctx, vms)
	if err != nil {
		return nil, err
	}
	return vmsToInfo(created), nil
}

func (v *virtualMachineServices) Update(ctx context.Context, namespace, name string, update *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	// Fetch the current object to get its ResourceVersion and preserve immutable
	// fields (Selector, Labels, OwnerReferences) that the caller does not supply.
	// A NotFound error is propagated as-is; the caller will retry on the next
	// reconcile cycle and trigger a Create if the object is gone.
	existing, err := v.client.GetVirtualMachineService(ctx, namespace, name)
	if err != nil {
		return nil, err
	}
	updated := existing.DeepCopy()
	applyVMSUpdate(updated, update)
	// TODO(https://github.com/kubernetes/cloud-provider-vsphere/issues/1130):
	// Handle 409 Conflict (ResourceVersion mismatch) with a retry loop.
	// Currently, a concurrent update between the Get and Update calls will
	// return an error to the caller rather than retrying automatically.
	result, err := v.client.UpdateVirtualMachineService(ctx, updated)
	if err != nil {
		return nil, err
	}
	return vmsToInfo(result), nil
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
	}
	return info
}

// vmsToInfo converts a v1alpha5 VirtualMachineService to the version-agnostic hub type.
func vmsToInfo(vms *vmopv5.VirtualMachineService) *types.VirtualMachineServiceInfo {
	info := &types.VirtualMachineServiceInfo{
		Name:            vms.Name,
		Namespace:       vms.Namespace,
		ResourceVersion: vms.ResourceVersion,
		Labels:          vms.Labels,
		Annotations:     vms.Annotations,
		OwnerReferences: vms.OwnerReferences,
		Spec: types.VirtualMachineServiceSpec{
			Type:                     types.VirtualMachineServiceType(vms.Spec.Type),
			Selector:                 vms.Spec.Selector,
			LoadBalancerIP:           vms.Spec.LoadBalancerIP,
			LoadBalancerSourceRanges: vms.Spec.LoadBalancerSourceRanges,
			Ports:                    make([]types.VirtualMachineServicePort, 0, len(vms.Spec.Ports)),
		},
		Status: types.VirtualMachineServiceStatus{
			LoadBalancerIngress: make([]types.LoadBalancerIngress, 0, len(vms.Status.LoadBalancer.Ingress)),
		},
	}
	for _, p := range vms.Spec.Ports {
		info.Spec.Ports = append(info.Spec.Ports, types.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
	for _, ing := range vms.Status.LoadBalancer.Ingress {
		info.Status.LoadBalancerIngress = append(info.Status.LoadBalancerIngress, types.LoadBalancerIngress{
			IP:       ing.IP,
			Hostname: ing.Hostname,
		})
	}
	return info
}

// infoToVMS converts a hub VirtualMachineServiceInfo to a v1alpha5 VirtualMachineService.
// Only fields that are set on Create are populated; status is not carried over.
func infoToVMS(info *types.VirtualMachineServiceInfo) *vmopv5.VirtualMachineService {
	vms := &vmopv5.VirtualMachineService{
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
		vms.Spec.Ports = append(vms.Spec.Ports, vmopv5.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
	return vms
}

// applyVMSUpdate copies mutable fields from update into dst.
func applyVMSUpdate(dst *vmopv5.VirtualMachineService, update *types.VirtualMachineServiceInfo) {
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
