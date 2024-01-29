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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	cloudprovider "k8s.io/cloud-provider"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"

	vmopclient "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/client"
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

func newTestLoadBalancer() (cloudprovider.LoadBalancer, *vmopclient.FakeClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	fcw := vmopclient.NewFakeClient(fc)

	vms := vmservice.NewVMService(fcw, testClusterNameSpace, &testOwnerReference)
	return &loadBalancer{vmService: vms}, fcw
}

func TestNewLoadBalancer(t *testing.T) {
	testCases := []struct {
		name   string
		config *rest.Config
		err    error
	}{
		{
			name:   "NewLoadBalancer: when everything is ok",
			config: &rest.Config{},
			err:    nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewLoadBalancer(testClusterNameSpace, testCase.config, &testOwnerReference)
			assert.Equal(t, testCase.err, err)
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
			expectErr: false,
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
				fcw.DynamicClient.PrependReactor("update", "virtualmachineservices", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, fmt.Errorf("Some undefined update error")
				})
				err = lb.UpdateLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
				assert.Error(t, err)
			} else {
				err = lb.UpdateLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
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

	fcw.DynamicClient.PrependReactor("create", "virtualmachineservices", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		unstructuredObj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&vmopv1alpha1.VirtualMachineService{
			Status: vmopv1alpha1.VirtualMachineServiceStatus{
				LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
					Ingress: []vmopv1alpha1.LoadBalancerIngress{
						{
							IP: "10.10.10.10",
						},
					},
				},
			},
		})
		return true, &unstructured.Unstructured{Object: unstructuredObj}, nil
	})

	_, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.NoError(t, ensureErr)

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer(t *testing.T) {
	testCases := []struct {
		name       string
		createFunc func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectErr  error
	}{
		{
			name: "when VMService is created but IP not found",
			createFunc: func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &vmopv1alpha1.VirtualMachineService{}, fmt.Errorf(vmservice.ErrVMServiceIPNotFound.Error())
			},
			expectErr: vmservice.ErrVMServiceIPNotFound,
		},
		{
			name: "when VMService creation failed",
			createFunc: func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, &vmopv1alpha1.VirtualMachineService{}, fmt.Errorf(vmservice.ErrCreateVMService.Error())
			},
			expectErr: vmservice.ErrCreateVMService,
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
			fcw.DynamicClient.PrependReactor("create", "virtualmachineservices", testCase.createFunc)

			_, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
			assert.Equal(t, ensureErr.Error(), testCase.expectErr.Error())

			err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
			assert.NoError(t, err)
		})
	}
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
	fcw.DynamicClient.PrependReactor("create", "virtualmachineservices", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		unstructuredObj, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(&vmopv1alpha1.VirtualMachineService{
			Status: vmopv1alpha1.VirtualMachineServiceStatus{
				LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
					Ingress: []vmopv1alpha1.LoadBalancerIngress{
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
				Ports: []vmopv1alpha1.VirtualMachineServicePort{
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
		})

		return true, &unstructured.Unstructured{Object: unstructuredObj}, nil
	})

	status, ensureErr := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
	assert.NoError(t, ensureErr)
	assert.Equal(t, status.Ingress[0].IP, "10.10.10.10")

	err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
	assert.NoError(t, err)
}

func TestEnsureLoadBalancer_DeleteLB(t *testing.T) {
	testCases := []struct {
		name       string
		deleteFunc func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectErr  string
	}{
		{
			name: "should ignore not found error",
			deleteFunc: func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, apierrors.NewNotFound(vmopv1alpha1.Resource("virtualmachineservice"), testClustername)
			},
		},
		{
			name: "should return error",
			deleteFunc: func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("an error occurred while deleting load balancer")
			},
			expectErr: "an error occurred while deleting load balancer",
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

			// should pass without error
			err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
			assert.NoError(t, err)

			fcw.DynamicClient.PrependReactor("delete", "virtualmachineservices", testCase.deleteFunc)

			err = lb.EnsureLoadBalancerDeleted(context.Background(), "test", testK8sService)
			if err != nil {
				assert.Equal(t, err.Error(), testCase.expectErr)
			}
		})
	}
}
