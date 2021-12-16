package vsphereparavirtual

import (
	"context"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider-vsphere/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func discoverNodeByProviderID(ctx context.Context, providerID string, namespace string, vmClient client.Client) (*vmopv1alpha1.VirtualMachine, error) {
	var discoveredNode *vmopv1alpha1.VirtualMachine = nil

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			uuid := GetUUIDFromProviderID(providerID)
			vms := vmopv1alpha1.VirtualMachineList{}
			err := vmClient.List(ctx, &vms, &client.ListOptions{
				Namespace: namespace,
			})
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
func discoverNodeByName(ctx context.Context, name types.NodeName, namespace string, vmClient client.Client) (*vmopv1alpha1.VirtualMachine, error) {
	var discoveredNode *vmopv1alpha1.VirtualMachine = nil

	// Adding Retry here because there is no retry in caller from node controller
	// https://github.com/kubernetes/kubernetes/blob/master/pkg/controller/cloud/node_controller.go#L368
	err := util.RetryOnError(
		DiscoverNodeBackoff,
		checkError,
		func() error {
			vmKey := types.NamespacedName{Name: string(name), Namespace: namespace}
			vm := vmopv1alpha1.VirtualMachine{}
			err := vmClient.Get(ctx, vmKey, &vm)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}
			discoveredNode = &vm
			return nil
		})

	return discoveredNode, err
}
