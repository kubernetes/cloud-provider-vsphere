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
	"github.com/stretchr/testify/require"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	cloudprovider "k8s.io/cloud-provider"

	adapterv2 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha2"
	fakev2 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha2/fake"
	clientv2 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha2"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

var (
	testVMName     = types.NodeName("test-vm")
	testVMUUID     = "1bbf49a7-fbce-4502-bb4c-4c3544cacc9e"
	testProviderID = providerPrefix + testVMUUID
)

func createTestVM(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
		},
	}
}

func createTestVMWithVMIPAndHost(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			Host:     "test-host",
			Network: &vmopv1.VirtualMachineNetworkStatus{
				PrimaryIP4: "1.2.3.4",
			},
		},
	}
}

func createTestVMWithIPv6(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			Host:     "test-host",
			Network: &vmopv1.VirtualMachineNetworkStatus{
				PrimaryIP6: "2001:db8::1",
			},
		},
	}
}

func createTestVMWithDualStack(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			Host:     "test-host",
			Network: &vmopv1.VirtualMachineNetworkStatus{
				PrimaryIP4: "1.2.3.4",
				PrimaryIP6: "2001:db8::1",
			},
		},
	}
}

func createTestVMWithMultipleIPs(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			Host:     "test-host",
			Network: &vmopv1.VirtualMachineNetworkStatus{
				PrimaryIP4: "1.2.3.4",
				PrimaryIP6: "2001:db8::1",
				Interfaces: []vmopv1.VirtualMachineNetworkInterfaceStatus{
					{
						IP: &vmopv1.VirtualMachineNetworkInterfaceIPStatus{
							Addresses: []vmopv1.VirtualMachineNetworkInterfaceIPAddrStatus{
								{Address: "1.2.3.4/24"},       // Primary IPv4 (will be deduplicated)
								{Address: "10.0.0.5/24"},      // Secondary IPv4
								{Address: "2001:db8::1/64"},   // Primary IPv6 (will be deduplicated)
								{Address: "2001:db8::100/64"}, // Secondary IPv6
							},
						},
					},
				},
			},
		},
	}
}

func createTestVMWithLinkLocalAddresses(name, namespace, biosUUID string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: vmopv1.VirtualMachineStatus{
			BiosUUID: biosUUID,
			Host:     "test-host",
			Network: &vmopv1.VirtualMachineNetworkStatus{
				PrimaryIP4: "1.2.3.4",
				Interfaces: []vmopv1.VirtualMachineNetworkInterfaceStatus{
					{
						IP: &vmopv1.VirtualMachineNetworkInterfaceIPStatus{
							Addresses: []vmopv1.VirtualMachineNetworkInterfaceIPAddrStatus{
								{Address: "1.2.3.4/24"},
								{Address: "fe80::1/64"},     // Link-local IPv6 (should be filtered)
								{Address: "169.254.1.1/16"}, // Link-local IPv4 (should be filtered)
								{Address: "10.0.0.5/24"},
							},
						},
					},
				},
			},
		},
	}
}

func TestNewInstances(t *testing.T) {
	fakeAdapter, _ := fakev2.NewAdapter()
	_, err := NewInstances(testClusterNameSpace, fakeAdapter, ClusterIPFamilyIPv4)
	require.NoError(t, err)
}

func initTest(testVM *vmopv1.VirtualMachine) (*instances, *dynamicfake.FakeDynamicClient, error) {
	scheme := runtime.NewScheme()
	_ = vmopv1.AddToScheme(scheme)
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	vmopAdapter := adapterv2.NewWithFakeClient(clientv2.NewWithDynamicClient(fc))
	instance := &instances{
		vmClient:        vmopAdapter,
		namespace:       testClusterNameSpace,
		clusterIPFamily: ClusterIPFamilyIPv4,
	}
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(testVM)
	if err != nil {
		return nil, nil, err
	}
	obj["apiVersion"] = clientv2.VirtualMachineGVR.Group + "/" + clientv2.VirtualMachineGVR.Version
	obj["kind"] = "VirtualMachine"
	_, err = fc.Resource(clientv2.VirtualMachineGVR).Namespace(testVM.Namespace).Create(
		context.TODO(), &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{})
	return instance, fc, err
}

func TestInstanceID(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1.VirtualMachine
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
			instance, _, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			instanceID, err := instance.InstanceID(context.Background(), testVMName)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedInstanceID, instanceID)
		})
	}
}

