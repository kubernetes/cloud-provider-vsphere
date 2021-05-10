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

package vmservice

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/api/node/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"

	"k8s.io/cloud-provider-vsphere/pkg/util"
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
	vms      VMService
	fakeLBIP = "1.1.1.1"

	// FakeClientWrapper allows functions to be replaced for fault injection
	fcw *util.FakeClientWrapper
)

func initTest() (*v1.Service, VMService, *util.FakeClientWrapper) {
	testK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
					NodePort: 30800,
				},
			},
		},
	}
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := fakeClient.NewFakeClientWithScheme(scheme)
	fcw = util.NewFakeClientWrapper(fc)
	vms = NewVMService(fcw, testClusterNameSpace, &testOwnerReference)

	return testK8sService, vms, fcw
}

func TestNewVMService(t *testing.T) {
	testCases := []struct {
		name    string
		testEnv *envtest.Environment
		err     error
	}{
		{
			name:    "NewVMService: when everything is ok",
			testEnv: &envtest.Environment{},
			err:     nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg, err := testCase.testEnv.Start()
			assert.NoError(t, err)

			client, err := GetVmopClient(cfg)
			assert.NoError(t, err)
			assert.NotEqual(t, client, nil)

			realVms := NewVMService(client, testClusterNameSpace, &testOwnerReference)
			assert.NotEqual(t, realVms, nil)

			err = testCase.testEnv.Stop()
			assert.NoError(t, err)
		})
	}
}

func TestGetVMServiceName(t *testing.T) {
	_, vms, _ := initTest()
	k8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}
	name := vms.GetVMServiceName(k8sService, testClustername)
	hashStr := vms.(*vmService).hashString(testK8sServiceName + "." + testK8sServiceNameSpace)
	expectedName := testClustername + "-" + hashStr[:MaxCheckSumLen]
	assert.Equal(t, name, expectedName)
}

func TestGetVMService_ReturnNil(t *testing.T) {
	_, vms, _ := initTest()
	k8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}
	vmService, err := vms.Get(context.Background(), k8sService, testClustername)
	assert.Equal(t, vmService, (*vmopv1alpha1.VirtualMachineService)(nil))
	assert.NoError(t, err)
}

func TestGetVMService(t *testing.T) {
	testK8sService, vms, _ := initTest()
	k8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
	}
	// create a fake VMService
	createdVMService, _ := vms.Create(context.Background(), k8sService, testClustername)

	vmService, err := vms.Get(context.Background(), k8sService, testClustername)
	assert.NoError(t, err)
	assert.Equal(t, (*vmService).Spec, (*createdVMService).Spec)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestCreateVMService(t *testing.T) {
	testK8sService, vms, _ := initTest()
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
	}

	vmServiceObj, err := vms.Create(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestCreateVMService_ZeroNodeport(t *testing.T) {
	_, vms, _ := initTest()
	k8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
				},
			},
		},
	}
	vmServiceObj, err := vms.Create(context.Background(), k8sService, testClustername)
	assert.Equal(t, vmServiceObj, (*vmopv1alpha1.VirtualMachineService)(nil))
	assert.Error(t, err)
}

func TestCreateDuplicateVMService(t *testing.T) {
	testK8sService, vms, _ := initTest()
	vmServiceObj, err := vms.Create(context.Background(), testK8sService, testClustername)
	assert.NotEqual(t, vmServiceObj, (*vmopv1alpha1.VirtualMachineService)(nil))
	assert.NoError(t, err)
	// Try to create the same object twice
	vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)
	assert.Equal(t, vmServiceObj, (*vmopv1alpha1.VirtualMachineService)(nil))
	assert.Error(t, err)
}

