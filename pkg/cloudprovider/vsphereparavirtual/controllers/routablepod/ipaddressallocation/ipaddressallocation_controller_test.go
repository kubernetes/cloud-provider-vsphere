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

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ipfamily"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
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

	// IPFamily for the controller.
	ipFamily ipfamily.IPFamily
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{}
	f.t = t
	f.objects = []runtime.Object{}
	f.kubeobjects = []runtime.Object{}
	f.ipFamily = ipfamily.IPv4 // default to IPv4
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

// newIPAddressAllocation builds a legacy-style IPAddressAllocation: the CR
// carries no nodeName/ipfamily labels and leaves Spec.IPAddressType unset, so
// the controller falls through to the backward-compatible name-stripping
// path. New tests that need to exercise the label-based path should use
// newLabeledIPAddressAllocation below.
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

// newLabeledIPAddressAllocation builds an IPAddressAllocation that matches
// what nsxipmanager.NSXVPCIPManager.createIPAddressAllocation produces: it
// carries the nodeName + ipfamily labels and sets Spec.IPAddressType. Tests
// that want to exercise the post-fix identification path (especially for
// node names that themselves end in "-ipv6") should use this constructor.
func newLabeledIPAddressAllocation(crName, nodeName string, ipv4 bool, cidr string, conditionStatus corev1.ConditionStatus) *vpcapisv1.IPAddressAllocation {
	addrType := vpcapisv1.IPAllocationIPAddressTypeIPv6
	if ipv4 {
		addrType = vpcapisv1.IPAllocationIPAddressTypeIPv4
	}
	return &vpcapisv1.IPAddressAllocation{
		TypeMeta: metav1.TypeMeta{APIVersion: vpcapisv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:      crName,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				helper.LabelKeyNodeName:    nodeName,
				helper.LabelKeyClusterName: "test-cluster",
			},
		},
		Spec: vpcapisv1.IPAddressAllocationSpec{
			IPAddressType: addrType,
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
	c := NewController(ctx, f.kubeclient, nodeInformer.Lister(), nodeInformer.Informer().HasSynced, ipAddressAllocationInformer, f.ipFamily, "test-cluster")

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

// TestPatchNodeCIDR_AlreadySet_SingleStack verifies that a single-stack node
// whose PodCIDRs already matches the allocation CIDR does not trigger a
// redundant API server PATCH (the 30-second resync idempotency guard).
func TestPatchNodeCIDR_AlreadySet_SingleStack(t *testing.T) {
	name := "node-1"
	cidr := "192.168.5.0/24"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipAddressAllocation := newIPAddressAllocation(name, vpcapisv1.IPAddressVisibilityPrivate, cidr, corev1.ConditionTrue)
	// Node already has the correct PodCIDR set — no patch should be issued.
	node := newNode(name, cidr, []string{cidr})

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipAddressAllocation)
	f.objects = append(f.objects, ipAddressAllocation)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// f.kubeactions is intentionally empty: no patch expected.
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

// newDualStackController builds a controller for the given dual-stack IPFamily.
func (f *fixture) newDualStackController(ctx context.Context, fam ipfamily.IPFamily) (*Controller, nsxinformers.SharedInformerFactory, kubeinformers.SharedInformerFactory) {
	f.client = nsxfake.NewSimpleClientset(f.objects...)
	f.kubeclient = k8sfake.NewSimpleClientset(f.kubeobjects...)

	i := nsxinformers.NewSharedInformerFactory(f.client, noResyncPeriodFunc())
	k8sI := kubeinformers.NewSharedInformerFactory(f.kubeclient, noResyncPeriodFunc())

	nodeInformer := k8sI.Core().V1().Nodes()
	ipAllocationInformer := i.Crd().V1alpha1().IPAddressAllocations()
	c := NewController(ctx, f.kubeclient, nodeInformer.Lister(), nodeInformer.Informer().HasSynced, ipAllocationInformer, fam, "test-cluster")
	c.ipAddressAllocationsSynced = alwaysReady
	c.nodesSynced = alwaysReady
	c.recorder = &record.FakeRecorder{}

	for _, a := range f.ipAddressAllocationsLister {
		ipAllocationInformer.Informer().GetIndexer().Add(a)
	}
	for _, d := range f.nodesLister {
		nodeInformer.Informer().GetIndexer().Add(d)
	}
	return c, i, k8sI
}

func (f *fixture) runDualStack(ctx context.Context, key string, fam ipfamily.IPFamily) {
	c, i, k8sI := f.newDualStackController(ctx, fam)
	i.Start(ctx.Done())
	k8sI.Start(ctx.Done())

	err := c.syncHandler(ctx, key)
	if err != nil {
		f.t.Errorf("error syncing IPAddressAllocation: %v", err)
	}
	util.CheckActions(f.actions, filterInformerActions(f.client.Actions()), f.t)
	util.CheckActions(f.kubeactions, filterInformerActions(f.kubeclient.Actions()), f.t)
}

func (f *fixture) runDualStackExpectError(ctx context.Context, key string, fam ipfamily.IPFamily) {
	c, i, k8sI := f.newDualStackController(ctx, fam)
	i.Start(ctx.Done())
	k8sI.Start(ctx.Done())

	err := c.syncHandler(ctx, key)
	if err == nil {
		f.t.Error("expected error syncing IPAddressAllocation, got nil")
	}
	util.CheckActions(f.actions, filterInformerActions(f.client.Actions()), f.t)
	util.CheckActions(f.kubeactions, filterInformerActions(f.kubeclient.Actions()), f.t)
}

// newDualStackPair returns IPv4 + IPv6 IPAddressAllocations for the given
// node, both labelled and with Spec.IPAddressType set so the controller's
// label/spec-based identification logic recognises them as a dual-stack
// pair (the same shape that nsxipmanager.createIPAddressAllocation produces).
func newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR string) (*vpcapisv1.IPAddressAllocation, *vpcapisv1.IPAddressAllocation) {
	ipv4 := newLabeledIPAddressAllocation(nodeName, nodeName, true, ipv4CIDR, corev1.ConditionTrue)
	ipv6 := newLabeledIPAddressAllocation(nodeName+helper.SuffixIPv6, nodeName, false, ipv6CIDR, corev1.ConditionTrue)
	return ipv4, ipv6
}

// TestDualStack_NoPatchWhenNodeAlreadyCorrect verifies that no API server PATCH is
// issued when the node's PodCIDRs already match the target CIDRs. This is the
// idempotency guard that prevents O(2N) redundant PATCHes per 30-second resync
// cycle in dual-stack clusters.
func TestDualStack_NoPatchWhenNodeAlreadyCorrect(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	// Node already has CIDRs in IPv4-primary order (matching ipfamily.IPv4IPv6).
	node := newNode(nodeName, ipv4CIDR, []string{ipv4CIDR, ipv6CIDR})

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// f.kubeactions is intentionally empty: no patch expected.
	f.runDualStack(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestDualStack_PatchWhenNodeOrderDiffers verifies that a PATCH is still issued
// when the node has both CIDRs but in the wrong order (e.g. node was previously
// set with IPv6-primary but the controller now runs with IPv4-primary).
func TestDualStack_PatchWhenNodeOrderDiffers(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	// Node has CIDRs in IPv6-first order, but controller is running IPv4-primary.
	node := newNode(nodeName, ipv6CIDR, []string{ipv6CIDR, ipv4CIDR})

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv4CIDR, ipv4CIDR, ipv6CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	f.runDualStack(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestDualStack_PatchNodeWhenBothPartnersReady verifies that the node is patched
// with both CIDRs in primary-family-first order when both partner allocations are ready.
func TestDualStack_PatchNodeWhenBothPartnersReady(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// ipfamily.IPv4IPv6 → PodCIDRs order: [ipv4, ipv6]
	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv4CIDR, ipv4CIDR, ipv6CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	f.runDualStack(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestDualStack_PatchNodeIPv6Primary verifies that IPv6 is listed first with ipfamily.IPv6IPv4.
func TestDualStack_PatchNodeIPv6Primary(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// ipfamily.IPv6IPv4 → PodCIDRs order: [ipv6, ipv4]
	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv6CIDR, ipv6CIDR, ipv4CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	f.runDualStack(ctx, getKey(ipv6Alloc, t), ipfamily.IPv6IPv4)
}

// TestDualStack_IPv6KeyIPv4Primary exercises triggered-by-IPv6 alloc with ipfamily.IPv4IPv6 (IPv4 primary)
// → PodCIDRs order [ipv4, ipv6] (partner=ipv4 is placed first).
func TestDualStack_IPv6KeyIPv4Primary(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// Triggered by IPv6 alloc, ipfamily.IPv4IPv6 → order [ipv4, ipv6]
	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv4CIDR, ipv4CIDR, ipv6CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	f.runDualStack(ctx, getKey(ipv6Alloc, t), ipfamily.IPv4IPv6)
}

// TestDualStack_IPv4KeyIPv6Primary exercises triggered-by-IPv4 alloc with ipfamily.IPv6IPv4 (IPv6 primary)
// → PodCIDRs order [ipv6, ipv4] (partner=ipv6 is placed first).
func TestDualStack_IPv4KeyIPv6Primary(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// Triggered by IPv4 alloc, ipfamily.IPv6IPv4 → order [ipv6, ipv4]
	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv6CIDR, ipv6CIDR, ipv4CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	f.runDualStack(ctx, getKey(ipv4Alloc, t), ipfamily.IPv6IPv4)
}

// TestDualStack_HoldWhenPartnerNotReady verifies that the node is NOT patched when the partner allocation is absent.
func TestDualStack_HoldWhenPartnerNotReady(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc := newLabeledIPAddressAllocation(nodeName, nodeName, true, ipv4CIDR, corev1.ConditionTrue)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc)
	f.objects = append(f.objects, ipv4Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	// No patch expected since partner IPv6 allocation doesn't exist yet.
	f.runDualStackExpectError(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestDualStack_HoldWhenPartnerCIDREmpty verifies that the node is NOT patched when the
// partner allocation exists but has not yet received a CIDR (status not ready).
func TestDualStack_HoldWhenPartnerCIDREmpty(t *testing.T) {
	nodeName := "node-1"
	ipv4CIDR := "10.244.0.0/24"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	ipv4Alloc := newLabeledIPAddressAllocation(nodeName, nodeName, true, ipv4CIDR, corev1.ConditionTrue)
	// IPv6 partner exists but condition is False → no CIDR yet.
	ipv6AllocNotReady := newLabeledIPAddressAllocation(nodeName+helper.SuffixIPv6, nodeName, false, "", corev1.ConditionFalse)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6AllocNotReady)
	f.objects = append(f.objects, ipv4Alloc, ipv6AllocNotReady)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	f.runDualStackExpectError(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestHelpers_IsIPv4Allocation verifies that family detection uses
// Spec.IPAddressType, not the CR name. This is the regression check for
// nodes whose names themselves end in "-ipv6".
func TestHelpers_IsIPv4Allocation(t *testing.T) {
	tests := []struct {
		name     string
		alloc    *vpcapisv1.IPAddressAllocation
		expected bool
	}{
		{
			name:     "explicit IPv4 spec",
			alloc:    &vpcapisv1.IPAddressAllocation{Spec: vpcapisv1.IPAddressAllocationSpec{IPAddressType: vpcapisv1.IPAllocationIPAddressTypeIPv4}},
			expected: true,
		},
		{
			name:     "explicit IPv6 spec",
			alloc:    &vpcapisv1.IPAddressAllocation{Spec: vpcapisv1.IPAddressAllocationSpec{IPAddressType: vpcapisv1.IPAllocationIPAddressTypeIPv6}},
			expected: false,
		},
		{
			name:     "unset IPAddressType defaults to IPv4 for legacy CRs",
			alloc:    &vpcapisv1.IPAddressAllocation{},
			expected: true,
		},
		{
			// Critical: a CR whose NAME ends in "-ipv6" but whose SPEC says
			// IPv4 (e.g. legacy IPv4 CR for a node literally named "worker-ipv6")
			// must still be classified as IPv4.
			name:     "name ends in -ipv6 but spec says IPv4",
			alloc:    &vpcapisv1.IPAddressAllocation{ObjectMeta: metav1.ObjectMeta{Name: "worker-ipv6"}, Spec: vpcapisv1.IPAddressAllocationSpec{IPAddressType: vpcapisv1.IPAllocationIPAddressTypeIPv4}},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isIPv4Allocation(tt.alloc); got != tt.expected {
				t.Errorf("isIPv4Allocation = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHelpers_NodeNameForAllocation verifies that node identity is taken from
// the nodeName label when present, and falls back to suffix-stripping only
// for legacy single-stack IPv4 CRs (where the bare-name strip is a no-op).
func TestHelpers_NodeNameForAllocation(t *testing.T) {
	tests := []struct {
		name     string
		alloc    *vpcapisv1.IPAddressAllocation
		expected string
	}{
		{
			name: "label is authoritative — node literally ends in -ipv6",
			alloc: &vpcapisv1.IPAddressAllocation{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "worker-ipv6" + helper.SuffixIPv6,
					Labels: map[string]string{helper.LabelKeyNodeName: "worker-ipv6"},
				},
			},
			expected: "worker-ipv6",
		},
		{
			name: "label wins over name parsing",
			alloc: &vpcapisv1.IPAddressAllocation{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "node-1" + helper.SuffixIPv6,
					Labels: map[string]string{helper.LabelKeyNodeName: "node-1"},
				},
			},
			expected: "node-1",
		},
		{
			name:     "legacy IPv4 CR (no label) — bare name, strip is a no-op",
			alloc:    &vpcapisv1.IPAddressAllocation{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
			expected: "node-1",
		},
		{
			name:     "empty label value falls back",
			alloc:    &vpcapisv1.IPAddressAllocation{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: map[string]string{helper.LabelKeyNodeName: ""}}},
			expected: "node-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodeNameForAllocation(tt.alloc); got != tt.expected {
				t.Errorf("nodeNameForAllocation = %q, want %q", got, tt.expected)
			}
		})
	}
}

// TestHelpers_PartnerCRNameFor confirms the canonical CR-naming scheme is
// correct, especially for nodes whose names themselves end in "-ipv6".
// TestDualStack_NodeNameEndingInIPv6 is the end-to-end regression test for
// the most subtle bug fixed by the label-based identification: a node whose
// name literally ends in "-ipv6" must still get both CIDRs patched.
//
// Before the fix, the controller would parse the IPv4 CR name "worker-ipv6"
// as "an IPv6 CR for node 'worker'" and look up the wrong node / wrong
// partner.
func TestDualStack_NodeNameEndingInIPv6(t *testing.T) {
	nodeName := "worker-ipv6"
	ipv4CIDR := "10.244.0.0/24"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	_, ctx := ktesting.NewTestContext(t)

	// IPv4 CR for this node uses the bare node name (which itself ends in -ipv6).
	// IPv6 partner appends another -ipv6 suffix.
	ipv4Alloc, ipv6Alloc := newDualStackPair(nodeName, ipv4CIDR, ipv6CIDR)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv4Alloc, ipv6Alloc)
	f.objects = append(f.objects, ipv4Alloc, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s","%s"]}}`, ipv4CIDR, ipv4CIDR, ipv6CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	// Trigger by the IPv4 CR to exercise the partner-lookup path with a node
	// name that would have been mis-parsed before the fix.
	f.runDualStack(ctx, getKey(ipv4Alloc, t), ipfamily.IPv4IPv6)
}

// TestSingleStack_IPv6Only verifies that IPv6-only mode patches the node from
// an IPv6 CR (whose name ends in -ipv6) without needing a partner lookup.
func TestSingleStack_IPv6Only(t *testing.T) {
	nodeName := "node-1"
	ipv6CIDR := "fd00::/80"
	f := newFixture(t)
	f.ipFamily = ipfamily.IPv6
	_, ctx := ktesting.NewTestContext(t)

	ipv6Alloc := newLabeledIPAddressAllocation(nodeName+helper.SuffixIPv6, nodeName, false, ipv6CIDR, corev1.ConditionTrue)
	node := newNode(nodeName, "", nil)

	f.ipAddressAllocationsLister = append(f.ipAddressAllocationsLister, ipv6Alloc)
	f.objects = append(f.objects, ipv6Alloc)
	f.nodesLister = append(f.nodesLister, node)
	f.kubeobjects = append(f.kubeobjects, node)

	patch := []byte(fmt.Sprintf(`{"spec":{"podCIDR":"%s","podCIDRs":["%s"]}}`, ipv6CIDR, ipv6CIDR))
	f.expectPatchNodeAction(nodeName, types.StrategicMergePatchType, patch)

	// Single-stack: ipfamily.IPv4, partner lookup is skipped.
	f.run(ctx, getKey(ipv6Alloc, t))
}
