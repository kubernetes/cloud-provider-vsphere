/*
Copyright 2017 The Kubernetes Authors.

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

package ipaddressallocation

import (
	"context"
	"fmt"
	"testing"
	"time"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	kubeinformers "k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2/ktesting"

	nsxfake "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/fake"
	nsxinformers "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions"

	"k8s.io/cloud-provider-vsphere/pkg/util"
)

var (
	alwaysReady        = func() bool { return true }
	noResyncPeriodFunc = func() time.Duration { return 0 }
)

type reactor struct {
	verb     string
	resource string
	reaction core.ReactionFunc
}

type fixture struct {
	t *testing.T

	client     *nsxfake.Clientset
	kubeclient *k8sfake.Clientset
	// Objects to put in the store.
	ipAddressAllocationsLister []*vpcapisv1.IPAddressAllocation
	nodesLister                []*corev1.Node
	// Actions expected to happen on the client.
	kubeactions []core.Action
	actions     []core.Action
	// Objects from here preloaded into NewSimpleFake.
	kubeobjects []runtime.Object
	objects     []runtime.Object
	// Client reaction from here preloaded into NewSimpleFake.
	kubeReactors []reactor
	reactors     []reactor
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	return f
}

func newNode(name string, podCIDR string, podCIDRs []string) *corev1.Node {
	return &corev1.Node{
		TypeMeta: metav1.TypeMeta{APIVersion: corev1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: corev1.NodeSpec{
			PodCIDR:  podCIDR,
			PodCIDRs: podCIDRs,
		},
	}
}

func newIPAddressAllocation(name string, ipAddressBlockVisibility vpcapisv1.IPAddressVisibility, cidr string, conditionStatus corev1.ConditionStatus) *vpcapisv1.IPAddressAllocation {
	return &vpcapisv1.IPAddressAllocation{
		TypeMeta: metav1.TypeMeta{APIVersion: vpcapisv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceDefault,
		},
		Spec: vpcapisv1.IPAddressAllocationSpec{
			IPAddressBlockVisibility: ipAddressBlockVisibility,
			AllocationSize:           24,
		},
		Status: vpcapisv1.IPAddressAllocationStatus{
			AllocationIPs: cidr,
			Conditions: []vpcapisv1.Condition{
				{
					Type:   vpcapisv1.Ready,
					Status: conditionStatus,
				},
			},
		},
	}
}

func (f *fixture) newController(ctx context.Context) (*Controller, nsxinformers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = nsxfake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	for _, r := range f.kubeReactors {
		f.kubeclient.PrependReactor(r.verb, r.resource, r.reaction)
	}

	for _, r := range f.reactors {
		f.client.PrependReactor(r.verb, r.resource, r.reaction)
	}

	i := nsxinformers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	nodeInformer := k8sI.Core().V1().Nodes()
	ipAddressAllocationInformer := i.Crd().V1alpha1().IPAddressAllocations()
	c := NewController(ctx, f.kubeclient, nodeInformer.Lister(), nodeInformer.Informer().HasSynced, ipAddressAllocationInformer)

	c.ipAddressAllocationsSynced = alwaysReady
	c.nodesSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, f := range f.ipAddressAllocationsLister {
		ipAddressAllocationInformer.Informer().GetIndexer().Add(f)
	}

	for _, d := range f.nodesLister {
		nodeInformer.Informer().GetIndexer().Add(d)
	}

	return c, i, k8sI
}

func (f *fixture) run(ctx context.Context, key string) {
	f.runController(ctx, key, true, false)
}

func (f *fixture) runExpectError(ctx context.Context, key string) {
	f.runController(ctx, key, true, true)
}

func (f *fixture) runController(ctx context.Context, key string, startInformers bool, expectError bool) {
	c, i, k8sI := f.newController(ctx)
	if startInformers {
		i.Start(ctx.Done())
		k8sI.Start(ctx.Done())
	}

	err := c.syncHandler(ctx, key)
	if !expectError && err != nil {
		f.t.Errorf("error syncing IPAddressAllocation: %v", err)
	} else if expectError && err == nil {
		f.t.Error("expected error syncing IPAddressAllocation, got nil")
	}

	util.CheckActions(f.actions, filterInformerActions(f.client.Actions()), f.t)

	util.CheckActions(f.kubeactions, filterInformerActions(f.kubeclient.Actions()), f.t)
}

// filterInformerActions filters list and watch actions for testing resources.
// Since list and watch don't change resource state we can filter it to lower
// nose level in our tests.
func filterInformerActions(actions []core.Action) []core.Action {
	ret := []core.Action{}
	for _, action := range actions {
		if len(action.GetNamespace()) == 0 &&
			(action.Matches("list", "ipaddressallocations") ||
				action.Matches("watch", "ipaddressallocations") ||
				action.Matches("list", "nodes") ||
				action.Matches("watch", "nodes")) {
			continue
		}
		ret = append(ret, action)
	}

	return ret
}

func (f *fixture) expectPatchNodeAction(name string, patchType types.PatchType, patch []byte) {
	action := core.NewRootPatchAction(schema.GroupVersionResource{Resource: "nodes", Version: "v1"}, name, patchType, patch)
	f.kubeactions = append(f.kubeactions, action)
}

func getKey(ipAddressAllocation *vpcapisv1.IPAddressAllocation, t *testing.T) string {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(ipAddressAllocation)
	if err != nil {
		t.Errorf("Unexpected error getting key for IPAddressAllocation %v: %v", ipAddressAllocation.Name, err)
		return ""
	}
	return key
}

func TestPatchNodeCIDR(t *testing.T) {
	name := "node-1"
	cidr := "192.168.5.0/24"
	f := newFixture(t)
	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, cidr, corev1.ConditionTrue)
	node := newNode(name, "", nil)
	_, ctx := ktesting.NewTestContext(t)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s"]}}`, cidr, cidr))
	f.expectPatchNodeAction(name, types.StrategicMergePatchType, patch)

	f.run(ctx, getKey(ipAddressAllocation, t))
}

func TestIPAddressAllocationNotExist(t *testing.T) {
	name := "node-1"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	f.run(ctx, name)
}

func TestIPAddressAllocationNotReady(t *testing.T) {
	name := "node-1"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, "192.168.5.0/24", corev1.ConditionFalse)
	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)

	f.runExpectError(ctx, getKey(ipAddressAllocation, t))
}
func TestIPAddressAllocationEmptyCIDR(t *testing.T) {
	name := "node-1"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, "", corev1.ConditionTrue)
	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)

	f.runExpectError(ctx, getKey(ipAddressAllocation, t))
}

func TestNodeNotExist(t *testing.T) {
	name := "node-1"
	cidr := "192.168.5.0/24"
	f := newFixture(t)
	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, cidr, corev1.ConditionTrue)
	node := newNode("node-2", "", nil)
	_, ctx := ktesting.NewTestContext(t)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	f.runExpectError(ctx, getKey(ipAddressAllocation, t))
}
func TestPatchNodeCIDRFailed(t *testing.T) {
	name := "node-1"
	cidr := "192.168.5.0/24"
	f := newFixture(t)
	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, cidr, corev1.ConditionTrue)
	node := newNode(name, "", nil)
	_, ctx := ktesting.NewTestContext(t)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	patchReactor := reactor{
		verb:     "patch",
		resource: "nodes",
		reaction: func(action core.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("API server failed")
		},
	}
	f.kubeReactors = append(f.kubeReactors, patchReactor)
	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s"]}}`, cidr, cidr))
	// expect 3 times to patch the node
	f.expectPatchNodeAction(name, types.StrategicMergePatchType, patch)
	f.expectPatchNodeAction(name, types.StrategicMergePatchType, patch)
	f.expectPatchNodeAction(name, types.StrategicMergePatchType, patch)

	f.runExpectError(ctx, getKey(ipAddressAllocation, t))
}
