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

package vsphereparavirtual

import (
	"context"
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	fakev6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/adapter/v1alpha6/fake"
	clientv6 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/provider/v1alpha6"
)

func TestSupervisorVMOPIPv6SupportFlagDefaults(t *testing.T) {
	f := flag.Lookup("supervisor-vmop-ipv6-support")
	require.NotNil(t, f, "Supervisor IPv6/dual-stack support must be explicitly configurable")
	assert.Equal(t, "false", f.DefValue)
	assert.Contains(t, f.Usage, "VM Operator")
	assert.Contains(t, f.Usage, "WorkloadIPv6")
	assert.Contains(t, f.Usage, "does not enable")
	assert.Contains(t, f.Usage, "IPv4")
}

func TestNewLoadBalancerServiceIPFamilyPropagation(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "disabled"
		if enabled {
			name = "enabled for IPv4-only guest"
		}
		t.Run(name, func(t *testing.T) {
			adapter, fc := fakev6.NewAdapter()
			lb, err := NewLoadBalancer(testClusterNameSpace, adapter, &testOwnerReference, false, enabled)
			require.NoError(t, err)
			policy := corev1.IPFamilyPolicySingleStack
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "ipv4-lb", Namespace: "guest"},
				Spec: corev1.ServiceSpec{
					Type:           corev1.ServiceTypeLoadBalancer,
					Ports:          []corev1.ServicePort{{Name: "http", Protocol: corev1.ProtocolTCP, Port: 80, NodePort: 30080}},
					IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol},
					IPFamilyPolicy: &policy,
				},
			}
			created, err := lb.(*loadBalancer).vmService.Create(context.Background(), service, testClustername)
			require.NoError(t, err)
			raw, err := fc.Resource(clientv6.VirtualMachineServiceGVR).Namespace(testClusterNameSpace).Get(context.Background(), created.Name, metav1.GetOptions{})
			require.NoError(t, err)
			families, hasFamilies, err := unstructured.NestedStringSlice(raw.Object, "spec", "ipFamilies")
			require.NoError(t, err)
			actualPolicy, hasPolicy, err := unstructured.NestedString(raw.Object, "spec", "ipFamilyPolicy")
			require.NoError(t, err)
			assert.Equal(t, enabled, hasFamilies)
			assert.Equal(t, enabled, hasPolicy)
			if enabled {
				assert.Equal(t, []string{"IPv4"}, families)
				assert.Equal(t, string(policy), actualPolicy)
			}
		})
	}
}
