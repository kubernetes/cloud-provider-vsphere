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
	"errors"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
)

type instances struct {
	vmClient  client.Client
	namespace string
}

const (
	// providerPrefix is the Kubernetes cloud provider prefix for this
	// cloud provider.
	providerPrefix = ProviderName + "://"
)

// DiscoverNodeBackoff is set to be the same with https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L83
var DiscoverNodeBackoff = wait.Backoff{
	Steps:    20,
	Duration: 50 * time.Millisecond,
	Jitter:   1.0,
}

var (
	errBiosUUIDEmpty = errors.New("discovered Bios UUID is empty")
)

func checkError(err error) bool {
	return err != nil
}

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (i instances) discoverNodeByProviderID(ctx context.Context, providerID string) (*vmopv1alpha1.VirtualMachine, error) {
	return discoverNodeByProviderID(ctx, providerID, i.namespace, i.vmClient)
}

// discoverNodeByName takes a node name and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (i instances) discoverNodeByName(ctx context.Context, name types.NodeName) (*vmopv1alpha1.VirtualMachine, error) {
	return discoverNodeByName(ctx, name, i.namespace, i.vmClient)
}

// NewInstances returns an implementation of cloudprovider.Instances
func NewInstances(clusterNS string, kcfg *rest.Config) (cloudprovider.Instances, error) {
	vmClient, err := vmservice.GetVmopClient(kcfg)

	if err != nil {
		return nil, err
	}

	return &instances{
		vmClient:  vmClient,
		namespace: clusterNS,
	}, nil
}

func createNodeAddresses(vm *vmopv1alpha1.VirtualMachine) []v1.NodeAddress {
	if vm.Status.VmIp == "" {
		klog.V(4).Info("instance found, but no address yet")
		return []v1.NodeAddress{}
	}
	return []v1.NodeAddress{
		{
			Type:    v1.NodeInternalIP,
			Address: vm.Status.VmIp,
		},
		{
			Type:    v1.NodeHostName,
			Address: "",
		},
	}
}

// NodeAddresses returns the addresses of the specified instance if one exists, otherwise nil
// If the instance exists but does not yet have an IP address, the function returns a zero length slice
func (i *instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddresses() called with ", name)

	vm, err := i.discoverNodeByName(ctx, name)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return nil, err
	}
	if vm == nil {
		klog.V(4).Info("instances.NodeAddresses() InstanceNotFound ", name)
		return nil, cloudprovider.InstanceNotFound
	}
	return createNodeAddresses(vm), err
}

// NodeAddressesByProviderID returns the addresses of the specified instance if one exists, otherwise nil
// If the instance exists but does not yet have an IP address, the function returns a zero length slice
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddressesByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return nil, err
	}
	if vm == nil {
		klog.V(4).Info("instances.NodeAddressesByProviderID() InstanceNotFound ", providerID)
		return nil, cloudprovider.InstanceNotFound
	}
	return createNodeAddresses(vm), nil
}

// InstanceID returns the cloud provider ID of the named instance if one exists, otherwise an empty string
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	vm, err := i.discoverNodeByName(ctx, nodeName)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return "", err
	}
	if vm == nil {
		klog.V(4).Info("instances.InstanceID() InstanceNotFound ", nodeName)
		return "", cloudprovider.InstanceNotFound
	}

	if vm.Status.BiosUUID == "" {
		return "", errBiosUUIDEmpty
	}

	klog.V(4).Infof("instances.InstanceID() called to get vm: %v uuid: %v", nodeName, vm.Status.BiosUUID)
	return vm.Status.BiosUUID, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.V(4).Info("instances.InstanceTypeByProviderID() called with ", name)
	return "", nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.V(4).Info("instances.InstanceTypeByProviderID() called with ", providerID)
	return "", nil
}

// CurrentNodeName returns the name of the node we are currently running on
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	klog.V(4).Info("instances.CurrentNodeName() called with ", hostname)
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceExistsByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return false, err
	}
	return vm != nil, nil
}

// InstanceShutdownByProviderID returns true if the instance exists and is shut down
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceShutdownByProviderID() called with ", providerID)

	vm, err := i.discoverNodeByProviderID(ctx, providerID)
	if err != nil {
		klog.Errorf("Error trying to find VM: %v", err)
		return false, err
	}
	if vm == nil {
		klog.V(4).Info("instances.InstanceShutdownByProviderID() InstanceNotFound ", providerID)
		return false, cloudprovider.InstanceNotFound
	}
	return vm.Status.PowerState == vmopv1alpha1.VirtualMachinePoweredOff, nil
}

func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	klog.V(4).Info("instances.AddSSHKeyToAllInstances() called")
	return cloudprovider.NotImplemented
}

// GetUUIDFromProviderID returns a UUID from the supplied cloud provider ID.
func GetUUIDFromProviderID(providerID string) string {
	withoutPrefix := strings.TrimPrefix(providerID, providerPrefix)
	return strings.ToLower(strings.TrimSpace(withoutPrefix))
}
