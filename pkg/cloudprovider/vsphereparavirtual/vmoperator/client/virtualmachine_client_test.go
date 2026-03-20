/*
Copyright 2026 The Kubernetes Authors.

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

package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
)

const testNS = "test-ns"

func initVMTest() (*Client, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	return NewFakeClient(fc), fc
}

// seedVM seeds a VirtualMachine directly into the fake dynamic client,
// bypassing the client layer. This mirrors the approach used in the v1alpha5
// tests and avoids relying on write methods that do not exist on the read-only
// VirtualMachine client.
func seedVM(t *testing.T, fc *dynamicfake.FakeDynamicClient, vm *vmopv1.VirtualMachine) {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	assert.NoError(t, err)
	obj["apiVersion"] = VirtualMachineGVR.Group + "/" + VirtualMachineGVR.Version
	obj["kind"] = "VirtualMachine"
	_, err = fc.Resource(VirtualMachineGVR).Namespace(vm.Namespace).Create(
		context.Background(), &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	assert.NoError(t, err)
}

func TestVMGet(t *testing.T) {
	testCases := []struct {
		name           string
		virtualMachine *vmopv1.VirtualMachine
		getFunc        func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM     *vmopv1.VirtualMachine
		expectedErr    bool
	}{
		{
			name: "Get: when everything is ok",
			virtualMachine: &vmopv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm",
					Namespace: testNS,
				},
				Spec: vmopv1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			expectedVM: &vmopv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
		},
		{
			name: "Get: when get error",
			virtualMachine: &vmopv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-vm-error",
					Namespace: testNS,
				},
				Spec: vmopv1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			getFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVM:  nil,
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMTest()
			seedVM(t, fc, testCase.virtualMachine)
			if testCase.getFunc != nil {
				fc.PrependReactor("get", "*", testCase.getFunc)
			}
			actualVM, err := c.GetVirtualMachine(context.Background(), testCase.virtualMachine.Namespace, testCase.virtualMachine.Name)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVM.Spec, actualVM.Spec)
			}
		})
	}
}

// TestVMListSetsResourceVersion verifies that ListVirtualMachines always sets
// ResourceVersion="0" on the list request so the API server serves the response
// from its watch cache rather than performing a quorum read from etcd.
//
// The client-go fake's ListAction does not expose ResourceVersion (it is
// discarded by ExtractFromListOptions before being stored in ListActionImpl).
// We therefore use a spy dynamic.Interface that captures the metav1.ListOptions
// passed to List, allowing direct assertion of the ResourceVersion field.
func TestVMListSetsResourceVersion(t *testing.T) {
	testCases := []struct {
		name       string
		inputRV    string
		expectedRV string
	}{
		{
			name:       "sets ResourceVersion=0 when caller passes empty string",
			inputRV:    "",
			expectedRV: "0",
		},
		{
			name:       "preserves caller-supplied ResourceVersion when non-empty",
			inputRV:    "42",
			expectedRV: "42",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &listOptsSpy{}
			c := NewWithDynamicClient(spy)
			_, _ = c.ListVirtualMachines(context.Background(), testNS, metav1.ListOptions{ResourceVersion: tc.inputRV})
			assert.Equal(t, tc.expectedRV, spy.capturedRV,
				"ListVirtualMachines must send ResourceVersion=%q to the API server", tc.expectedRV)
		})
	}
}

func TestVMList(t *testing.T) {
	testCases := []struct {
		name               string
		virtualMachineList *vmopv1.VirtualMachineList
		listFunc           func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMNum      int
		expectedErr        bool
	}{
		{
			name: "List: when there is one virtual machine, list should be ok",
			virtualMachineList: &vmopv1.VirtualMachineList{
				Items: []vmopv1.VirtualMachine{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: testNS},
						Spec:       vmopv1.VirtualMachineSpec{ImageName: "test-image"},
					},
				},
			},
			expectedVMNum: 1,
		},
		{
			name: "List: when there is 2 virtual machines, list should be ok",
			virtualMachineList: &vmopv1.VirtualMachineList{
				Items: []vmopv1.VirtualMachine{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: testNS},
						Spec:       vmopv1.VirtualMachineSpec{ImageName: "test-image"},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-vm-2", Namespace: testNS},
						Spec:       vmopv1.VirtualMachineSpec{ImageName: "test-image"},
					},
				},
			},
			expectedVMNum: 2,
		},
		{
			name: "List: when there is 0 virtual machine, list should be ok",
			virtualMachineList: &vmopv1.VirtualMachineList{
				Items: []vmopv1.VirtualMachine{},
			},
			expectedVMNum: 0,
		},
		{
			name: "List: when list error",
			virtualMachineList: &vmopv1.VirtualMachineList{
				Items: []vmopv1.VirtualMachine{
					{
						ObjectMeta: metav1.ObjectMeta{Name: "test-vm", Namespace: testNS},
						Spec:       vmopv1.VirtualMachineSpec{ImageName: "test-image"},
					},
				},
			},
			listFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVMNum: 0,
			expectedErr:   true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMTest()
			for i := range testCase.virtualMachineList.Items {
				seedVM(t, fc, &testCase.virtualMachineList.Items[i])
			}
			if testCase.listFunc != nil {
				fc.PrependReactor("list", "*", testCase.listFunc)
			}
			vmList, err := c.ListVirtualMachines(context.Background(), testNS, metav1.ListOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMNum, len(vmList.Items))
			}
		})
	}
}