func TestCreateVMService_LBConfigs(t *testing.T) {
	_, vms, _ := initTest()
	testCases := []struct {
		name           string
		testK8sService *v1.Service
		expectedSpec   vmopv1alpha1.VirtualMachineServiceSpec
	}{
		{
			name: "when Service has loadBalancerIP defined",
			testK8sService: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testK8sServiceName,
					Namespace: testK8sServiceNameSpace,
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "http",
							Protocol: "tcp",
							Port:     80,
							TargetPort: intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 80,
							},
							NodePort: 30800,
						},
					},
					LoadBalancerIP: fakeLBIP,
				},
			},
			expectedSpec: vmopv1alpha1.VirtualMachineServiceSpec{
				Type: vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
				Selector: map[string]string{
					ClusterSelectorKey: testClustername,
					NodeSelectorKey:    NodeRole,
				},
				LoadBalancerIP: fakeLBIP,
			},
		},
		{
			name: "when Service has LoadBalancerSourceRanges defined",
			testK8sService: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testK8sServiceName,
					Namespace: testK8sServiceNameSpace,
				},
				Spec: v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "http",
							Protocol: "tcp",
							Port:     80,
							TargetPort: intstr.IntOrString{
								Type:   intstr.Int,
								IntVal: 80,
							},
							NodePort: 30800,
						},
					},
					LoadBalancerSourceRanges: []string{"1.1.1.0/24", "10.0.0.0/19"},
				},
			},
			expectedSpec: vmopv1alpha1.VirtualMachineServiceSpec{
				Type: vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
				Selector: map[string]string{
					ClusterSelectorKey: testClustername,
					NodeSelectorKey:    NodeRole,
				},
				LoadBalancerSourceRanges: []string{"1.1.1.0/24", "10.0.0.0/19"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ports, _ := findPorts(testCase.testK8sService)
			testCase.expectedSpec.Ports = ports
			vmServiceObj, err := vms.Create(context.Background(), testCase.testK8sService, testClustername)
			assert.NoError(t, err)
			assert.Equal(t, (*vmServiceObj).Spec, testCase.expectedSpec)

			testCase.testK8sService.Spec.LoadBalancerIP = ""
			testCase.testK8sService.Spec.LoadBalancerSourceRanges = []string{}
			err = vms.Delete(context.Background(), testCase.testK8sService, testClustername)
			assert.NoError(t, err)
		})
	}
}

func TestCreateVMService_ExternalTrafficPolicyTypeLocal(t *testing.T) {
	testK8sService, vms, _ := initTest()
	testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	testK8sService.Spec.HealthCheckNodePort = 30012
	vmServiceObj, err := vms.Create(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)

	v, ok := vmServiceObj.Annotations[AnnotationServiceExternalTrafficPolicyKey]
	assert.Equal(t, ok, true)
	assert.Equal(t, v, string(v1.ServiceExternalTrafficPolicyTypeLocal))

	hcPort, ok := vmServiceObj.Annotations[AnnotationServiceHealthCheckNodePortKey]
	assert.Equal(t, ok, true)
	assert.Equal(t, hcPort, strconv.Itoa(30012))

	testK8sService.Spec.ExternalTrafficPolicy = ""
	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestCreateVMService_ExternalTrafficPolicyTypeCluster(t *testing.T) {
	testK8sService, vms, _ := initTest()
	testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
	vmServiceObj, err := vms.Create(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)

	_, ok := vmServiceObj.Annotations[AnnotationServiceExternalTrafficPolicyKey]
	assert.NotEqual(t, ok, true)

	_, ok = vmServiceObj.Annotations[AnnotationServiceHealthCheckNodePortKey]
	assert.NotEqual(t, ok, true)

	testK8sService.Spec.ExternalTrafficPolicy = ""
	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestCreateOrUpdateVMService(t *testing.T) {
	testK8sService, vms, _ := initTest()
	testCases := []struct {
		name        string
		k8sService  *v1.Service
		clustername string
		expectedErr string
	}{
		{
			name: "when VMService does not exist",
			k8sService: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testK8sServiceName,
					Namespace: testK8sServiceNameSpace,
				},
			},
			clustername: testClustername,
			expectedErr: ErrVMServiceIPNotFound.Error(),
		},
		{
			name:        "when clusterName is empty",
			k8sService:  testK8sService,
			clustername: "",
			expectedErr: "cluster name cannot be empty: " + ErrCreateVMService.Error(),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := vms.CreateOrUpdate(context.Background(), testCase.k8sService, testCase.clustername)
			assert.Error(t, err)
			assert.Equal(t, err.Error(), testCase.expectedErr)
		})
	}
}