func TestInstanceIDThrowsErr(t *testing.T) {
	testCases := []struct {
		name               string
		testVM             *vmopv1.VirtualMachine
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
			instance, fc, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			fc.PrependReactor("get", "virtualmachines", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("Internal error getting VMs")
			})
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
		testVM         *vmopv1.VirtualMachine
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
			instance, _, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			providerID, err := instance.InstanceExistsByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedResult, providerID)
		})
	}
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	testCases := []struct {
		name             string
		testVM           *vmopv1.VirtualMachine
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
				testCase.testVM.Status.PowerState = vmopv1.VirtualMachinePowerStateOn
			} else {
				testCase.testVM.Status.PowerState = vmopv1.VirtualMachinePowerStateOff
			}

			instance, _, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			ret, err := instance.InstanceShutdownByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedResult, ret)
		})
	}
}

func TestNodeAddressesByProviderID(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1.VirtualMachine
		clusterIPFamily     string
		expectedNodeAddress []v1.NodeAddress
		expectedErr         error
	}{
		{
			name:                "NodeAddressesByProviderID returns an empty address for found node with no IP",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily:     ClusterIPFamilyIPv4,
			expectedNodeAddress: []v1.NodeAddress{},
			expectedErr:         nil,
		},
		{
			name:                "NodeAddressesByProviderID returns a NotFound error for a not found node",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, "bogus"),
			clusterIPFamily:     ClusterIPFamilyIPv4,
			expectedNodeAddress: nil,
			expectedErr:         cloudprovider.InstanceNotFound,
		},
		{
			name:            "NodeAddressesByProviderID returns a valid IPv4 address for a found node",
			testVM:          createTestVMWithVMIPAndHost(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4,
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
		{
			name:            "NodeAddressesByProviderID returns a valid IPv6 address for a found node",
			testVM:          createTestVMWithIPv6(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddressesByProviderID returns both IPv4 and IPv6 addresses for dual-stack node",
			testVM:          createTestVMWithDualStack(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddressesByProviderID returns all valid IPs including secondary addresses",
			testVM:          createTestVMWithMultipleIPs(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "10.0.0.5",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::100",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddressesByProviderID filters out link-local addresses",
			testVM:          createTestVMWithLinkLocalAddresses(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "10.0.0.5",
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
			instance, _, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			instance.clusterIPFamily = testCase.clusterIPFamily
			ret, err := instance.NodeAddressesByProviderID(context.Background(), testProviderID)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}

func TestNodeAddressesByProviderIDInternalErr(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1.VirtualMachine
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
			instance, fc, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			fc.PrependReactor("list", "virtualmachines", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("Internal error listing VMs")
			})
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
		testVM              *vmopv1.VirtualMachine
		clusterIPFamily     string
		expectedNodeAddress []v1.NodeAddress
		expectedErr         error
	}{
		{
			name:                "NodeAddresses returns an empty address for found node with no IP",
			testVM:              createTestVM(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily:     ClusterIPFamilyIPv4,
			expectedNodeAddress: []v1.NodeAddress{},
			expectedErr:         nil,
		},
		{
			name:                "NodeAddresses returns a NotFound error for a not found node",
			testVM:              createTestVM("bogus", testClusterNameSpace, testVMUUID),
			clusterIPFamily:     ClusterIPFamilyIPv4,
			expectedNodeAddress: nil,
			expectedErr:         cloudprovider.InstanceNotFound,
		},
		{
			name:            "NodeAddresses returns a valid IPv4 address for a found node",
			testVM:          createTestVMWithVMIPAndHost(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4,
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
		{
			name:            "NodeAddresses returns a valid IPv6 address for a found node",
			testVM:          createTestVMWithIPv6(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddresses returns both IPv4 and IPv6 addresses for dual-stack node",
			testVM:          createTestVMWithDualStack(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddresses returns all valid IPs including secondary addresses",
			testVM:          createTestVMWithMultipleIPs(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4IPv6,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::1",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "10.0.0.5",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "2001:db8::100",
				},
				{
					Type:    v1.NodeHostName,
					Address: "",
				},
			},
			expectedErr: nil,
		},
		{
			name:            "NodeAddresses filters out link-local addresses",
			testVM:          createTestVMWithLinkLocalAddresses(string(testVMName), testClusterNameSpace, testVMUUID),
			clusterIPFamily: ClusterIPFamilyIPv4,
			expectedNodeAddress: []v1.NodeAddress{
				{
					Type:    v1.NodeInternalIP,
					Address: "1.2.3.4",
				},
				{
					Type:    v1.NodeInternalIP,
					Address: "10.0.0.5",
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
			instance, _, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			instance.clusterIPFamily = testCase.clusterIPFamily
			ret, err := instance.NodeAddresses(context.Background(), testVMName)
			assert.Equal(t, testCase.expectedErr, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}

func internalIPs(addrs []v1.NodeAddress) []string {
	var ips []string
	for _, a := range addrs {
		if a.Type == v1.NodeInternalIP {
			ips = append(ips, a.Address)
		}
	}
	return ips
}

func TestIsLinkLocalIP(t *testing.T) {
	testCases := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "empty string is not link-local", ip: "", want: false},
		{name: "unparseable string is not link-local", ip: "not-an-ip", want: false},
		{name: "IPv4 link-local 169.254.x.x", ip: "169.254.1.1", want: true},
		{name: "IPv4 non-link-local", ip: "10.0.0.1", want: false},
		{name: "IPv6 link-local fe80::1", ip: "fe80::1", want: true},
		{name: "IPv6 link-local upper boundary fe80::ffff", ip: "fe80::ffff", want: true},
		{name: "IPv6 non-link-local 2001:db8::1", ip: "2001:db8::1", want: false},
		{name: "loopback 127.0.0.1 is not link-local", ip: "127.0.0.1", want: false},
		{name: "loopback ::1 is not link-local", ip: "::1", want: false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := isLinkLocalIP(tc.ip)
			if got != tc.want {
				t.Errorf("isLinkLocalIP(%q) = %v, want %v", tc.ip, got, tc.want)
			}
		})
	}
}

func TestCreateNodeAddressesUnexpectedFamilyPanics(t *testing.T) {
	vm := &vmoptypes.VirtualMachineInfo{
		Name:       "test-vm",
		PrimaryIP4: "1.2.3.4",
		PrimaryIP6: "2001:db8::1",
	}
	// Invariant: clusterIPFamily must be a canonical value from
	// ParseClusterIPFamily. A non-canonical value indicates a programming
	// error in the caller and must fail fast rather than silently picking
	// a default ordering.
	assert.PanicsWithValue(t,
		`createNodeAddresses: invariant violated: clusterIPFamily "not-from-ParseClusterIPFamily" `+
			`is not canonical (must be one of "ipv4"/"ipv6"/"ipv4ipv6"/"ipv6ipv4" from ParseClusterIPFamily)`,
		func() {
			createNodeAddresses(vm, "not-from-ParseClusterIPFamily")
		},
	)
}

func TestNodeAddressesIPFamilyOrdering(t *testing.T) {
	dualStackVM := createTestVMWithDualStack(string(testVMName), testClusterNameSpace, testVMUUID)

	t.Run("ipv4ipv6 dual-stack cluster places IPv4 address first via NodeAddresses", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv4IPv6
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.2.3.4", "2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv6 single-stack cluster reports only IPv6 even when VM has both IPs", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv6
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv4ipv6 dual-stack cluster places IPv4 address first via NodeAddressesByProviderID", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv4IPv6
		addrs, err := instance.NodeAddressesByProviderID(context.Background(), testProviderID)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.2.3.4", "2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv6 single-stack cluster reports only IPv6 via NodeAddressesByProviderID even when VM has both IPs", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv6
		addrs, err := instance.NodeAddressesByProviderID(context.Background(), testProviderID)
		assert.NoError(t, err)
		assert.Equal(t, []string{"2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv6-primary cluster with IPv6-only VM returns single IPv6 address first", func(t *testing.T) {
		ipv6OnlyVM := createTestVMWithIPv6(string(testVMName), testClusterNameSpace, testVMUUID)
		instance, _, err := initTest(ipv6OnlyVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv6
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv4ipv6 uses same primary ordering as ipv4", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv4IPv6
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.2.3.4", "2001:db8::1"}, internalIPs(addrs))
	})

	t.Run("ipv6ipv4 uses same primary ordering as ipv6", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv6IPv4
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"2001:db8::1", "1.2.3.4"}, internalIPs(addrs))
	})

	t.Run("ipv4 single-stack cluster reports only IPv4 even when VM has both IPs", func(t *testing.T) {
		instance, _, err := initTest(dualStackVM)
		assert.NoError(t, err)
		instance.clusterIPFamily = ClusterIPFamilyIPv4
		addrs, err := instance.NodeAddresses(context.Background(), testVMName)
		assert.NoError(t, err)
		assert.Equal(t, []string{"1.2.3.4"}, internalIPs(addrs))
	})
}

func TestNodeAddressesInternalErr(t *testing.T) {
	testCases := []struct {
		name                string
		testVM              *vmopv1.VirtualMachine
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
			instance, fc, err := initTest(testCase.testVM)
			assert.NoError(t, err)
			fc.PrependReactor("get", "virtualmachines", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				return true, nil, fmt.Errorf("Internal error getting VMs")
			})
			ret, err := instance.NodeAddresses(context.Background(), testVMName)
			assert.NotEqual(t, nil, err)
			assert.NotEqual(t, cloudprovider.InstanceNotFound, err)
			assert.Equal(t, testCase.expectedNodeAddress, ret)
		})
	}
}
