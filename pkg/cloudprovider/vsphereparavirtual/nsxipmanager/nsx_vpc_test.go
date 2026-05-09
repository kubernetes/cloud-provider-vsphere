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

package nsxipmanager

import (
	"fmt"
	"strings"
	"testing"
	"time"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	nsxfake "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/fake"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	core "k8s.io/client-go/testing"
	"k8s.io/klog/v2/ktesting"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
	"k8s.io/cloud-provider-vsphere/pkg/util"
)

var (
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type reactor struct {
	verb     string
	resource string
	reaction core.ReactionFunc
}

type testCase struct {
	name          string
	svNamespace   string
	podIPPoolType string
	node          *corev1.Node
	// Objects to put in the store.
	ipAddressAllocationsLister []*vpcapisv1.IPAddressAllocation
	// Actions expected to happen on the client.
	actions []core.Action
	// Objects from here preloaded into NewSimpleFake.
	objects []runtime.Object
	// Client reaction from here preloaded into NewSimpleFake.
	reactors       []reactor
	expectedError  bool
	expectedErrMsg string
}

func initTest(tc *testCase, ns string, ownerRef *metav1.OwnerReference, t *testing.T) (NSXIPManager, *nsxfake.Clientset) {
	client := nsxfake.NewSimpleClientset()
	for _, obj := range tc.objects {
		client.Tracker().Add(obj)
	}
	for _, r := range tc.reactors {
		client.PrependReactor(r.verb, r.resource, r.reaction)
	}
	i := nsxinformers.NewSharedInformerFactory(client, noResyncPeriodFunc())
	for _, obj := range tc.ipAddressAllocationsLister {
		i.Crd().V1alpha1().IPAddressAllocations().Informer().GetIndexer().Add(obj)
	}
	_, ctx := ktesting.NewTestContext(t)
	i.Start(ctx.Done())
	return NewNSXVPCIPManager(client, i, ns, tc.podIPPoolType, ownerRef, true, false), client
}

// expectedCRLabels returns the label set that createIPAddressAllocation
// stamps onto every CR. Tests use this helper to keep fixtures aligned with
// the production label scheme.
func expectedCRLabels(nodeName string, ipv4 bool) map[string]string {
	family := helper.LabelValueIPFamilyIPv6
	if ipv4 {
		family = helper.LabelValueIPFamilyIPv4
	}
	return map[string]string{
		helper.LabelKeyNodeName: nodeName,
		helper.LabelKeyIPFamily: family,
	}
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "ipaddressallocations") ||
				action.Matches("watch", "ipaddressallocations")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func TestNSXVPCIPManager_ClaimPodCIDR(t *testing.T) {
	ns := "test-ns"
	nodeName := "test-node"
	nodeBeforeSet := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	nodeAfterSet := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: corev1.NodeSpec{
			PodCIDR: "10.244.0.0/24",
			PodCIDRs: []string{
				"10.244.0.0/24",
			},
		},
	}
	ownerRef := &metav1.OwnerReference{
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		Name:       "test-cluster",
		UID:        "1234",
	}
	testcases := []testCase{
		{
			name:          "Claim Public PodCIDR Successfully",
			svNamespace:   ns,
			node:          nodeBeforeSet,
			podIPPoolType: PublicIPPoolType,
			actions: []core.Action{
				core.NewCreateAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					&vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: ns,
							Labels:    expectedCRLabels(nodeName, true),
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "External",
							AllocationSize:           allocationSize,
						},
					},
				),
			},
		},
		{
			name:          "Claim Private PodCIDR Successfully",
			svNamespace:   ns,
			node:          nodeBeforeSet,
			podIPPoolType: PrivateIPPoolType,
			actions: []core.Action{
				core.NewCreateAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					&vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: ns,
							Labels:    expectedCRLabels(nodeName, true),
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "Private",
							AllocationSize:           allocationSize,
						},
					},
				),
			},
		},
		{
			name:          "Node PodCIDR Already Set",
			svNamespace:   ns,
			node:          nodeAfterSet,
			podIPPoolType: PrivateIPPoolType,
		},
		{
			name:          "Fail to create IPAddressAllocation",
			svNamespace:   ns,
			node:          nodeBeforeSet,
			podIPPoolType: PrivateIPPoolType,
			actions: []core.Action{
				core.NewCreateAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					&vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nodeName,
							Namespace: ns,
							Labels:    expectedCRLabels(nodeName, true),
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "Private",
							AllocationSize:           allocationSize,
						},
					},
				),
			},
			reactors: []reactor{
				{
					verb:     "create",
					resource: "ipaddressallocations",
					reaction: func(action core.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("API server failed")
					},
				},
			},
			expectedError:  true,
			expectedErrMsg: "API server failed",
		},
		{
			name:          "IPAddressAllocation Already Exists",
			svNamespace:   ns,
			node:          nodeBeforeSet,
			podIPPoolType: PrivateIPPoolType,
			ipAddressAllocationsLister: []*vpcapisv1.IPAddressAllocation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nodeName,
						Namespace: ns,
						OwnerReferences: []metav1.OwnerReference{
							*ownerRef,
						},
					},
					Spec: vpcapisv1.IPAddressAllocationSpec{
						IPAddressBlockVisibility: "Private",
						AllocationSize:           allocationSize,
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ipManager, client := initTest(&tc, ns, ownerRef, t)
			err := ipManager.ClaimPodCIDR(tc.node)
			if tc.expectedError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tc.expectedError && err != nil {
				t.Errorf("expected no error but got %v", err)
			}

			if tc.expectedError && !strings.Contains(err.Error(), tc.expectedErrMsg) {
				t.Errorf("expected error contains %s but got %s", tc.expectedErrMsg, err.Error())
			}

			util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
		})
	}
}

