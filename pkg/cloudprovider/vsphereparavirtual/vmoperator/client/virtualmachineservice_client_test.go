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
	vms := newVirtualMachineServices(NewFakeClientSet(fc).V1alpha1(), "test-ns")
	return vms, fc
}

func TestVMServiceCreate(t *testing.T) {
	testCases := []struct {
		name                  string
		virtualMachineService *vmopv1alpha1.VirtualMachineService
		createFunc            func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService     *vmopv1alpha1.VirtualMachineService
		expectedErr           bool
	}{
		{
			name: "Create: when everything is ok",
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVMService: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Create: when create error",
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
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
			vms, fc := initVMServiceTest()
			if testCase.createFunc != nil {
				fc.PrependReactor("create", "*", testCase.createFunc)
			}
			actualVM, err := vms.Create(context.Background(), testCase.virtualMachineService, metav1.CreateOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, actualVM.Spec)
			}
		})
	}
}

func TestVMServiceUpdate(t *testing.T) {
	testCases := []struct {
		name              string
		oldVMService      *vmopv1alpha1.VirtualMachineService
		newVMService      *vmopv1alpha1.VirtualMachineService
		updateFunc        func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService *vmopv1alpha1.VirtualMachineService
		expectedErr       bool
	}{
		{
			name: "Update: when everything is ok",
			oldVMService: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			newVMService: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVMService: &vmopv1alpha1.VirtualMachineService{
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
			oldVMService: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			newVMService: &vmopv1alpha1.VirtualMachineService{
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
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
			vms, fc := initVMServiceTest()
			_, err := vms.Create(context.Background(), testCase.oldVMService, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.updateFunc != nil {
				fc.PrependReactor("update", "*", testCase.updateFunc)
			}
			updatedVM, err := vms.Update(context.Background(), testCase.newVMService, metav1.UpdateOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, updatedVM.Spec)
			}
		})
	}
}

func TestVMServiceDelete(t *testing.T) {
	testCases := []struct {
		name                  string
		virtualMachineService *vmopv1alpha1.VirtualMachineService
		deleteFunc            func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedErr           bool
	}{
		{
			name: "Delete: when everything is ok",
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
		},
		{
			name: "Delete: when delete error",
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
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
			vms, fc := initVMServiceTest()
			_, err := vms.Create(context.Background(), testCase.virtualMachineService, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.deleteFunc != nil {
				fc.PrependReactor("delete", "*", testCase.deleteFunc)
			}
			err = vms.Delete(context.Background(), testCase.virtualMachineService.Name, metav1.DeleteOptions{})
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
		virtualMachineService *vmopv1alpha1.VirtualMachineService
		getFunc               func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMService     *vmopv1alpha1.VirtualMachineService
		expectedErr           bool
	}{
		{
			name: "Get: when everything is ok",
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-vm",
				},
				Spec: vmopv1alpha1.VirtualMachineServiceSpec{
					Type: "NodePort",
				},
			},
			expectedVMService: &vmopv1alpha1.VirtualMachineService{
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
			virtualMachineService: &vmopv1alpha1.VirtualMachineService{
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
			expectedVMService: nil,
			expectedErr:       true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			vms, fc := initVMServiceTest()
			_, err := vms.Create(context.Background(), testCase.virtualMachineService, metav1.CreateOptions{})
			assert.NoError(t, err)
			if testCase.getFunc != nil {
				fc.PrependReactor("get", "*", testCase.getFunc)
			}
			actualVM, err := vms.Get(context.Background(), testCase.virtualMachineService.Name, metav1.GetOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMService.Spec, actualVM.Spec)
			}
		})
	}
}

func TestVMServiceList(t *testing.T) {
	testCases := []struct {
		name                      string
		virtualMachineServiceList *vmopv1alpha1.VirtualMachineServiceList
		listFunc                  func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error)
		expectedVMServiceNum      int
		expectedErr               bool
	}{
		{
			name: "List: when there is one virtual machine service, list should be ok",
			virtualMachineServiceList: &vmopv1alpha1.VirtualMachineServiceList{
				Items: []vmopv1alpha1.VirtualMachineService{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-vm",
						},
						Spec: vmopv1alpha1.VirtualMachineServiceSpec{
							Type: "NodePort",
						},
					},
				},
			},
			expectedVMServiceNum: 1,
		},
		{
			name: "List: when there is 2 virtual machine services, list should be ok",
			virtualMachineServiceList: &vmopv1alpha1.VirtualMachineServiceList{
				Items: []vmopv1alpha1.VirtualMachineService{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-vm",
						},
						Spec: vmopv1alpha1.VirtualMachineServiceSpec{
							Type: "NodePort",
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-vm-2",
						},
						Spec: vmopv1alpha1.VirtualMachineServiceSpec{
							Type: "NodePort",
						},
					},
				},
			},
			expectedVMServiceNum: 2,
		},
		{
			name: "List: when there is 0 virtual machine service, list should be ok",
			virtualMachineServiceList: &vmopv1alpha1.VirtualMachineServiceList{
				Items: []vmopv1alpha1.VirtualMachineService{},
			},
			expectedVMServiceNum: 0,
		},
		{
			name: "List: when list error",
			virtualMachineServiceList: &vmopv1alpha1.VirtualMachineServiceList{
				Items: []vmopv1alpha1.VirtualMachineService{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-vm",
						},
						Spec: vmopv1alpha1.VirtualMachineServiceSpec{
							Type: "NodePort",
						},
					},
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
			vms, fc := initVMServiceTest()
			for _, vmservice := range testCase.virtualMachineServiceList.Items {
				_, err := vms.Create(context.Background(), &vmservice, metav1.CreateOptions{})
				assert.NoError(t, err)
				if testCase.listFunc != nil {
					fc.PrependReactor("list", "*", testCase.listFunc)
				}
			}

			vmServiceList, err := vms.List(context.Background(), metav1.ListOptions{})
			if testCase.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedVMServiceNum, len(vmServiceList.Items))
			}
		})
	}
}
