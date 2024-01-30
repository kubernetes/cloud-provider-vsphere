package vsphereparavirtual

import (
	"context"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider-vsphere/pkg/util"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
)

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func discoverNodeByProviderID(ctx context.Context, providerID string, namespace string, vmClient vmop.V1alpha1Interface) (*vmopv1alpha1.VirtualMachine, error) {
	var discoveredNode *vmopv1alpha1.VirtualMachine = nil

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			uuid := GetUUIDFromProviderID(providerID)
			vms, err := vmClient.VirtualMachines(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for i := range vms.Items {
				vm := vms.Items[i]
				if uuid == vm.Status.BiosUUID {
					discoveredNode = &vm
					break
				}
			}

			return nil
		})

	return discoveredNode, err
}

// discoverNodeByName takes a node name and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func discoverNodeByName(ctx context.Context, name types.NodeName, namespace string, vmClient vmop.V1alpha1Interface) (*vmopv1alpha1.VirtualMachine, error) {
	var discoveredNode *vmopv1alpha1.VirtualMachine = nil

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			vm, err := vmClient.VirtualMachines(namespace).Get(ctx, string(name), metav1.GetOptions{})
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