func TestNSXVPCIPManager_ReleasePodCIDR(t *testing.T) {
	ns := "test-ns"
	nodeName := "test-node"
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: corev1.NodeSpec{
			PodCIDR: "10.244.0.0/24",
			PodCIDRs: []string{
				"10.244.0.0/24",
			},
		},
	}
	ipa := &vpcapisv1.IPAddressAllocation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: ns,
		},
		Spec: vpcapisv1.IPAddressAllocationSpec{
			IPAddressBlockVisibility: "External",
			AllocationSize:           allocationSize,
		},
	}
	ownerRef := &metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Cluster",
		Name:       "test-cluster",
		UID:        "1234",
	}
	testcases := []testCase{
		{
			name:          "IPAddressAllocation Not Found",
			svNamespace:   ns,
			node:          node,
			podIPPoolType: PublicIPPoolType,
			// ReleasePodCIDR unconditionally calls Delete; a NotFound response is not an error.
			actions: []core.Action{
				core.NewDeleteAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					nodeName,
				),
			},
		},
		{
			name:          "Delete IPAddressAllocation Successfully",
			svNamespace:   ns,
			node:          node,
			podIPPoolType: PublicIPPoolType,
			objects: []runtime.Object{
				ipa,
			},
			ipAddressAllocationsLister: []*vpcapisv1.IPAddressAllocation{
				ipa,
			},
			actions: []core.Action{
				core.NewDeleteAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					ipa.Name,
				),
			},
		},
		{
			name:          "Delete IPAddressAllocation Fail",
			svNamespace:   ns,
			node:          node,
			podIPPoolType: PublicIPPoolType,
			objects: []runtime.Object{
				ipa,
			},
			ipAddressAllocationsLister: []*vpcapisv1.IPAddressAllocation{
				ipa,
			},
			actions: []core.Action{
				core.NewDeleteAction(
					schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version},
					ns,
					ipa.Name,
				),
			},
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "ipaddressallocations",
					reaction: func(action core.Action) (bool, runtime.Object, error) {
						return true, nil, fmt.Errorf("API server failed")
					},
				},
			},
			expectedError:  true,
			expectedErrMsg: "API server failed",
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ipManager, client := initTest(&tc, ns, ownerRef, t)
			err := ipManager.ReleasePodCIDR(tc.node)
			if tc.expectedError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tc.expectedError && err != nil {
				t.Errorf("expected no error but got %v", err)
			}

			if tc.expectedError && !strings.Contains(err.Error(), tc.expectedErrMsg) {
				t.Errorf("expected error contains %s but got %s", tc.expectedErrMsg, err.Error())
			}

			util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
		})
	}
}

func initTestWithFamilies(tc *testCase, ns string, ownerRef *metav1.OwnerReference, ipv4, ipv6 bool, t *testing.T) (NSXIPManager, *nsxfake.Clientset) {
	client := nsxfake.NewSimpleClientset()
	for _, obj := range tc.objects {
		client.Tracker().Add(obj)
	}
	for _, r := range tc.reactors {
		client.PrependReactor(r.verb, r.resource, r.reaction)
	}
	i := nsxinformers.NewSharedInformerFactory(client, noResyncPeriodFunc())
	for _, obj := range tc.ipAddressAllocationsLister {
		i.Crd().V1alpha1().IPAddressAllocations().Informer().GetIndexer().Add(obj)
	}
	_, ctx := ktesting.NewTestContext(t)
	i.Start(ctx.Done())
	return NewNSXVPCIPManager(client, i, ns, tc.podIPPoolType, ownerRef, ipv4, ipv6), client
}

