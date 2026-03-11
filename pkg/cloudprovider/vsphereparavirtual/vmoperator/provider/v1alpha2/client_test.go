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

package v1alpha2

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
)

const testNS = "test-ns"

func newTestClient() (*Client, *dynamicfake.FakeDynamicClient) {
	scheme := runtime.NewScheme()
	_ = vmopv1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	return NewWithDynamicClient(fc), fc
}

// seedVM seeds a VirtualMachine directly into the fake dynamic client.
// apiVersion and kind must be set explicitly to match what a real API server stores.
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

func TestGetVirtualMachine(t *testing.T) {
	testCases := []struct {
		name       string
		seedName   string
		queryName  string
		getFunc    func(clientgotesting.Action) (bool, runtime.Object, error)
		expectBios string
		expectErr  bool
	}{
		{
			name:       "returns VM when it exists",
			seedName:   "vm-1",
			queryName:  "vm-1",
			expectBios: "bios-1",
		},
		{
			name:      "returns error when VM does not exist",
			seedName:  "other-vm",
			queryName: "nonexistent",
			expectErr: true,
		},
		{
			name:      "returns error on API failure",
			seedName:  "vm-1",
			queryName: "vm-1",
			getFunc: func(clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("api error")
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, fc := newTestClient()
			seedVM(t, fc, &vmopv1.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: tc.seedName, Namespace: testNS},
				Status:     vmopv1.VirtualMachineStatus{BiosUUID: "bios-1"},
			})
			if tc.getFunc != nil {
				fc.PrependReactor("get", "virtualmachines", tc.getFunc)
			}

			vm, err := c.GetVirtualMachine(context.Background(), testNS, tc.queryName)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, vm)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expectBios, vm.Status.BiosUUID)
		})
	}
}

func TestListVirtualMachines(t *testing.T) {
	testCases := []struct {
		name        string
		seedVMs     []string
		listFunc    func(clientgotesting.Action) (bool, runtime.Object, error)
		expectedLen int
		expectErr   bool
	}{
		{
			name:        "returns empty list when no VMs exist",
			seedVMs:     nil,
			expectedLen: 0,
		},
		{
			name:        "returns all VMs in namespace",
			seedVMs:     []string{"vm-1", "vm-2", "vm-3"},
			expectedLen: 3,
		},
		{
			name:    "returns error on API failure",
			seedVMs: []string{"vm-1"},
			listFunc: func(clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("api error")
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, fc := newTestClient()
			for _, name := range tc.seedVMs {
				seedVM(t, fc, &vmopv1.VirtualMachine{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNS},
				})
			}
			if tc.listFunc != nil {
				fc.PrependReactor("list", "virtualmachines", tc.listFunc)
			}

			list, err := c.ListVirtualMachines(context.Background(), testNS, metav1.ListOptions{})
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			// Items must be a non-nil slice even when empty, so callers can safely
			// range over it without a nil check.
			assert.NotNil(t, list.Items)
			assert.Len(t, list.Items, tc.expectedLen)
		})
	}
}

// TestVMSCreateSetsGVK verifies that CreateVirtualMachineService sets apiVersion and kind
// on the object sent to the API server. Without this, the dynamic client sends a
// request missing TypeMeta and the API server returns "Object 'Kind' is missing".
func TestVMSCreateSetsGVK(t *testing.T) {
	c, fc := newTestClient()
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

// TestVMSUpdateSetsGVK verifies that UpdateVirtualMachineService sets apiVersion and kind.
func TestVMSUpdateSetsGVK(t *testing.T) {
	c, fc := newTestClient()
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

func TestVirtualMachineServiceCRUD(t *testing.T) {
	testCases := []struct {
		name       string
		vmsName    string
		createFunc func(clientgotesting.Action) (bool, runtime.Object, error)
		expectErr  bool
	}{
		{
			name:    "create, get, update, list, delete succeeds",
			vmsName: "test-vms",
		},
		{
			name:    "create returns error on API failure",
			vmsName: "test-vms",
			createFunc: func(clientgotesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("api error")
			},
			expectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c, fc := newTestClient()
			if tc.createFunc != nil {
				fc.PrependReactor("create", "virtualmachineservices", tc.createFunc)
			}

			vmService := &vmopv1.VirtualMachineService{
				ObjectMeta: metav1.ObjectMeta{Name: tc.vmsName, Namespace: testNS},
				Spec: vmopv1.VirtualMachineServiceSpec{
					Type: vmopv1.VirtualMachineServiceTypeLoadBalancer,
				},
			}

			created, err := c.CreateVirtualMachineService(context.Background(), vmService)
			if tc.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.vmsName, created.Name)

			// Get
			got, err := c.GetVirtualMachineService(context.Background(), testNS, tc.vmsName)
			assert.NoError(t, err)
			assert.Equal(t, tc.vmsName, got.Name)

			// Update
			got.Spec.LoadBalancerIP = "1.2.3.4"
			updated, err := c.UpdateVirtualMachineService(context.Background(), got)
			assert.NoError(t, err)
			assert.Equal(t, "1.2.3.4", updated.Spec.LoadBalancerIP)

			// List
			list, err := c.ListVirtualMachineServices(context.Background(), testNS, metav1.ListOptions{})
			assert.NoError(t, err)
			assert.NotNil(t, list.Items)
			assert.Len(t, list.Items, 1)

			// Delete
			err = c.DeleteVirtualMachineService(context.Background(), testNS, tc.vmsName)
			assert.NoError(t, err)
		})
	}
}

