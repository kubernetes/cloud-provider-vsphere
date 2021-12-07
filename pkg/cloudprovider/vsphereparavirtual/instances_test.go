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
	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	testVMName     = types.NodeName("test-vm")
	testVMUUID     = "1bbf49a7-fbce-4502-bb4c-4c3544cacc9e"
	testProviderID = providerPrefix + testVMUUID
)

func createTestVM(name, namespace, biosUUID string) *vmopv1alpha1.VirtualMachine {
	return &vmopv1alpha1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1alpha1.VirtualMachineStatus{
			BiosUUID: biosUUID,
		},
	}
}

func createTestVMWithVMIPAndHost(name, namespace, biosUUID string) *vmopv1alpha1.VirtualMachine {
	return &vmopv1alpha1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1alpha1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			VmIp:     "1.2.3.4",
			Host:     "test-host",
		},
	}
}

func TestNewInstances(t *testing.T) {
	testCases := []struct {
		name        string
		testEnv     *envtest.Environment
		expectedErr error
	}{
		{
			name:        "NewInstance: when everything is ok",
			testEnv:     &envtest.Environment{},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg, err := testCase.testEnv.Start()
			assert.NoError(t, err)

			_, err = NewInstances(testClusterNameSpace, cfg)
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedErr, err)

			err = testCase.testEnv.Stop()
			assert.NoError(t, err)
		})
	}
}

func initTest(testVM *vmopv1alpha1.VirtualMachine) (*instances, *util.FakeClientWrapper) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := fakeClient.NewFakeClientWithScheme(scheme, testVM)
	fcw := util.NewFakeClientWrapper(fc)
	instance := &instances{
		vmClient:  fcw,
		namespace: testClusterNameSpace,
	}
	return instance, fcw
}

func TestInstanceID(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1alpha1.VirtualMachine
		expectInternalError bool
		expectedInstanceID  string
		expectedErr         error
	}{
		{
			name:               "test Instance interface: should not return error",
			testVM:             createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedInstanceID: testVMUUID,
			expectedErr:        nil,
		},
		{
			name:               "cannot find virtualmachine with node name",
			testVM:             createTestVM("bogus", testClusterNameSpace, testVMUUID),
			expectedInstanceID: "",
			expectedErr:        cloudprovider.InstanceNotFound,
		},
		{
			name:               "cannot find virtualmachine with namespace",
			testVM:             createTestVM(string(testVMName), "bogus", testVMUUID),
			expectedInstanceID: "",
			expectedErr:        cloudprovider.InstanceNotFound,
		},
		{
			name:               "cannot find virtualmachine with empty bios uuid",
			testVM:             createTestVM(string(testVMName), testClusterNameSpace, ""),
			expectedInstanceID: "",
			expectedErr:        errBiosUUIDEmpty,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, _ := initTest(testCase.testVM)
			instanceID, err := instance.InstanceID(context.Background(), testVMName)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedInstanceID, instanceID)
		})
	}
}

func TestInstanceIDThrowsErr(t *testing.T) {
	testCases := []struct {
		name               string
		testVM             *vmopv1alpha1.VirtualMachine
		expectedInstanceID string
	}{
		{
			name:               "test Instance interface: throws an error in client.Get()",
			testVM:             createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedInstanceID: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, fcw := initTest(testCase.testVM)
			fcw.GetFunc = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Internal error getting VMs")
			}

			instanceID, err := instance.InstanceID(context.Background(), testVMName)
			assert.NotEqual(t, nil, err)
			assert.NotEqual(t, cloudprovider.InstanceNotFound, err)
			assert.Equal(t, testCase.expectedInstanceID, instanceID)
		})
	}
}