// familyTestCase extends testCase with the ipv4/ipv6 enabled flags for the manager under test.
type familyTestCase struct {
	testCase
	ipv4Enabled bool
	ipv6Enabled bool
}

func TestNSXVPCIPManager_ClaimPodCIDR_Families(t *testing.T) {
	ns := "test-ns"
	nodeName := "test-node"
	nodeEmpty := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	ownerRef := &metav1.OwnerReference{
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		Name:       "test-cluster",
		UID:        "1234",
	}
	grv := schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version}

	testcases := []familyTestCase{
		{
			testCase: testCase{
				name:          "Dual stack creates both IPv4 and IPv6 CRs",
				svNamespace:   ns,
				node:          nodeEmpty,
				podIPPoolType: PrivateIPPoolType,
				actions: []core.Action{
					core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:            nodeName,
							Namespace:       ns,
							Labels:          expectedCRLabels(nodeName, true),
							OwnerReferences: []metav1.OwnerReference{*ownerRef},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "Private",
							AllocationSize:           allocationSize,
						},
					}),
					core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:            nodeName + helper.SuffixIPv6,
							Namespace:       ns,
							Labels:          expectedCRLabels(nodeName, false),
							OwnerReferences: []metav1.OwnerReference{*ownerRef},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:              vpcapisv1.IPAllocationIPAddressTypeIPv6,
							IPv6AllocationPrefixLength: ipv6AllocationPrefixLength,
						},
					}),
				},
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:          "IPv6 only creates one CR with -ipv6 suffix",
				svNamespace:   ns,
				node:          nodeEmpty,
				podIPPoolType: PrivateIPPoolType,
				actions: []core.Action{
					core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:            nodeName + helper.SuffixIPv6,
							Namespace:       ns,
							Labels:          expectedCRLabels(nodeName, false),
							OwnerReferences: []metav1.OwnerReference{*ownerRef},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:              vpcapisv1.IPAllocationIPAddressTypeIPv6,
							IPv6AllocationPrefixLength: ipv6AllocationPrefixLength,
						},
					}),
				},
			},
			ipv4Enabled: false,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:        "Dual stack skips both CRs when already fully allocated",
				svNamespace: ns,
				node: &corev1.Node{
					ObjectMeta: metav1.ObjectMeta{Name: nodeName},
					Spec: corev1.NodeSpec{
						PodCIDR:  "10.244.0.0/24",
						PodCIDRs: []string{"10.244.0.0/24", "fd00::/80"},
					},
				},
				podIPPoolType: PrivateIPPoolType,
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:          "Dual stack skips IPv4 CR when it already exists in lister",
				svNamespace:   ns,
				node:          nodeEmpty,
				podIPPoolType: PrivateIPPoolType,
				ipAddressAllocationsLister: []*vpcapisv1.IPAddressAllocation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: nodeName, Namespace: ns},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "Private",
							AllocationSize:           allocationSize,
						},
					},
				},
				// Only IPv6 create is expected; IPv4 already exists in lister.
				actions: []core.Action{
					core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:            nodeName + helper.SuffixIPv6,
							Namespace:       ns,
							Labels:          expectedCRLabels(nodeName, false),
							OwnerReferences: []metav1.OwnerReference{*ownerRef},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:              vpcapisv1.IPAllocationIPAddressTypeIPv6,
							IPv6AllocationPrefixLength: ipv6AllocationPrefixLength,
						},
					}),
				},
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:        "No families enabled — no CRs created",
				svNamespace: ns,
				node:        nodeEmpty,
				// Neither IPv4 nor IPv6 is enabled; ClaimPodCIDR should be a no-op.
				actions: nil,
			},
			ipv4Enabled: false,
			ipv6Enabled: false,
		},
		{
			testCase: testCase{
				name:          "Dual stack skips IPv6 CR when it already exists in lister",
				svNamespace:   ns,
				node:          nodeEmpty,
				podIPPoolType: PrivateIPPoolType,
				ipAddressAllocationsLister: []*vpcapisv1.IPAddressAllocation{
					{
						ObjectMeta: metav1.ObjectMeta{Name: nodeName + helper.SuffixIPv6, Namespace: ns},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:              vpcapisv1.IPAllocationIPAddressTypeIPv6,
							IPv6AllocationPrefixLength: ipv6AllocationPrefixLength,
						},
					},
				},
				// Only IPv4 create is expected; IPv6 already exists in lister.
				actions: []core.Action{
					core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
						ObjectMeta: metav1.ObjectMeta{
							Name:            nodeName,
							Namespace:       ns,
							Labels:          expectedCRLabels(nodeName, true),
							OwnerReferences: []metav1.OwnerReference{*ownerRef},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
							IPAddressBlockVisibility: "Private",
							AllocationSize:           allocationSize,
						},
					}),
				},
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ipManager, client := initTestWithFamilies(&tc.testCase, ns, ownerRef, tc.ipv4Enabled, tc.ipv6Enabled, t)
			err := ipManager.ClaimPodCIDR(tc.node)
			if tc.expectedError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tc.expectedError && err != nil {
				t.Errorf("expected no error but got %v", err)
			}
			util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
		})
	}
}