func TestListVirtualMachineServices_Empty(t *testing.T) {
	c, _ := newTestClient()
	list, err := c.ListVirtualMachineServices(context.Background(), testNS, metav1.ListOptions{})
	assert.NoError(t, err)
	// Items must be a non-nil slice even when empty, so callers can safely
	// range over it without a nil check.
	assert.NotNil(t, list.Items)
	assert.Empty(t, list.Items)
}

// listOptsSpy is a minimal dynamic.Interface spy that records the
// metav1.ListOptions passed to the most recent List call. It is used to verify
// that List methods set ResourceVersion="0" before forwarding the request to
// the API server, since the client-go fake's ListAction does not expose
// ResourceVersion (it is discarded by ExtractFromListOptions).
type listOptsSpy struct {
	tb         testing.TB
	capturedRV string
}

func (s *listOptsSpy) Resource(_ schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return &listOptsSpyResource{spy: s}
}

type listOptsSpyResource struct {
	spy *listOptsSpy
}

func (r *listOptsSpyResource) Namespace(_ string) dynamic.ResourceInterface {
	return r
}

func (r *listOptsSpyResource) List(_ context.Context, opts metav1.ListOptions) (*unstructured.UnstructuredList, error) {
	r.spy.capturedRV = opts.ResourceVersion
	return &unstructured.UnstructuredList{}, nil
}

func (r *listOptsSpyResource) Create(_ context.Context, _ *unstructured.Unstructured, _ metav1.CreateOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Create")
	return nil, nil
}
func (r *listOptsSpyResource) Update(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Update")
	return nil, nil
}
func (r *listOptsSpyResource) UpdateStatus(_ context.Context, _ *unstructured.Unstructured, _ metav1.UpdateOptions) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to UpdateStatus")
	return nil, nil
}
func (r *listOptsSpyResource) Delete(_ context.Context, _ string, _ metav1.DeleteOptions, _ ...string) error {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Delete")
	return nil
}
func (r *listOptsSpyResource) DeleteCollection(_ context.Context, _ metav1.DeleteOptions, _ metav1.ListOptions) error {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to DeleteCollection")
	return nil
}
func (r *listOptsSpyResource) Get(_ context.Context, _ string, _ metav1.GetOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Get")
	return nil, nil
}
func (r *listOptsSpyResource) Watch(_ context.Context, _ metav1.ListOptions) (watch.Interface, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Watch")
	return nil, nil
}
func (r *listOptsSpyResource) Patch(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Patch")
	return nil, nil
}
func (r *listOptsSpyResource) Apply(_ context.Context, _ string, _ *unstructured.Unstructured, _ metav1.ApplyOptions, _ ...string) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to Apply")
	return nil, nil
}
func (r *listOptsSpyResource) ApplyStatus(_ context.Context, _ string, _ *unstructured.Unstructured, _ metav1.ApplyOptions) (*unstructured.Unstructured, error) {
	r.spy.tb.Fatal("listOptsSpy: unexpected call to ApplyStatus")
	return nil, nil
}

// TestListVirtualMachines_SetsResourceVersion verifies that ListVirtualMachines
// always sets ResourceVersion="0" on the list request so the API server serves
// the response from its watch cache rather than performing a quorum read from etcd.
func TestListVirtualMachines_SetsResourceVersion(t *testing.T) {
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
			spy := &listOptsSpy{tb: t}
			c := NewWithDynamicClient(spy)
			_, _ = c.ListVirtualMachines(context.Background(), testNS, metav1.ListOptions{ResourceVersion: tc.inputRV})
			assert.Equal(t, tc.expectedRV, spy.capturedRV,
				"ListVirtualMachines must send ResourceVersion=%q to the API server", tc.expectedRV)
		})
	}
}

// TestListVirtualMachineServices_SetsResourceVersion verifies that
// ListVirtualMachineServices always sets ResourceVersion="0" on the list request.
func TestListVirtualMachineServices_SetsResourceVersion(t *testing.T) {
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
			spy := &listOptsSpy{tb: t}
			c := NewWithDynamicClient(spy)
			_, _ = c.ListVirtualMachineServices(context.Background(), testNS, metav1.ListOptions{ResourceVersion: tc.inputRV})
			assert.Equal(t, tc.expectedRV, spy.capturedRV,
				"ListVirtualMachineServices must send ResourceVersion=%q to the API server", tc.expectedRV)
		})
	}
}
