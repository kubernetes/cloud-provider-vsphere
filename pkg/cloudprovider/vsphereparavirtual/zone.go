package vsphereparavirtual

import (
	"context"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
	"k8s.io/klog/v2"
)

type zones struct {
	vmClient  vmop.V1alpha1Interface
	namespace string
}

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

// discoverNodeByProviderID takes a ProviderID and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (z zones) discoverNodeByProviderID(ctx context.Context, providerID string) (*vmopv1alpha1.VirtualMachine, error) {
	return discoverNodeByProviderID(ctx, providerID, z.namespace, z.vmClient)
}

// discoverNodeByName takes a node name and returns a VirtualMachine if one exists, or nil otherwise
// VirtualMachine not found is not an error
func (z zones) discoverNodeByName(ctx context.Context, name types.NodeName) (*vmopv1alpha1.VirtualMachine, error) {
	return discoverNodeByName(ctx, name, z.namespace, z.vmClient)
}

// NewZones returns an implementation of cloudprovider.Instances
func NewZones(namespace string, kcfg *rest.Config) (cloudprovider.Zones, error) {
	vmClient, err := vmservice.GetVmopClient(kcfg)

	if err != nil {
		return nil, err
	}

	return &zones{
		vmClient:  vmClient,
		namespace: namespace,
	}, nil
}
