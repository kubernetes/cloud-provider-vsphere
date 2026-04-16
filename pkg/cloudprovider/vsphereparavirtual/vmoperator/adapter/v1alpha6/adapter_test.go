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

package v1alpha6_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vmopv6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"

	fakev6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha6/fake"
	clientv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha6"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

const testNS = "test-ns"

// seedVM seeds a VirtualMachine directly into the fake dynamic client.
// The CPI never creates VMs, so there is no client-layer write path to go through.
func seedVM(t *testing.T, fc *dynamicfake.FakeDynamicClient, vm *vmopv6.VirtualMachine) {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(vm)
	assert.NoError(t, err)
	obj["apiVersion"] = clientv6.VirtualMachineGVR.Group + "/" + clientv6.VirtualMachineGVR.Version
	obj["kind"] = "VirtualMachine"
	_, err = fc.Resource(clientv6.VirtualMachineGVR).Namespace(vm.Namespace).Create(
		context.Background(), &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	assert.NoError(t, err)
}

func TestAdapter_VirtualMachines_Get(t *testing.T) {
	testCases := []struct {
		name          string
		seedVM        *vmopv6.VirtualMachine
		queryName     string
		expectedBios  string
		expectedIP4   string
		expectedIP6   string
		expectedPower vmoptypes.PowerState
		expectedLabel map[string]string
		expectErr     bool
	}{
		{
			name: "returns full VM info when VM exists",
			seedVM: &vmopv6.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "vm-1",
					Namespace: testNS,
					Labels:    map[string]string{"zone": "us-east-1"},
				},
				Status: vmopv6.VirtualMachineStatus{
					BiosUUID:   "bios-uuid-1",
					PowerState: vmopv6.VirtualMachinePowerStateOn,
					Network: &vmopv6.VirtualMachineNetworkStatus{
						PrimaryIP4: "10.0.0.1",
						PrimaryIP6: "fd00::1",
					},
				},
			},
			queryName:     "vm-1",
			expectedBios:  "bios-uuid-1",
			expectedIP4:   "10.0.0.1",
			expectedIP6:   "fd00::1",
			expectedPower: vmoptypes.PowerStatePoweredOn,
			expectedLabel: map[string]string{"zone": "us-east-1"},
		},
		{
			name:      "returns error when VM does not exist",
			seedVM:    &vmopv6.VirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: "other-vm", Namespace: testNS}},
			queryName: "nonexistent",
			expectErr: true,
		},
		{
			name: "returns zero IP when Network status is nil",
			seedVM: &vmopv6.VirtualMachine{
				ObjectMeta: metav1.ObjectMeta{Name: "no-network-vm", Namespace: testNS},
				Status: vmopv6.VirtualMachineStatus{
					BiosUUID:   "bios-no-net",
					PowerState: vmopv6.VirtualMachinePowerStateOn,
					Network:    nil,
				},
			},
			queryName:     "no-network-vm",
			expectedBios:  "bios-no-net",
			expectedIP4:   "",
			expectedIP6:   "",
			expectedPower: vmoptypes.PowerStatePoweredOn,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, fc := fakev6.NewAdapter()
			seedVM(t, fc, tc.seedVM)

			info, err := adapter.VirtualMachines().Get(context.Background(), testNS, tc.queryName)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, info)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.queryName, info.Name)
			assert.Equal(t, testNS, info.Namespace)
			assert.Equal(t, tc.expectedBios, info.BiosUUID)
			assert.Equal(t, tc.expectedPower, info.PowerState)
			assert.Equal(t, tc.expectedIP4, info.PrimaryIP4)
			assert.Equal(t, tc.expectedIP6, info.PrimaryIP6)
			assert.Equal(t, tc.expectedLabel, info.Labels)
		})
	}
}

func TestAdapter_VirtualMachines_List(t *testing.T) {
	testCases := []struct {
		name        string
		seedVMs     []*vmopv6.VirtualMachine
		expectedLen int
	}{
		{
			name:        "returns empty list when no VMs exist",
			seedVMs:     nil,
			expectedLen: 0,
		},
		{
			name: "returns all VMs in namespace",
			seedVMs: []*vmopv6.VirtualMachine{
				{ObjectMeta: metav1.ObjectMeta{Name: "vm-1", Namespace: testNS}},
				{ObjectMeta: metav1.ObjectMeta{Name: "vm-2", Namespace: testNS}},
			},
			expectedLen: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, fc := fakev6.NewAdapter()
			for _, vm := range tc.seedVMs {
				seedVM(t, fc, vm)
			}

			list, err := adapter.VirtualMachines().List(context.Background(), testNS, vmoptypes.ListOptions{})
			assert.NoError(t, err)
			assert.Len(t, list, tc.expectedLen)
		})
	}
}

