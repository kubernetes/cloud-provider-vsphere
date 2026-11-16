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

package vmservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	vmopv6 "github.com/vmware-tanzu/vm-operator/api/v1alpha6"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"

	adapterv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha6"
	clientv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha6"
)

func initIPFamilyTest(t *testing.T, enabled bool) (*v1.Service, VMService, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, vmopv6.AddToScheme(scheme))
	fc := dynamicfake.NewSimpleDynamicClient(scheme)
	adapter := adapterv6.NewWithFakeClient(clientv6.NewWithDynamicClient(fc))
	svc := NewVMService(adapter, testClusterNameSpace, &testOwnerReference, false, enabled)
	source := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "ip-family-test", Namespace: testK8sServiceNameSpace},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{Name: "http", Port: 80, NodePort: 30080, Protocol: v1.ProtocolTCP}},
		},
	}
	return source, svc, fc
}

func assertIPFamilyFields(t *testing.T, fc *dynamicfake.FakeDynamicClient, name string, families []v1.IPFamily, policy *v1.IPFamilyPolicyType) {
	t.Helper()
	raw, err := fc.Resource(clientv6.VirtualMachineServiceGVR).Namespace(testClusterNameSpace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	spec, found, err := unstructured.NestedMap(raw.Object, "spec")
	require.NoError(t, err)
	require.True(t, found)
	if len(families) == 0 {
		assert.NotContains(t, spec, "ipFamilies")
	} else {
		actual, found, err := unstructured.NestedStringSlice(raw.Object, "spec", "ipFamilies")
		require.NoError(t, err)
		require.True(t, found)
		expected := make([]string, len(families))
		for i, family := range families {
			expected[i] = string(family)
		}
		assert.Equal(t, expected, actual)
	}
	if policy == nil {
		assert.NotContains(t, spec, "ipFamilyPolicy")
	} else {
		assert.Equal(t, string(*policy), spec["ipFamilyPolicy"])
	}
}

func TestCreateVMService_ServiceIPFamilyFieldsGate(t *testing.T) {
	singleStack := v1.IPFamilyPolicySingleStack
	preferDualStack := v1.IPFamilyPolicyPreferDualStack
	requireDualStack := v1.IPFamilyPolicyRequireDualStack
	tests := []struct {
		name     string
		enabled  bool
		families []v1.IPFamily
		policy   *v1.IPFamilyPolicyType
	}{
		{name: "disabled IPv4 defaults", families: []v1.IPFamily{v1.IPv4Protocol}, policy: &singleStack},
		{name: "disabled explicit PreferDualStack", families: []v1.IPFamily{v1.IPv4Protocol}, policy: &preferDualStack},
		{name: "enabled IPv4 single stack", enabled: true, families: []v1.IPFamily{v1.IPv4Protocol}, policy: &singleStack},
		{name: "enabled IPv6 single stack", enabled: true, families: []v1.IPFamily{v1.IPv6Protocol}, policy: &singleStack},
		{name: "enabled IPv4 primary", enabled: true, families: []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}, policy: &preferDualStack},
		{name: "enabled IPv6 primary", enabled: true, families: []v1.IPFamily{v1.IPv6Protocol, v1.IPv4Protocol}, policy: &requireDualStack},
		{name: "enabled no fields", enabled: true},
		{name: "enabled empty families", enabled: true, families: []v1.IPFamily{}},
		{name: "enabled family only", enabled: true, families: []v1.IPFamily{v1.IPv6Protocol}},
		{name: "enabled policy only", enabled: true, policy: &preferDualStack},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, svc, fc := initIPFamilyTest(t, tt.enabled)
			source.Spec.IPFamilies, source.Spec.IPFamilyPolicy = tt.families, tt.policy
			before := source.DeepCopy()
			created, err := svc.Create(context.Background(), source, testClustername)
			require.NoError(t, err)
			families, policy := tt.families, tt.policy
			if !tt.enabled {
				families, policy = nil, nil
			}
			assertIPFamilyFields(t, fc, created.Name, families, policy)
			assert.Equal(t, before, source, "creation must not mutate the Guest Service")
		})
	}
}

func TestUpdateVMService_ServiceIPFamilyFieldsDisabledNoRepeatedUpdates(t *testing.T) {
	source, svc, fc := initIPFamilyTest(t, false)
	policy := v1.IPFamilyPolicySingleStack
	source.Spec.IPFamilies = []v1.IPFamily{v1.IPv4Protocol}
	source.Spec.IPFamilyPolicy = &policy
	current, err := svc.Create(context.Background(), source, testClustername)
	require.NoError(t, err)
	fc.ClearActions()
	for range 2 {
		current, err = svc.Update(context.Background(), source, testClustername, current)
		require.NoError(t, err)
	}
	for _, action := range fc.Actions() {
		assert.NotEqual(t, "update", action.GetVerb(), "ignored fields must not cause an update")
	}
	assertIPFamilyFields(t, fc, current.Name, nil, nil)
}

func TestUpdateVMService_ServiceIPFamilyFieldsDisabledOmitsFields(t *testing.T) {
	source, svc, fc := initIPFamilyTest(t, false)
	current, err := svc.Create(context.Background(), source, testClustername)
	require.NoError(t, err)
	singleStack := v1.IPFamilyPolicySingleStack
	source.Spec.IPFamilies = []v1.IPFamily{v1.IPv4Protocol}
	source.Spec.IPFamilyPolicy = &singleStack
	source.Spec.Ports[0].Port = 8080
	before := source.DeepCopy()
	fc.ClearActions()
	updated, err := svc.Update(context.Background(), source, testClustername, current)
	require.NoError(t, err)
	assert.Equal(t, int32(8080), updated.Spec.Ports[0].Port)
	assertIPFamilyFields(t, fc, updated.Name, nil, nil)
	assert.Equal(t, before, source)
	assert.Equal(t, int32(80), current.Spec.Ports[0].Port)
	updates := 0
	for _, action := range fc.Actions() {
		if action.Matches("update", "virtualmachineservices") {
			updates++
		}
	}
	assert.Equal(t, 1, updates)
}