func TestNSXVPCIPManager_ReleasePodCIDR_Families(t *testing.T) {
	ns := "test-ns"
	nodeName := "test-node"
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	ownerRef := &metav1.OwnerReference{
		APIVersion: "cluster.x-k8s.io/v1beta1",
		Kind:       "Cluster",
		Name:       "test-cluster",
		UID:        "1234",
	}
	grv := schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version}

	testcases := []familyTestCase{
		{
			testCase: testCase{
				name:          "Dual stack unconditionally deletes both CRs",
				svNamespace:   ns,
				node:          node,
				podIPPoolType: PrivateIPPoolType,
				actions: []core.Action{
					core.NewDeleteAction(grv, ns, nodeName),
					core.NewDeleteAction(grv, ns, nodeName+helper.SuffixIPv6),
				},
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:          "IPv6 only deletes just the -ipv6 CR",
				svNamespace:   ns,
				node:          node,
				podIPPoolType: PrivateIPPoolType,
				actions: []core.Action{
					core.NewDeleteAction(grv, ns, nodeName+helper.SuffixIPv6),
				},
			},
			ipv4Enabled: false,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:          "Dual stack treats NotFound as success for both CRs",
				svNamespace:   ns,
				node:          node,
				podIPPoolType: PrivateIPPoolType,
				// No objects pre-loaded so both deletes return 404; expect the actions
				// but no error.
				actions: []core.Action{
					core.NewDeleteAction(grv, ns, nodeName),
					core.NewDeleteAction(grv, ns, nodeName+helper.SuffixIPv6),
				},
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
		{
			testCase: testCase{
				name:          "Dual stack propagates error when IPv6 delete fails",
				svNamespace:   ns,
				node:          node,
				podIPPoolType: PrivateIPPoolType,
				// IPv4 delete succeeds (no reactor for it), IPv6 delete returns an error.
				// The IPv4 delete action still happens; the IPv6 action is attempted and fails.
				reactors: []reactor{
					{
						verb:     "delete",
						resource: "ipaddressallocations",
						reaction: func(action core.Action) (bool, runtime.Object, error) {
							da, ok := action.(core.DeleteAction)
							if ok && da.GetName() == nodeName+helper.SuffixIPv6 {
								return true, nil, fmt.Errorf("API server failed on IPv6 delete")
							}
							// Return false to pass through to the default fake reactor,
							// which handles the IPv4 delete as a no-op (NotFound).
							return false, nil, nil
						},
					},
				},
				actions: []core.Action{
					core.NewDeleteAction(grv, ns, nodeName),
					core.NewDeleteAction(grv, ns, nodeName+helper.SuffixIPv6),
				},
				expectedError: true,
			},
			ipv4Enabled: true,
			ipv6Enabled: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ipManager, client := initTestWithFamilies(&tc.testCase, ns, ownerRef, tc.ipv4Enabled, tc.ipv6Enabled, t)
			err := ipManager.ReleasePodCIDR(tc.node)
			if tc.expectedError && err == nil {
				t.Errorf("expected error but got none")
			} else if !tc.expectedError && err != nil {
				t.Errorf("expected no error but got %v", err)
			}
			util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
		})
	}
}

func TestCrNameForFamily(t *testing.T) {
	tests := []struct {
		name     string
		nodeName string
		ipv4     bool
		expected string
	}{
		{"plain node, ipv4 keeps bare name", "node-1", true, "node-1"},
		{"plain node, ipv6 appends suffix", "node-1", false, "node-1" + helper.SuffixIPv6},
		// Regression: a node literally named "worker-ipv6" must keep its bare
		// name as the IPv4 CR name and get "worker-ipv6-ipv6" for IPv6.
		{"node ending in -ipv6, ipv4 keeps bare name", "worker-ipv6", true, "worker-ipv6"},
		{"node ending in -ipv6, ipv6 still appends suffix", "worker-ipv6", false, "worker-ipv6" + helper.SuffixIPv6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := crNameForFamily(tt.nodeName, tt.ipv4)
			if got != tt.expected {
				t.Errorf("crNameForFamily(%q, %v) = %q, want %q", tt.nodeName, tt.ipv4, got, tt.expected)
			}
		})
	}
}