func TestInstanceExistsByProviderID(t *testing.T) {
	testCases := []struct {
		name           string
		testVM         *vmopv1alpha1.VirtualMachine
		expectedResult bool
		expectedErr    error
	}{
		{
			name:           "InstanceExistsByProviderID should return true",
			testVM:         createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedResult: true,
			expectedErr:    nil,
		},
		{
			name:           "InstanceExistsByProviderID should return false",
			testVM:         createTestVM(string(testVMName), testClusterNameSpace, "bogus"),
			expectedResult: false,
			expectedErr:    nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, _ := initTest(testCase.testVM)
			providerID, err := instance.InstanceExistsByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedResult, providerID)
		})
	}
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	testCases := []struct {
		name             string
		testVM           *vmopv1alpha1.VirtualMachine
		testVMPowerState string
		expectedResult   bool
		expectedErr      error
	}{
		{
			name:             "InstanceShutdownByProviderID should return true for powered-off VM",
			testVM:           createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			testVMPowerState: "PoweredOff",
			expectedResult:   true,
			expectedErr:      nil,
		},
		{
			name:             "InstanceShutdownByProviderID should return false for powered-on VM",
			testVM:           createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			testVMPowerState: "PoweredOn",
			expectedResult:   false,
			expectedErr:      nil,
		},
		{
			name:             "InstanceShutdownByProviderID node not found for powered-on VM",
			testVM:           createTestVM(string(testVMName), testClusterNameSpace, "bogus"),
			testVMPowerState: "PoweredOn",
			expectedResult:   false,
			expectedErr:      cloudprovider.InstanceNotFound,
		},
		{
			name:             "InstanceShutdownByProviderID node not found for powered-off VM",
			testVM:           createTestVM(string(testVMName), testClusterNameSpace, "bogus"),
			testVMPowerState: "PoweredOff",
			expectedResult:   false,
			expectedErr:      cloudprovider.InstanceNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.testVMPowerState == "PoweredOn" {
				testCase.testVM.Status.PowerState = vmopv1alpha1.VirtualMachinePoweredOn
			} else {
				testCase.testVM.Status.PowerState = vmopv1alpha1.VirtualMachinePoweredOff
			}

			instance, _ := initTest(testCase.testVM)
			ret, err := instance.InstanceShutdownByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedResult, ret)
		})
	}
}

func TestNodeAddressesByProviderID(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1alpha1.VirtualMachine
		expectedNodeAddress []v1.NodeAddress
		expectedErr         error
	}{
		{
			name:                "NodeAddressesByProviderID returns an empty address for found node with no IP",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: []v1.NodeAddress{},
			expectedErr:         nil,
		},
		{
			name:                "NodeAddressesByProviderID returns a NotFound error for a not found node",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, "bogus"),
			expectedNodeAddress: nil,
			expectedErr:         cloudprovider.InstanceNotFound,
		},
		{
			name:   "NodeAddressesByProviderID returns a valid address for a found node",
			testVM: createTestVMWithVMIPAndHost(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, _ := initTest(testCase.testVM)
			ret, err := instance.NodeAddressesByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}

func TestNodeAddressesByProviderIDInternalErr(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1alpha1.VirtualMachine
		expectedNodeAddress []v1.NodeAddress
	}{
		{
			name:                "NodeAddressesByProviderID returns a general error for an internal error",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, fcw := initTest(testCase.testVM)
			fcw.ListFunc = func(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
				return fmt.Errorf("Internal error listing VMs")
			}

			ret, err := instance.NodeAddressesByProviderID(context.Background(), testProviderID)
			assert.NotEqual(t, nil, err)
			assert.NotEqual(t, cloudprovider.InstanceNotFound, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}

func TestNodeAddresses(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1alpha1.VirtualMachine
		expectedNodeAddress []v1.NodeAddress
		expectedErr         error
	}{
		{
			name:                "NodeAddresses returns an empty address for found node with no IP",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: []v1.NodeAddress{},
			expectedErr:         nil,
		},
		{
			name:                "NodeAddresses returns a NotFound error for a not found node",
			testVM:              createTestVM("bogus", testClusterNameSpace, testVMUUID),
			expectedNodeAddress: nil,
			expectedErr:         cloudprovider.InstanceNotFound,
		},
		{
			name:   "NodeAddresses returns a valid address for a found node",
			testVM: createTestVMWithVMIPAndHost(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, _ := initTest(testCase.testVM)
			ret, err := instance.NodeAddresses(context.Background(), testVMName)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}

func TestNodeAddressesInternalErr(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1alpha1.VirtualMachine
		expectedNodeAddress []v1.NodeAddress
	}{
		{
			name:                "NodeAddresses returns a general error for an internal error",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			expectedNodeAddress: nil,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			instance, fcw := initTest(testCase.testVM)
			fcw.GetFunc = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
				return fmt.Errorf("Internal error getting VMs")
			}

			ret, err := instance.NodeAddresses(context.Background(), testVMName)
			assert.NotEqual(t, nil, err)
			assert.NotEqual(t, cloudprovider.InstanceNotFound, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}
