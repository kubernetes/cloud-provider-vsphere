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

package applyconfiguration

import (
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	v1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmopv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmop/applyconfiguration/vmop/v1alpha1"
)

// ForKind returns an apply configuration type for the given GroupVersionKind, or nil if no
// apply configuration type exists for the given GroupVersionKind.
func ForKind(kind schema.GroupVersionKind) interface{} {
	switch kind {
	// Group=vmoperator.vmware.com, Version=v1alpha1
	case v1alpha1.SchemeGroupVersion.WithKind("ClusterModuleSpec"):
		return &vmopv1alpha1.ClusterModuleSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ClusterModuleStatus"):
		return &vmopv1alpha1.ClusterModuleStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("FolderSpec"):
		return &vmopv1alpha1.FolderSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("LoadBalancerIngress"):
		return &vmopv1alpha1.LoadBalancerIngressApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("LoadBalancerStatus"):
		return &vmopv1alpha1.LoadBalancerStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("NetworkInterfaceProviderReference"):
		return &vmopv1alpha1.NetworkInterfaceProviderReferenceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("Probe"):
		return &vmopv1alpha1.ProbeApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("ResourcePoolSpec"):
		return &vmopv1alpha1.ResourcePoolSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("TCPSocketAction"):
		return &vmopv1alpha1.TCPSocketActionApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachine"):
		return &vmopv1alpha1.VirtualMachineApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineAdvancedOptions"):
		return &vmopv1alpha1.VirtualMachineAdvancedOptionsApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineClass"):
		return &vmopv1alpha1.VirtualMachineClassApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineClassHardware"):
		return &vmopv1alpha1.VirtualMachineClassHardwareApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineClassPolicies"):
		return &vmopv1alpha1.VirtualMachineClassPoliciesApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineClassResources"):
		return &vmopv1alpha1.VirtualMachineClassResourcesApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineClassSpec"):
		return &vmopv1alpha1.VirtualMachineClassSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineCondition"):
		return &vmopv1alpha1.VirtualMachineConditionApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineImage"):
		return &vmopv1alpha1.VirtualMachineImageApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineImageOSInfo"):
		return &vmopv1alpha1.VirtualMachineImageOSInfoApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineImageProductInfo"):
		return &vmopv1alpha1.VirtualMachineImageProductInfoApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineImageSpec"):
		return &vmopv1alpha1.VirtualMachineImageSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineImageStatus"):
		return &vmopv1alpha1.VirtualMachineImageStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineMetadata"):
		return &vmopv1alpha1.VirtualMachineMetadataApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineNetworkInterface"):
		return &vmopv1alpha1.VirtualMachineNetworkInterfaceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachinePort"):
		return &vmopv1alpha1.VirtualMachinePortApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineResourceSpec"):
		return &vmopv1alpha1.VirtualMachineResourceSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineService"):
		return &vmopv1alpha1.VirtualMachineServiceApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineServicePort"):
		return &vmopv1alpha1.VirtualMachineServicePortApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineServiceSpec"):
		return &vmopv1alpha1.VirtualMachineServiceSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineServiceStatus"):
		return &vmopv1alpha1.VirtualMachineServiceStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineSetResourcePolicy"):
		return &vmopv1alpha1.VirtualMachineSetResourcePolicyApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineSetResourcePolicySpec"):
		return &vmopv1alpha1.VirtualMachineSetResourcePolicySpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineSetResourcePolicyStatus"):
		return &vmopv1alpha1.VirtualMachineSetResourcePolicyStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineSpec"):
		return &vmopv1alpha1.VirtualMachineSpecApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineStatus"):
		return &vmopv1alpha1.VirtualMachineStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineVolume"):
		return &vmopv1alpha1.VirtualMachineVolumeApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineVolumeProvisioningOptions"):
		return &vmopv1alpha1.VirtualMachineVolumeProvisioningOptionsApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VirtualMachineVolumeStatus"):
		return &vmopv1alpha1.VirtualMachineVolumeStatusApplyConfiguration{}
	case v1alpha1.SchemeGroupVersion.WithKind("VsphereVolumeSource"):
		return &vmopv1alpha1.VsphereVolumeSourceApplyConfiguration{}

	}
	return nil
}