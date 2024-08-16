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
	return NewNSXVPCIPManager(client, i, ns, tc.podIPPoolType, ownerRef), client
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
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressBlockVisibility: vpcapisv1.IPAddressVisibilityExternal,
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
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressBlockVisibility: vpcapisv1.IPAddressVisibilityPrivate,
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
							OwnerReferences: []metav1.OwnerReference{
								*ownerRef,
							},
						},
						Spec: vpcapisv1.IPAddressAllocationSpec{
							IPAddressBlockVisibility: vpcapisv1.IPAddressVisibilityPrivate,
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
						IPAddressBlockVisibility: vpcapisv1.IPAddressVisibilityPrivate,
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
			IPAddressBlockVisibility: vpcapisv1.IPAddressVisibilityExternal,
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