func TestAdapter_VirtualMachines_GetByBiosUUID(t *testing.T) {
	testCases := []struct {
		name         string
		seedVMs      []*vmopv6.VirtualMachine
		queryUUID    string
		expectedName string
		expectNil    bool
	}{
		{
			name: "returns VM info when BiosUUID matches",
			seedVMs: []*vmopv6.VirtualMachine{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vm-a", Namespace: testNS},
					Status:     vmopv6.VirtualMachineStatus{BiosUUID: "uuid-a"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vm-b", Namespace: testNS},
					Status:     vmopv6.VirtualMachineStatus{BiosUUID: "uuid-b"},
				},
			},
			queryUUID:    "uuid-b",
			expectedName: "vm-b",
		},
		{
			name: "returns nil when no VM matches BiosUUID",
			seedVMs: []*vmopv6.VirtualMachine{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vm-a", Namespace: testNS},
					Status:     vmopv6.VirtualMachineStatus{BiosUUID: "uuid-a"},
				},
			},
			queryUUID: "nonexistent-uuid",
			expectNil: true,
		},
		{
			// An empty biosUUID returns nil immediately without scanning, preventing
			// a false match against VMs whose BiosUUID has not yet been assigned.
			name: "returns nil for empty biosUUID without scanning",
			seedVMs: []*vmopv6.VirtualMachine{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "vm-no-bios", Namespace: testNS},
					Status:     vmopv6.VirtualMachineStatus{BiosUUID: ""},
				},
			},
			queryUUID: "",
			expectNil: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, fc := fakev6.NewAdapter()
			for _, vm := range tc.seedVMs {
				seedVM(t, fc, vm)
			}

			info, err := adapter.VirtualMachines().GetByBiosUUID(context.Background(), testNS, tc.queryUUID)
			assert.NoError(t, err)
			if tc.expectNil {
				assert.Nil(t, info)
				return
			}
			assert.NotNil(t, info)
			assert.Equal(t, tc.expectedName, info.Name)
			assert.Equal(t, tc.queryUUID, info.BiosUUID)
		})
	}
}

func TestAdapter_VirtualMachineServices_CRUD(t *testing.T) {
	testCases := []struct {
		name              string
		createInfo        *vmoptypes.VirtualMachineServiceInfo
		updatePorts       []vmoptypes.VirtualMachineServicePort
		updateLBIP        string
		updateAnnotations map[string]string
	}{
		{
			name: "full CRUD lifecycle succeeds",
			createInfo: &vmoptypes.VirtualMachineServiceInfo{
				Name:      "test-vms",
				Namespace: testNS,
				Spec: vmoptypes.VirtualMachineServiceSpec{
					Type: vmoptypes.VirtualMachineServiceTypeLoadBalancer,
					Ports: []vmoptypes.VirtualMachineServicePort{
						{Name: "http", Protocol: "TCP", Port: 80, TargetPort: 30800},
					},
				},
			},
			updatePorts:       []vmoptypes.VirtualMachineServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: 30443}},
			updateLBIP:        "1.2.3.4",
			updateAnnotations: map[string]string{"key": "value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, _ := fakev6.NewAdapter()

			// Create
			created, err := adapter.VirtualMachineServices().Create(context.Background(), tc.createInfo)
			assert.NoError(t, err)
			assert.Equal(t, tc.createInfo.Name, created.Name)
			assert.Len(t, created.Spec.Ports, len(tc.createInfo.Spec.Ports))

			// Get
			got, err := adapter.VirtualMachineServices().Get(context.Background(), testNS, tc.createInfo.Name)
			assert.NoError(t, err)
			assert.Equal(t, tc.createInfo.Name, got.Name)

			// Update - verify ports, LoadBalancerIP, and Annotations are all applied
			update := &vmoptypes.VirtualMachineServiceInfo{
				Annotations: tc.updateAnnotations,
				Spec: vmoptypes.VirtualMachineServiceSpec{
					Ports:          tc.updatePorts,
					LoadBalancerIP: tc.updateLBIP,
				},
			}
			updated, err := adapter.VirtualMachineServices().Update(context.Background(), testNS, tc.createInfo.Name, update)
			assert.NoError(t, err)
			assert.Len(t, updated.Spec.Ports, 1)
			assert.Equal(t, tc.updatePorts[0].Name, updated.Spec.Ports[0].Name)
			assert.Equal(t, tc.updateLBIP, updated.Spec.LoadBalancerIP)
			assert.Equal(t, tc.updateAnnotations, updated.Annotations)

			// List
			list, err := adapter.VirtualMachineServices().List(context.Background(), testNS, vmoptypes.ListOptions{})
			assert.NoError(t, err)
			assert.Len(t, list, 1)

			// Delete
			err = adapter.VirtualMachineServices().Delete(context.Background(), testNS, tc.createInfo.Name)
			assert.NoError(t, err)

			// Verify deleted
			got, err = adapter.VirtualMachineServices().Get(context.Background(), testNS, tc.createInfo.Name)
			assert.Error(t, err)
			assert.Nil(t, got)
		})
	}
}

