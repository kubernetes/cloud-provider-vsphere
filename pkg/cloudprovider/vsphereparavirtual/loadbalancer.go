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

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"github.com/pkg/errors"
	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
)

// loadBalancer implements cloudprovider.LoadBalancer interface
type loadBalancer struct {
	vmService vmservice.VMService
}

// NewLoadBalancer returns an implementation of cloudprovider.LoadBalancer
func NewLoadBalancer(clusterNS string, kcfg *rest.Config, ownerRef *metav1.OwnerReference) (cloudprovider.LoadBalancer, error) {
	klog.V(1).Info("Create load balancer for vsphere paravirtual cloud provider")

	client, err := vmservice.GetVmopClient(kcfg)
	if err != nil {
		klog.Errorf("failed to create load balancer: %v", err)
		return nil, err
	}
	vmService := vmservice.NewVMService(client, clusterNS, ownerRef)
	return &loadBalancer{
		vmService: vmService,
	}, nil
}

// TODO: Break this up into different interfaces (LB, etc) when we have more than one type of service
// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	klog.V(1).Infof("Get load balancer for %s", namespacedName(service))

	vmService, err := l.vmService.Get(ctx, service, clusterName)

	if err != nil {
		klog.Errorf("failed to get load balancer for %s: %v", namespacedName(service), err)
		return nil, false, err
	}

	if vmService == nil {
		klog.Errorf("failed to get load balancer for %s: VirtualMachineService not found", namespacedName(service))
		return nil, false, errors.Errorf("VirtualMachineService not found")
	}

	return toStatus(vmService), true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancer) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	klog.V(1).Infof("Get load balancer name for service  %s", namespacedName(service))
	//TODO: confirm what name should be used here: vmService name? the real lb name on nsx-t ?

	return l.vmService.GetVMServiceName(service, clusterName)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	klog.V(1).Infof("Ensure Load Balancer for %s", namespacedName(service))

	vmService, err := l.vmService.CreateOrUpdate(ctx, service, clusterName)

	if err != nil {
		klog.Errorf("failed to ensure virtual machine service for %s: %v", namespacedName(service), err)
		return nil, err
	}

	klog.V(1).Infof("Ensured load balancer for %s with virtual machine service %s", namespacedName(service), vmService.Name)

	return toStatus(vmService), nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	klog.V(1).Infof("Update load balancer for %s", namespacedName(service))

	vmService, err := l.vmService.Get(ctx, service, clusterName)

	if err != nil {
		klog.Errorf("failed to update load balancer for %s: %v", namespacedName(service), err)
		return err
	}

	if vmService == nil {
		klog.Errorf("failed to update load balancer for %s: VirtualMachineService not found", namespacedName(service))
		return errors.Errorf("VirtualMachineService not found")
	}

	vmService, err = l.vmService.Update(ctx, service, clusterName, vmService)

	if err != nil {
		klog.Errorf("failed to update virtual machine service for %s: %v", namespacedName(service), err)
		return err
	}

	klog.V(1).Infof("updated virtual machine service: %s", vmService.Name)
	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	klog.V(1).Infof("Ensure load balancer is deleted %s", namespacedName(service))

	err := l.vmService.Delete(ctx, service, clusterName)

	if err != nil {
		if !k8serrors.IsNotFound(err) {
			klog.Errorf("failed to delete load balancer for %s", namespacedName(service))
			return err
		}
		klog.V(1).Infof("load balancer for %s is not found", namespacedName(service))
	}

	klog.V(1).Infof("load balancer for %s is deleted", namespacedName(service))

	return nil
}

func toStatus(vmService *vmopv1alpha1.VirtualMachineService) *v1.LoadBalancerStatus {

	if len(vmService.Status.LoadBalancer.Ingress) > 0 {
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: vmService.Status.LoadBalancer.Ingress[0].IP,
				},
			},
		}
	}
	return &v1.LoadBalancerStatus{}
}

func namespacedName(service *v1.Service) string {
	return service.Namespace + "/" + service.Name
}
