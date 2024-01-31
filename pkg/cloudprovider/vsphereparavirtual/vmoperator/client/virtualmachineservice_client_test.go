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

func initVMServiceTest() (*virtualMachineServices, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1alpha1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	vms := newVirtualMachineServices(NewFakeClient(fc), "test-ns")
	return vms, fc
}

func TestVMServiceCreate(t *testing.T) {
	vms, fc := initVMServiceTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachineService
		createFunc     func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM     *vmopv1alpha1.VirtualMachineService
		expectedErr    bool
	}{
		{
			name: "Create: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Create: when create error",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
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

func TestVMServiceUpdate(t *testing.T) {
	vms, fc := initVMServiceTest()
	testCases := []struct {
		name        string
		oldVM       *vmopv1alpha1.VirtualMachineService
		newVM       *vmopv1alpha1.VirtualMachineService
		updateFunc  func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM  *vmopv1alpha1.VirtualMachineService
		expectedErr bool
	}{
		{
			name: "Update: when everything is ok",
			oldVM: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			newVM: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Update: when update error",
			oldVM: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			newVM: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
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

func TestVMServiceDelete(t *testing.T) {
	vms, fc := initVMServiceTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachineService
		deleteFunc     func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedErr    bool
	}{
		{
			name: "Delete: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Create: when delete error",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
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

func TestVMServiceGet(t *testing.T) {
	vms, fc := initVMServiceTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachineService
		getFunc        func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVM     *vmopv1alpha1.VirtualMachineService
		expectedErr    bool
	}{
		{
			name: "Get: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVM: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Get: when get error",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm-error",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
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

func TestVMServiceList(t *testing.T) {
	vms, fc := initVMServiceTest()
	testCases := []struct {
		name           string
		virtualMachine *vmopv1alpha1.VirtualMachineService
		listFunc       func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMNum  int
		expectedErr    bool
	}{
		{
			name: "List: when everything is ok",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVMNum: 1,
		},
		{
			name: "List: when list error",
			virtualMachine: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm-error",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
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
