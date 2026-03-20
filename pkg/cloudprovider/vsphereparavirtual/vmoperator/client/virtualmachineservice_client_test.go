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

func initVMServiceTest() (*Client, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	return NewFakeClient(fc), fc
}

// TestVMServiceCreateSetsGVK verifies that CreateVirtualMachineService sets apiVersion and kind
// on the object sent to the API server. Without this, the dynamic client sends a
// request missing TypeMeta and the API server returns "Object 'Kind' is missing".
func TestVMServiceCreateSetsGVK(t *testing.T) {
	c, fc := initVMServiceTest()
	var capturedAPIVersion, capturedKind string
	fc.PrependReactor("create", "virtualmachineservices", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.CreateAction).GetObject().(*unstructured.Unstructured)
		capturedAPIVersion = obj.GetAPIVersion()
		capturedKind = obj.GetKind()
		return false, nil, nil
	})
	_, err := c.CreateVirtualMachineService(context.Background(), &vmopv1.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
	})
	assert.NoError(t, err)
	assert.Equal(t, VirtualMachineServiceGVR.Group+"/"+VirtualMachineServiceGVR.Version, capturedAPIVersion,
		"CreateVirtualMachineService must set apiVersion on the unstructured object")
	assert.Equal(t, "VirtualMachineService", capturedKind,
		"CreateVirtualMachineService must set kind on the unstructured object")
}

// TestVMServiceUpdateSetsGVK verifies that UpdateVirtualMachineService sets apiVersion and kind.
func TestVMServiceUpdateSetsGVK(t *testing.T) {
	c, fc := initVMServiceTest()
	_, err := c.CreateVirtualMachineService(context.Background(), &vmopv1.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
	})
	assert.NoError(t, err)

	var capturedAPIVersion, capturedKind string
	fc.PrependReactor("update", "virtualmachineservices", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		obj := action.(clientgotesting.UpdateAction).GetObject().(*unstructured.Unstructured)
		capturedAPIVersion = obj.GetAPIVersion()
		capturedKind = obj.GetKind()
		return false, nil, nil
	})
	_, err = c.UpdateVirtualMachineService(context.Background(), &vmopv1.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
	})
	assert.NoError(t, err)
	assert.Equal(t, VirtualMachineServiceGVR.Group+"/"+VirtualMachineServiceGVR.Version, capturedAPIVersion,
		"UpdateVirtualMachineService must set apiVersion on the unstructured object")
	assert.Equal(t, "VirtualMachineService", capturedKind,
		"UpdateVirtualMachineService must set kind on the unstructured object")
}

func TestVMServiceCreate(t *testing.T) {
	testCases := []struct {
		name                  string
		virtualMachineService *vmopv1.VirtualMachineService
		createFunc            func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService     *vmopv1.VirtualMachineService
		expectedErr           bool
	}{
		{
			name: "Create: when everything is ok",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: vmopv1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVMService: &vmopv1.VirtualMachineService{
				Spec: vmopv1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Create: when create error",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec: vmopv1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			createFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVMService: nil,
			expectedErr:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMServiceTest()
			if testCase.createFunc != nil {
				fc.PrependReactor("create", "*", testCase.createFunc)
			}
			actualVMS, err := c.CreateVirtualMachineService(context.Background(), testCase.virtualMachineService)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, actualVMS.Spec)
			}
		})
	}
}

func TestVMServiceUpdate(t *testing.T) {
	testCases := []struct {
		name              string
		oldVMService      *vmopv1.VirtualMachineService
		newVMService      *vmopv1.VirtualMachineService
		updateFunc        func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService *vmopv1.VirtualMachineService
		expectedErr       bool
	}{
		{
			name: "Update: when everything is ok",
			oldVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			newVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			expectedVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms"},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
		},
		{
			name: "Update: when update error",
			oldVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			newVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			updateFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVMService: nil,
			expectedErr:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMServiceTest()
			_, err := c.CreateVirtualMachineService(context.Background(), testCase.oldVMService)
			assert.NoError(t, err)
			if testCase.updateFunc != nil {
				fc.PrependReactor("update", "*", testCase.updateFunc)
			}
			updatedVMS, err := c.UpdateVirtualMachineService(context.Background(), testCase.newVMService)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, updatedVMS.Spec)
			}
		})
	}
}