func TestCreateOrUpdateVMService_RedefineGetFunc(t *testing.T) {
	testCases := []struct {
		name        string
		getFunc     func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error
		expectedErr error
	}{
		{
			name: "failed to create VirtualMachineService",
			getFunc: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("failed to get VirtualMachineService")
			},
			expectedErr: ErrGetVMService,
		},
		{
			name: "when VMService does not exist",
			getFunc: func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return apierrors.NewNotFound(v1alpha1.Resource("virtualmachineservice"), testClustername)
			},
			expectedErr: ErrVMServiceIPNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testK8sService, vms, fcw := initTest()
			// Redefine Get in the client to return an error
			fcw.GetFunc = testCase.getFunc
			_, err := vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)
			assert.Equal(t, testCase.expectedErr.Error(), err.Error())
		})
	}
}

func TestCreateOrUpdateVMService_RedefineCreateFunc(t *testing.T) {
	testK8sService, vms, fcw := initTest()
	// Redefine Create in the client to return an error
	fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
		return fmt.Errorf("failed to create VirtualMachineService")
	}
	_, err := vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)
	assert.Equal(t, ErrCreateVMService.Error(), err.Error())
}

func TestVMService_AlreadyExists(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testK8sServiceName,
			Namespace: testK8sServiceNameSpace,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:     "http",
					Protocol: "tcp",
					Port:     80,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 80,
					},
					NodePort: 30500,
				},
			},
		},
	}

	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
	}
	// create an old VMService
	_, _ = vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)
	assert.Equal(t, err, ErrVMServiceIPNotFound)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_NoChange(t *testing.T) {
	testK8sService, vms, _ := initTest()
	createdVMService, _ := vms.Create(context.Background(), testK8sService, testClustername)
	_, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_NodePortChanges(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	oldK8sService.Spec.Ports[0].NodePort = 30500
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_LBIPAdded(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.LoadBalancerIP = fakeLBIP
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
		LoadBalancerIP: fakeLBIP,
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_LBIPChanges(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.LoadBalancerIP = fakeLBIP
	oldK8sService.Spec.LoadBalancerIP = "2.2.2.2"
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
		LoadBalancerIP: fakeLBIP,
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_LBSourceRangesAdded(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.LoadBalancerSourceRanges = []string{"1.1.1.0/24"}
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
		LoadBalancerSourceRanges: []string{"1.1.1.0/24"},
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_LBSourceRangesChanges(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.LoadBalancerSourceRanges = []string{"1.1.1.0/24"}
	oldK8sService.Spec.LoadBalancerSourceRanges = []string{"2.2.2.0/24"}
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
		LoadBalancerSourceRanges: []string{"1.1.1.0/24"},
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.NoError(t, err)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_ExternalTrafficPolicyLocal(t *testing.T) {
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	testK8sService.Spec.HealthCheckNodePort = 31234
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
	}
	expectedAnnotations := map[string]string{
		AnnotationServiceExternalTrafficPolicyKey: string(v1.ServiceExternalTrafficPolicyTypeLocal),
		AnnotationServiceHealthCheckNodePortKey:   "31234",
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.NoError(t, err)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.Equal(t, (*vmServiceObj).Annotations, expectedAnnotations)

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestUpdateVMService_ExternalTrafficPolicyCluster(t *testing.T) {
	// test when external traffic policy is set to cluster from local
	testK8sService, vms, _ := initTest()
	oldK8sService := testK8sService.DeepCopy()
	testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
	testK8sService.Spec.HealthCheckNodePort = 31234
	oldK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
	ports, _ := findPorts(testK8sService)
	expectedSpec := vmopv1alpha1.VirtualMachineServiceSpec{
		Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
		Ports: ports,
		Selector: map[string]string{
			ClusterSelectorKey: testClustername,
			NodeSelectorKey:    NodeRole,
		},
	}
	// create an old VMService
	createdVMService, _ := vms.Create(context.Background(), oldK8sService, testClustername)

	vmServiceObj, err := vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
	assert.NoError(t, err)
	assert.Equal(t, (*vmServiceObj).Spec, expectedSpec)
	assert.Equal(t, (*vmServiceObj).Annotations, map[string]string(nil))

	err = vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}

func TestDeleteVMService(t *testing.T) {
	testK8sService, vms, _ := initTest()
	_, _ = vms.Create(context.Background(), testK8sService, testClustername)
	err := vms.Delete(context.Background(), testK8sService, testClustername)
	assert.NoError(t, err)
}