// TestNSXVPCIPManager_ReleasePodCIDR_AggregatesErrors verifies that when
// both IPv4 and IPv6 deletes fail, both errors are reported (not just the
// first one), and the second delete is still attempted.
func TestNSXVPCIPManager_ReleasePodCIDR_AggregatesErrors(t *testing.T) {
	ns := "test-ns"
	nodeName := "test-node"
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	ownerRef := &metav1.OwnerReference{Name: "test-cluster"}
	grv := schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version}

	tc := testCase{
		svNamespace:   ns,
		node:          node,
		podIPPoolType: PrivateIPPoolType,
		// Both deletes fail; aggregation must surface both failure messages
		// AND still issue both delete actions.
		reactors: []reactor{
			{
				verb:     "delete",
				resource: "ipaddressallocations",
				reaction: func(action core.Action) (bool, runtime.Object, error) {
					da := action.(core.DeleteAction)
					return true, nil, fmt.Errorf("API failed on %s", da.GetName())
				},
			},
		},
		actions: []core.Action{
			core.NewDeleteAction(grv, ns, nodeName),
			core.NewDeleteAction(grv, ns, nodeName+helper.SuffixIPv6),
		},
	}

	ipManager, client := initTestWithFamilies(&tc, ns, ownerRef, true, true, t)
	err := ipManager.ReleasePodCIDR(node)
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}
	if !strings.Contains(err.Error(), "API failed on "+nodeName) {
		t.Errorf("expected IPv4 failure in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "API failed on "+nodeName+helper.SuffixIPv6) {
		t.Errorf("expected IPv6 failure in error, got %v", err)
	}
	util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
}

// TestNSXVPCIPManager_NodeEndingInIPv6 verifies that a node whose name itself
// ends in "-ipv6" gets correctly-named CRs in dual-stack mode: the IPv4 CR
// uses the bare name "worker-ipv6" and the IPv6 CR appends another -ipv6 to
// become "worker-ipv6-ipv6". Labels distinguish the two unambiguously.
func TestNSXVPCIPManager_NodeEndingInIPv6(t *testing.T) {
	ns := "test-ns"
	nodeName := "worker-ipv6"
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
	ownerRef := &metav1.OwnerReference{Name: "test-cluster"}
	grv := schema.GroupVersionResource{Resource: "ipaddressallocations", Group: vpcapisv1.GroupVersion.Group, Version: vpcapisv1.GroupVersion.Version}

	tc := testCase{
		svNamespace:   ns,
		node:          node,
		podIPPoolType: PrivateIPPoolType,
		actions: []core.Action{
			core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
				ObjectMeta: metav1.ObjectMeta{
					Name:            nodeName, // bare name, *not* stripped
					Namespace:       ns,
					Labels:          expectedCRLabels(nodeName, true),
					OwnerReferences: []metav1.OwnerReference{*ownerRef},
				},
				Spec: vpcapisv1.IPAddressAllocationSpec{
					IPAddressType:            vpcapisv1.IPAllocationIPAddressTypeIPv4,
					IPAddressBlockVisibility: "Private",
					AllocationSize:           allocationSize,
				},
			}),
			core.NewCreateAction(grv, ns, &vpcapisv1.IPAddressAllocation{
				ObjectMeta: metav1.ObjectMeta{
					Name:            nodeName + helper.SuffixIPv6, // -ipv6 appended once
					Namespace:       ns,
					Labels:          expectedCRLabels(nodeName, false),
					OwnerReferences: []metav1.OwnerReference{*ownerRef},
				},
				Spec: vpcapisv1.IPAddressAllocationSpec{
					IPAddressType:              vpcapisv1.IPAllocationIPAddressTypeIPv6,
					IPv6AllocationPrefixLength: ipv6AllocationPrefixLength,
				},
			}),
		},
	}

	ipManager, client := initTestWithFamilies(&tc, ns, ownerRef, true, true, t)
	if err := ipManager.ClaimPodCIDR(node); err != nil {
		t.Fatalf("ClaimPodCIDR failed: %v", err)
	}
	util.CheckActions(tc.actions, filterInformerActions(client.Actions()), t)
}
