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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
	util "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice/testutil"

	"github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
)

var (
	testClusterNameSpace    = "test-guest-cluster-ns"
	testClustername         = "test-cluster"
	testK8sServiceName      = "test-lb-service"
	testK8sServiceNameSpace = "test-service-ns"
	testOwnerReference      = metav1.OwnerReference{
		APIVersion: "v1alpha1",
		Kind:       "TanzuKubernetesCluster",
		Name:       testClustername,
		UID:        "1bbf49a7-fbce-4502-bb4c-4c3544cacc9e",
	}
)

func newTestLoadBalancer() (cloudprovider.LoadBalancer, *util.FakeClientWrapper) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := fakeClient.NewFakeClientWithScheme(scheme)
	fcw := util.NewFakeClientWrapper(fc)
	vms := vmservice.NewVMService(fcw, testClusterNameSpace, &testOwnerReference)
	return &loadBalancer{
		vmService: vms,
	}, fcw
}

func TestNewLoadBalancer(t *testing.T) {
	testCases := []struct {
		name    string
		testEnv *envtest.Environment
		err     error
	}{
		{
			name:    "NewLoadBalancer: when everything is ok",
			testEnv: &envtest.Environment{},
			err:     nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg, err := testCase.testEnv.Start()
			assert.NoError(t, err)

			_, err = NewLoadBalancer(testClusterNameSpace, cfg, &testOwnerReference)
			assert.NoError(t, err)
			assert.Equal(t, testCase.err, err)

			err = testCase.testEnv.Stop()
			assert.NoError(t, err)
		})
	}
}

func TestGetLoadBalancer_VMServiceNotFound(t *testing.T) {
	lb, _ := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}

	_, exists, err := lb.GetLoadBalancer(context.Background(), testClustername, testK8sService)
	assert.Equal(t, exists, false)
	assert.Error(t, err)
}

func TestGetLoadBalancer_VMServiceCreated(t *testing.T) {
	lb, _ := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}

	_, err := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.Equal(t, vmservice.ErrVMServiceIPNotFound, err)

	_, exists, err := lb.GetLoadBalancer(context.Background(), testClustername, testK8sService)
	assert.Equal(t, exists, true)
	assert.NoError(t, err)

	err = lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestUpdateLoadBalancer_GetVMServiceFailed(t *testing.T) {
	lb, _ := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}

	err := lb.UpdateLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	// Error should be NotFound during the Get() call
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "VirtualMachineService not found")
}

func TestUpdateLoadBalancer(t *testing.T) {
	testCases := []struct {
		name      string
		expectErr bool
	}{
		{
			name:      "when VMService update failed",
			expectErr: true,
		},
		{
			name:      "when VMService is updated",
			expectErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lb, fcw := newTestLoadBalancer()
			testK8sService := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testK8sServiceName,
					Namespace: testK8sServiceNameSpace,
				},
			}

			// Add the service with no ports
			_, err := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
			assert.Equal(t, vmservice.ErrVMServiceIPNotFound, err)

			// Update the service definition to add ports
			testK8sService.Spec = v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:     "test-port",
						Port:     80,
						NodePort: 30900,
						Protocol: "TCP",
					},
				},
			}

			if testCase.expectErr {
				// Ensure that the client Update call returns an error on update
				fcw.UpdateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
					return fmt.Errorf("Some undefined update error")
				}
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureLoadBalancer_VMServiceExternalTrafficPolicyLocal(t *testing.T) {
	lb, fcw := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
		Spec: v1.ServiceSpec{
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
		},
	}
	fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		vms := &vmopv1alpha1.VirtualMachineService{
			Status: vmopv1alpha1.VirtualMachineServiceStatus{
				LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
					Ingress: []v1alpha1.LoadBalancerIngress{
						{
							IP: "10.10.10.10",
						},
					},
				},
			},
		}
		vms.DeepCopyInto(obj.(*vmopv1alpha1.VirtualMachineService))
		return nil
	}

	_, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.NoError(t, ensureErr)

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer_VMServiceCreatedIPNotFound(t *testing.T) {
	// test when VMService is created but IP not found
	lb, _ := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}

	_, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.Equal(t, ensureErr, vmservice.ErrVMServiceIPNotFound)

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer_VMServiceCreationFailed(t *testing.T) {
	lb, fcw := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}
	// Ensure that the client Create call will fail. The preceding Get call should return NotFound
	fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		return fmt.Errorf("failed to create VirtualMachineService")
	}

	_, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.Error(t, ensureErr)

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer_VMServiceCreatedIPFound(t *testing.T) {
	lb, fcw := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}
	// Ensure that the client Create call returns a VMService with a valid IP
	fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		vms := &vmopv1alpha1.VirtualMachineService{
			Status: vmopv1alpha1.VirtualMachineServiceStatus{
				LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
					Ingress: []v1alpha1.LoadBalancerIngress{
						{
							IP: "10.10.10.10",
						},
					},
				},
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-vm-service-name",
				OwnerReferences: []metav1.OwnerReference{
					testOwnerReference,
				},
			},
			Spec: vmopv1alpha1.VirtualMachineServiceSpec{
				Type: vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
				Ports: []v1alpha1.VirtualMachineServicePort{
					{
						Name:       "test-port",
						Port:       80,
						TargetPort: 30800,
						Protocol:   "TCP",
					},
				},
				Selector: map[string]string{
					vmservice.ClusterSelectorKey: testClustername,
					vmservice.NodeSelectorKey:    vmservice.NodeRole,
				},
			},
		}
		vms.DeepCopyInto(obj.(*vmopv1alpha1.VirtualMachineService))
		return nil
	}

	status, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.NoError(t, ensureErr)
	assert.Equal(t, status.Ingress[0].IP, "10.10.10.10")

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer_DeleteLB(t *testing.T) {
	lb, fcw := newTestLoadBalancer()
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}

	// should pass without error
	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)

	// should ignore not found error
	fcw.DeleteFunc = func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
		return apierrors.NewNotFound(v1alpha1.Resource("virtualmachineservice"), testClustername)
	}
	err = lb.EnsureLoadBalancerDeleted(context.Background(), "test", testK8sService)
	assert.NoError(t, err)

	// should return error
	fcw.DeleteFunc = func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
		return fmt.Errorf("an error occurred while deleting load balancer")
	}

	err = lb.EnsureLoadBalancerDeleted(context.Background(), "test", testK8sService)
	assert.Equal(t, err.Error(), "an error occurred while deleting load balancer")
}
