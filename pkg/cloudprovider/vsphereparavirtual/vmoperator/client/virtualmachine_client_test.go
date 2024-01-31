package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func initVMTest() (*virtualMachines, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	vms := newVirtualMachines(NewFakeClient(fc), "test-ns")
	return vms, fc
}

func TestVMCreate(t *testing.T) {
	vms, fc := initVMTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachine
		createFunc     func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM     *vmopv1alpha1.VirtualMachine
		expectedErr    bool
	}{
		{
			name: "Create: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachine{
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
		},
		{
			name: "Create: when create error",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			createFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVM:  nil,
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.createFunc != nil {
				fc.PrependReactor("create", "*", testCase.createFunc)
			}
			actualVM, err := vms.Create(context.Background(), testCase.virtualMachine, metav1.CreateOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVM.Spec, actualVM.Spec)
			}
		})
	}
}

func TestVMUpdate(t *testing.T) {
	vms, fc := initVMTest()
	testCases := []struct {
		name        string
		oldVM       *vmopv1alpha1.VirtualMachine
		newVM       *vmopv1alpha1.VirtualMachine
		updateFunc  func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM  *vmopv1alpha1.VirtualMachine
		expectedErr bool
	}{
		{
			name: "Update: when everything is ok",
			oldVM: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			newVM: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image-2",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image-2",
				},
			},
		},
		{
			name: "Update: when update error",
			oldVM: &vmopv1alpha1.VirtualMachine{
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			newVM: &vmopv1alpha1.VirtualMachine{
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			updateFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedVM:  nil,
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := vms.Create(context.Background(), testCase.oldVM, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.updateFunc != nil {
				fc.PrependReactor("update", "*", testCase.updateFunc)
			}
			updatedVM, err := vms.Update(context.Background(), testCase.newVM, metav1.UpdateOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVM.Spec, updatedVM.Spec)
			}
		})
	}
}

func TestVMDelete(t *testing.T) {
	vms, fc := initVMTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachine
		deleteFunc     func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedErr    bool
	}{
		{
			name: "Delete: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
		},
		{
			name: "Create: when delete error",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			deleteFunc: func(action clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("test error")
			},
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := vms.Create(context.Background(), testCase.virtualMachine, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.deleteFunc != nil {
				fc.PrependReactor("delete", "*", testCase.deleteFunc)
			}
			err = vms.Delete(context.Background(), testCase.virtualMachine.Name, metav1.DeleteOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVMGet(t *testing.T) {
	vms, fc := initVMTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachine
		getFunc        func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM     *vmopv1alpha1.VirtualMachine
		expectedErr    bool
	}{
		{
			name: "Get: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
		},
		{
			name: "Get: when get error",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm-error",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
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
			_, err := vms.Create(context.Background(), testCase.virtualMachine, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.getFunc != nil {
				fc.PrependReactor("get", "*", testCase.getFunc)
			}
			actualVM, err := vms.Get(context.Background(), testCase.virtualMachine.Name, metav1.GetOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVM.Spec, actualVM.Spec)
			}
		})
	}
}

func TestVMList(t *testing.T) {
	vms, fc := initVMTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachine
		listFunc       func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMNum  int
		expectedErr    bool
	}{
		{
			name: "List: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
				},
			},
			expectedVMNum: 1,
		},
		{
			name: "List: when list error",
			virtualMachine: &vmopv1alpha1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm-error",
				},
				Spec: vmopv1alpha1.VirtualMachineSpec{
					ImageName: "test-image",
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
			_, err := vms.Create(context.Background(), testCase.virtualMachine, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.listFunc != nil {
				fc.PrependReactor("list", "*", testCase.listFunc)
			}
			vmList, err := vms.List(context.Background(), metav1.ListOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMNum, len(vmList.Items))
			}
		})
	}
}
