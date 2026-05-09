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

// Package v1alpha6 provides a vmoperator.Interface adapter for the VM Operator v1alpha6 API.
// VirtualMachineService dual-stack fields (ipFamilies, ipFamilyPolicy) are merged via
// unstructured JSON when they are not yet present on generated Go types.
package v1alpha6

import (
	"context"

	vmopv6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/networkutil"
	clientv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha6"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

// Adapter implements vmoperator.Interface for the v1alpha6 API version.
type Adapter struct {
	vms  *virtualMachines
	vmss *virtualMachineServices
}

// Compile-time assertion that Adapter implements vmoperator.Interface.
var _ vmop.Interface = &Adapter{}

// New creates a new v1alpha6 Adapter using the provided REST config.
func New(cfg *rest.Config) (*Adapter, error) {
	c, err := clientv6.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return newAdapter(c), nil
}

// NewWithFakeClient creates a v1alpha6 Adapter backed by a fake client for testing.
func NewWithFakeClient(fakeClient *clientv6.Client) *Adapter {
	return newAdapter(fakeClient)
}

func newAdapter(c *clientv6.Client) *Adapter {
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

// virtualMachines implements vmop.VirtualMachineInterface for the v1alpha6 API.
type virtualMachines struct {
	client *clientv6.Client
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

// virtualMachineServices implements vmop.VirtualMachineServiceInterface for the v1alpha6 API.
type virtualMachineServices struct {
	client *clientv6.Client
}

// Compile-time assertion that virtualMachineServices implements vmop.VirtualMachineServiceInterface.
var _ vmop.VirtualMachineServiceInterface = &virtualMachineServices{}

func (v *virtualMachineServices) Get(ctx context.Context, namespace, name string) (*types.VirtualMachineServiceInfo, error) {
	raw, err := v.client.Dynamic().Resource(clientv6.VirtualMachineServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var vmService vmopv6.VirtualMachineService
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.UnstructuredContent(), &vmService); err != nil {
		return nil, err
	}
	info := vmServiceToInfo(&vmService)
	if err := readDualStackFromUnstructured(raw.UnstructuredContent(), &info.Spec); err != nil {
		return nil, err
	}
	return info, nil
}

func (v *virtualMachineServices) List(ctx context.Context, namespace string, opts types.ListOptions) ([]*types.VirtualMachineServiceInfo, error) {
	// Use the dynamic client directly so that dual-stack fields can be read from
	// the unstructured representation; the typed ListVirtualMachineServices path
	// would discard them during conversion.
	// ResourceVersion="0" reads from the API server watch cache (same default as
	// client.ListVirtualMachineServices / client.ListVirtualMachines).
	listOpts := metav1.ListOptions{
		LabelSelector:   opts.LabelSelector,
		ResourceVersion: "0",
	}
	rawList, err := v.client.Dynamic().Resource(clientv6.VirtualMachineServiceGVR).Namespace(namespace).List(ctx, listOpts)
	if err != nil {
		return nil, err
	}
	result := make([]*types.VirtualMachineServiceInfo, 0, len(rawList.Items))
	for i := range rawList.Items {
		item := rawList.Items[i]
		var vmService vmopv6.VirtualMachineService
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), &vmService); err != nil {
			return nil, err
		}
		info := vmServiceToInfo(&vmService)
		if err := readDualStackFromUnstructured(item.UnstructuredContent(), &info.Spec); err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

func (v *virtualMachineServices) Create(ctx context.Context, info *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	vmService := infoToVMService(info)
	// The API server rejects a Create request that carries a non-empty ResourceVersion.
	// Zero it defensively here so that callers do not need to remember this constraint.
	vmService.ResourceVersion = ""
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vmService)
	if err != nil {
		return nil, err
	}
	if err := writeDualStackToUnstructured(raw, &info.Spec); err != nil {
		return nil, err
	}
	raw["apiVersion"] = clientv6.VirtualMachineServiceGVR.Group + "/" + clientv6.VirtualMachineServiceGVR.Version
	raw["kind"] = "VirtualMachineService"
	created, err := v.client.Dynamic().Resource(clientv6.VirtualMachineServiceGVR).Namespace(info.Namespace).Create(ctx, &unstructured.Unstructured{Object: raw}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	var out vmopv6.VirtualMachineService
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(created.UnstructuredContent(), &out); err != nil {
		return nil, err
	}
	result := vmServiceToInfo(&out)
	if err := readDualStackFromUnstructured(created.UnstructuredContent(), &result.Spec); err != nil {
		return nil, err
	}
	return result, nil
}

func (v *virtualMachineServices) Update(ctx context.Context, namespace, name string, update *types.VirtualMachineServiceInfo) (*types.VirtualMachineServiceInfo, error) {
	raw, err := v.client.Dynamic().Resource(clientv6.VirtualMachineServiceGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var existing vmopv6.VirtualMachineService
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.UnstructuredContent(), &existing); err != nil {
		return nil, err
	}
	updated := existing.DeepCopy()
	applyVMServiceUpdate(updated, update)
	outObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(updated)
	if err != nil {
		return nil, err
	}
	if err := writeDualStackToUnstructured(outObj, &update.Spec); err != nil {
		return nil, err
	}
	outObj["apiVersion"] = clientv6.VirtualMachineServiceGVR.Group + "/" + clientv6.VirtualMachineServiceGVR.Version
	outObj["kind"] = "VirtualMachineService"
	updatedRaw, err := v.client.Dynamic().Resource(clientv6.VirtualMachineServiceGVR).Namespace(namespace).Update(ctx, &unstructured.Unstructured{Object: outObj}, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	var resultTyped vmopv6.VirtualMachineService
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(updatedRaw.UnstructuredContent(), &resultTyped); err != nil {
		return nil, err
	}
	result := vmServiceToInfo(&resultTyped)
	if err := readDualStackFromUnstructured(updatedRaw.UnstructuredContent(), &result.Spec); err != nil {
		return nil, err
	}
	return result, nil
}

func (v *virtualMachineServices) Delete(ctx context.Context, namespace, name string) error {
	return v.client.DeleteVirtualMachineService(ctx, namespace, name)
}

// --- conversion helpers ---

func vmToInfo(vm *vmopv6.VirtualMachine) *types.VirtualMachineInfo {
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

func vmServiceToInfo(vmService *vmopv6.VirtualMachineService) *types.VirtualMachineServiceInfo {
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

// infoToVMService converts a hub VirtualMachineServiceInfo to a v1alpha6 VirtualMachineService.
// Only fields that are set on Create are populated; status is not carried over.
// ResourceVersion is copied from info; callers must ensure it is empty on Create
// (the API server rejects a Create with a non-empty ResourceVersion).
func infoToVMService(info *types.VirtualMachineServiceInfo) *vmopv6.VirtualMachineService {
	vmService := &vmopv6.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            info.Name,
			Namespace:       info.Namespace,
			ResourceVersion: info.ResourceVersion,
			Labels:          info.Labels,
			Annotations:     info.Annotations,
			OwnerReferences: info.OwnerReferences,
		},
		Spec: vmopv6.VirtualMachineServiceSpec{
			Type:                     vmopv6.VirtualMachineServiceType(string(info.Spec.Type)),
			Selector:                 info.Spec.Selector,
			LoadBalancerIP:           info.Spec.LoadBalancerIP,
			LoadBalancerSourceRanges: info.Spec.LoadBalancerSourceRanges,
			Ports:                    make([]vmopv6.VirtualMachineServicePort, 0, len(info.Spec.Ports)),
		},
	}
	for _, p := range info.Spec.Ports {
		vmService.Spec.Ports = append(vmService.Spec.Ports, vmopv6.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
	return vmService
}

// applyVMServiceUpdate copies mutable fields from update into dst.
// IPFamilies and IPFamilyPolicy are intentionally not set here because the v1alpha6
// Go types do not yet carry those fields; they are written directly onto the
// unstructured map by writeDualStackToUnstructured after ToUnstructured conversion.
func applyVMServiceUpdate(dst *vmopv6.VirtualMachineService, update *types.VirtualMachineServiceInfo) {
	dst.Annotations = update.Annotations
	dst.Spec.LoadBalancerIP = update.Spec.LoadBalancerIP
	dst.Spec.LoadBalancerSourceRanges = update.Spec.LoadBalancerSourceRanges
	dst.Spec.Ports = make([]vmopv6.VirtualMachineServicePort, 0, len(update.Spec.Ports))
	for _, p := range update.Spec.Ports {
		dst.Spec.Ports = append(dst.Spec.Ports, vmopv6.VirtualMachineServicePort{
			Name:       p.Name,
			Protocol:   p.Protocol,
			Port:       p.Port,
			TargetPort: p.TargetPort,
		})
	}
}