// TestAdapter_VirtualMachineServices_Update_Conflict verifies that when the
// underlying Update call returns a 409 Conflict (ResourceVersion mismatch), the
// error is propagated to the caller unchanged. The caller is responsible for
// retrying on the next reconcile cycle.
//
// This test documents the current behaviour described in the TODO comment in
// adapter.Update: automatic retry on conflict is not yet implemented.
func TestAdapter_VirtualMachineServices_Update_Conflict(t *testing.T) {
	adapter, fc := fakev6.NewAdapter()

	// Seed the object via a second client wrapping the same fake.
	c := clientv6.NewWithDynamicClient(fc)
	_, err := c.CreateVirtualMachineService(context.Background(), &vmopv6.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{Name: "test-vms", Namespace: testNS},
		Spec:       vmopv6.VirtualMachineServiceSpec{Type: vmopv6.VirtualMachineServiceTypeLoadBalancer},
	})
	assert.NoError(t, err)

	// Simulate a 409 Conflict returned by the API server on the update call.
	conflictErr := apierrors.NewConflict(
		schema.GroupResource{Group: "vmoperator.vmware.com", Resource: "virtualmachineservices"},
		"test-vms",
		nil,
	)
	fc.PrependReactor("update", "virtualmachineservices", func(_ clientgotesting.Action) (bool, runtime.Object, error) {
		return true, nil, conflictErr
	})

	update := &vmoptypes.VirtualMachineServiceInfo{
		Spec: vmoptypes.VirtualMachineServiceSpec{
			Ports: []vmoptypes.VirtualMachineServicePort{
				{Name: "http", Protocol: "TCP", Port: 80, TargetPort: 30800},
			},
		},
	}
	result, err := adapter.VirtualMachineServices().Update(context.Background(), testNS, "test-vms", update)
	assert.Error(t, err, "Update must propagate 409 Conflict to the caller")
	assert.True(t, apierrors.IsConflict(err), "returned error must be a Conflict error")
	assert.Nil(t, result)
}

func TestAdapter_VirtualMachineServices_DualStackRoundTrip(t *testing.T) {
	adapter, _ := fakev6.NewAdapter()
	policy := corev1.IPFamilyPolicyPreferDualStack
	info := &vmoptypes.VirtualMachineServiceInfo{
		Name:      "dual-vms",
		Namespace: testNS,
		Spec: vmoptypes.VirtualMachineServiceSpec{
			Type: vmoptypes.VirtualMachineServiceTypeLoadBalancer,
			Ports: []vmoptypes.VirtualMachineServicePort{
				{Name: "http", Protocol: "TCP", Port: 80, TargetPort: 30800},
			},
			IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
			IPFamilyPolicy: &policy,
		},
	}
	created, err := adapter.VirtualMachineServices().Create(context.Background(), info)
	assert.NoError(t, err)
	assert.Equal(t, []corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol}, created.Spec.IPFamilies)
	assert.NotNil(t, created.Spec.IPFamilyPolicy)
	assert.Equal(t, policy, *created.Spec.IPFamilyPolicy)

	got, err := adapter.VirtualMachineServices().Get(context.Background(), testNS, "dual-vms")
	assert.NoError(t, err)
	assert.Equal(t, created.Spec.IPFamilies, got.Spec.IPFamilies)
	assert.Equal(t, *created.Spec.IPFamilyPolicy, *got.Spec.IPFamilyPolicy)
}