func TestUpdateVMService_ServiceIPFamilyFieldsEnabled(t *testing.T) {
	singleStack := v1.IPFamilyPolicySingleStack
	preferDualStack := v1.IPFamilyPolicyPreferDualStack
	tests := []struct {
		name     string
		families []v1.IPFamily
		policy   *v1.IPFamilyPolicyType
	}{
		{name: "IPv4 single stack", families: []v1.IPFamily{v1.IPv4Protocol}, policy: &singleStack},
		{name: "IPv6 single stack", families: []v1.IPFamily{v1.IPv6Protocol}, policy: &singleStack},
		{name: "IPv4 primary", families: []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}, policy: &preferDualStack},
		{name: "IPv6 primary", families: []v1.IPFamily{v1.IPv6Protocol, v1.IPv4Protocol}, policy: &preferDualStack},
		{name: "no defaults"},
		{name: "empty families", families: []v1.IPFamily{}},
		{name: "family only", families: []v1.IPFamily{v1.IPv6Protocol}},
		{name: "policy only", policy: &preferDualStack},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, svc, fc := initIPFamilyTest(t, true)
			initialPolicy := v1.IPFamilyPolicyRequireDualStack
			source.Spec.IPFamilies = []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}
			source.Spec.IPFamilyPolicy = &initialPolicy
			current, err := svc.Create(context.Background(), source, testClustername)
			require.NoError(t, err)
			source.Spec.IPFamilies, source.Spec.IPFamilyPolicy = tt.families, tt.policy
			source.Spec.Ports[0].Port = 8080
			before := source.DeepCopy()
			updated, err := svc.Update(context.Background(), source, testClustername, current)
			require.NoError(t, err)
			assertIPFamilyFields(t, fc, updated.Name, tt.families, tt.policy)
			assert.Equal(t, before, source)
			fc.ClearActions()
			_, err = svc.Update(context.Background(), source, testClustername, updated)
			require.NoError(t, err)
			for _, action := range fc.Actions() {
				assert.False(t, action.Matches("update", "virtualmachineservices"), "identical effective fields must not trigger repeated updates")
			}
		})
	}
}

func TestUpdateVMService_ServiceIPFamilyFieldsOnly(t *testing.T) {
	tests := []struct {
		name     string
		families []v1.IPFamily
		policy   v1.IPFamilyPolicyType
	}{
		{name: "families only", families: []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}, policy: v1.IPFamilyPolicyPreferDualStack},
		{name: "policy only", families: []v1.IPFamily{v1.IPv4Protocol}, policy: v1.IPFamilyPolicySingleStack},
		{name: "families and policy", families: []v1.IPFamily{v1.IPv4Protocol, v1.IPv6Protocol}, policy: v1.IPFamilyPolicyRequireDualStack},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, svc, fc := initIPFamilyTest(t, true)
			initialPolicy := v1.IPFamilyPolicyPreferDualStack
			source.Spec.IPFamilies = []v1.IPFamily{v1.IPv4Protocol}
			source.Spec.IPFamilyPolicy = &initialPolicy
			current, err := svc.Create(context.Background(), source, testClustername)
			require.NoError(t, err)
			initial, err := fc.Resource(clientv6.VirtualMachineServiceGVR).Namespace(testClusterNameSpace).Get(context.Background(), current.Name, metav1.GetOptions{})
			require.NoError(t, err)
			desired := source.DeepCopy()
			desired.Spec.IPFamilies = tt.families
			desired.Spec.IPFamilyPolicy = &tt.policy
			before := desired.DeepCopy()
			fc.ClearActions()

			updated, err := svc.Update(context.Background(), desired, testClustername, current)
			require.NoError(t, err)
			var updates []clienttesting.Action
			for _, action := range fc.Actions() {
				if action.Matches("update", "virtualmachineservices") {
					updates = append(updates, action)
				}
			}
			require.Len(t, updates, 1, "family fields alone must trigger exactly one update")
			update, ok := updates[0].(clienttesting.UpdateAction)
			require.True(t, ok)
			raw, ok := update.GetObject().(*unstructured.Unstructured)
			require.True(t, ok)
			expected := initial.DeepCopy()
			families := make([]string, len(tt.families))
			for i, family := range tt.families {
				families[i] = string(family)
			}
			require.NoError(t, unstructured.SetNestedStringSlice(expected.Object, families, "spec", "ipFamilies"))
			require.NoError(t, unstructured.SetNestedField(expected.Object, string(tt.policy), "spec", "ipFamilyPolicy"))
			assert.Equal(t, expected.Object["spec"], raw.Object["spec"], "serialized update must change only IP family fields")
			assert.Equal(t, current.Spec.Ports, updated.Spec.Ports, "ports must remain unchanged")
			assert.Equal(t, tt.families, updated.Spec.IPFamilies)
			require.NotNil(t, updated.Spec.IPFamilyPolicy)
			assert.Equal(t, tt.policy, *updated.Spec.IPFamilyPolicy)
			assert.Equal(t, before, desired, "update must not mutate the Guest Service")
			assertIPFamilyFields(t, fc, updated.Name, tt.families, &tt.policy)

			fc.ClearActions()
			_, err = svc.Update(context.Background(), desired, testClustername, updated)
			require.NoError(t, err)
			for _, action := range fc.Actions() {
				assert.False(t, action.Matches("update", "virtualmachineservices"), "identical family fields must not trigger another update")
			}
		})
	}
}