func TestVMServiceDelete(t *testing.T) {
	testCases := []struct {
		name                  string
		virtualMachineService *vmopv1.VirtualMachineService
		deleteFunc            func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedErr           bool
	}{
		{
			name: "Delete: when everything is ok",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
		},
		{
			name: "Delete: when delete error",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			deleteFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMServiceTest()
			_, err := c.CreateVirtualMachineService(context.Background(), testCase.virtualMachineService)
			assert.NoError(t, err)
			if testCase.deleteFunc != nil {
				fc.PrependReactor("delete", "*", testCase.deleteFunc)
			}
			err = c.DeleteVirtualMachineService(context.Background(), testCase.virtualMachineService.Namespace, testCase.virtualMachineService.Name)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVMServiceGet(t *testing.T) {
	testCases := []struct {
		name                  string
		virtualMachineService *vmopv1.VirtualMachineService
		getFunc               func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService     *vmopv1.VirtualMachineService
		expectedErr           bool
	}{
		{
			name: "Get: when everything is ok",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			expectedVMService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms"},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
		},
		{
			name: "Get: when get error",
			virtualMachineService: &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: "test-vms-error", Namespace: testNS},
				Spec:       vmopv1.VirtualMachineServiceSpec{Type: "NodePort"},
			},
			getFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVMService: nil,
			expectedErr:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMServiceTest()
			_, err := c.CreateVirtualMachineService(context.Background(), testCase.virtualMachineService)
			assert.NoError(t, err)
			if testCase.getFunc != nil {
				fc.PrependReactor("get", "*", testCase.getFunc)
			}
			actualVMS, err := c.GetVirtualMachineService(context.Background(), testCase.virtualMachineService.Namespace, testCase.virtualMachineService.Name)
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, actualVMS.Spec)
			}
		})
	}
}

// TestVMServiceListSetsResourceVersion verifies that ListVirtualMachineServices
// always sets ResourceVersion="0" on the list request so the API server serves
// the response from its watch cache rather than performing a quorum read from etcd.
//
// Uses the listOptsSpy (defined in fake_client_test.go) to capture the
// metav1.ListOptions forwarded to the dynamic client, since the client-go fake's
// ListAction does not expose ResourceVersion.
func TestVMServiceListSetsResourceVersion(t *testing.T) {
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
			_, _ = c.ListVirtualMachineServices(context.Background(), testNS, metav1.ListOptions{ResourceVersion: tc.inputRV})
			assert.Equal(t, tc.expectedRV, spy.capturedRV,
				"ListVirtualMachineServices must send ResourceVersion=%q to the API server", tc.expectedRV)
		})
	}
}

func TestVMServiceList(t *testing.T) {
	testCases := []struct {
		name                      string
		virtualMachineServiceList *vmopv1.VirtualMachineServiceList
		listFunc                  func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMServiceNum      int
		expectedErr               bool
	}{
		{
			name: "List: when there is one virtual machine service, list should be ok",
			virtualMachineServiceList: &vmopv1.VirtualMachineServiceList{
				Items: []vmopv1.VirtualMachineService{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS}, Spec: vmopv1.VirtualMachineServiceSpec{Type: "NodePort"}},
				},
			},
			expectedVMServiceNum: 1,
		},
		{
			name: "List: when there is 2 virtual machine services, list should be ok",
			virtualMachineServiceList: &vmopv1.VirtualMachineServiceList{
				Items: []vmopv1.VirtualMachineService{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS}, Spec: vmopv1.VirtualMachineServiceSpec{Type: "NodePort"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "test-vms-2", Namespace: testNS}, Spec: vmopv1.VirtualMachineServiceSpec{Type: "NodePort"}},
				},
			},
			expectedVMServiceNum: 2,
		},
		{
			name: "List: when there is 0 virtual machine service, list should be ok",
			virtualMachineServiceList: &vmopv1.VirtualMachineServiceList{
				Items: []vmopv1.VirtualMachineService{},
			},
			expectedVMServiceNum: 0,
		},
		{
			name: "List: when list error",
			virtualMachineServiceList: &vmopv1.VirtualMachineServiceList{
				Items: []vmopv1.VirtualMachineService{
					{ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS}, Spec: vmopv1.VirtualMachineServiceSpec{Type: "NodePort"}},
				},
			},
			listFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVMServiceNum: 0,
			expectedErr:          true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			c, fc := initVMServiceTest()
			for i := range testCase.virtualMachineServiceList.Items {
				_, err := c.CreateVirtualMachineService(context.Background(), &testCase.virtualMachineServiceList.Items[i])
				assert.NoError(t, err)
			}
			if testCase.listFunc != nil {
				fc.PrependReactor("list", "*", testCase.listFunc)
			}
			vmsList, err := c.ListVirtualMachineServices(context.Background(), testNS, metav1.ListOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMServiceNum, len(vmsList.Items))
			}
		})
	}
}
