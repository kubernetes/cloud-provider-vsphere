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

package ippool

import (
	"context"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ippoolv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	fakeippoolclientset "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	ippoolscheme "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/scheme"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/helper"

	klog "k8s.io/klog/v2"
)

const (
	testClusterNS   = "ns"
	testClusterName = "n"
)

var (
	n1Name = "name1"
	n2Name = "name2"
	n3Name = "name3"
	n1     = createNode(n1Name)
	n2     = createNode(n2Name)
	n3     = createNode(n3Name)
)

func alwaysReady() bool { return true }

func newController() (*Controller, *fake.Clientset) {
	kubeClient := fake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	recorder := eventBroadcaster.NewRecorder(ippoolscheme.Scheme, corev1.EventSource{Component: controllerName})

	ippoolclientset := fakeippoolclientset.NewSimpleClientset()

	c := &Controller{
		kubeclientset:   kubeClient,
		ippoolclientset: ippoolclientset,

		recorder:  recorder,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "IPPools"),
	}

	c.ippoolListerSynced = alwaysReady
	c.recorder = record.NewFakeRecorder(100)
	ippoolclientset.ClearActions()
	kubeClient.ClearActions()

	return c, kubeClient
}

func TestProcessIPPoolCreateOrUpdate(t *testing.T) {
	var (
		testIPPool = *createIPPool(nil)
	)
	testCases := []struct {
		desc               string
		ippool             ippoolv1alpha1.IPPool
		nodes              []corev1.Node
		subnetAllocated    []ippoolv1alpha1.SubnetResult
		subnetAfterRevoked []ippoolv1alpha1.SubnetResult
		expectedNumPatches int
	}{
		{
			desc:   "update 2 nodes' cidr, then 2 nodes are removed",
			ippool: testIPPool,
			nodes:  []corev1.Node{n1, n2, n3},
			subnetAllocated: []ippoolv1alpha1.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
				{
					Name: n2.Name,
					CIDR: "10.0.0.2/24",
				},
			},
			subnetAfterRevoked: nil,
			expectedNumPatches: 2,
		},
		{
			desc:   "update 1 nodes' cidr, then 1 node are revoved",
			ippool: testIPPool,
			nodes:  []corev1.Node{n1},
			subnetAllocated: []ippoolv1alpha1.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
			},
			subnetAfterRevoked: nil,
			expectedNumPatches: 1,
		},
		{
			desc:   "ippool exhausted, updates 1 node, the 2 node didn't get ip. then node 2 was removed",
			ippool: testIPPool,
			nodes:  []corev1.Node{n1, n2, n3},
			subnetAllocated: []ippoolv1alpha1.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
				{
					Name: n2.Name,
					CIDR: "",
				},
			},
			subnetAfterRevoked: []ippoolv1alpha1.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
			},
			expectedNumPatches: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s := scheme.Scheme
			if err := ippoolscheme.AddToScheme(s); err != nil {
				t.Fatalf("Unable to add route scheme: (%v)", err)
			}

			c, cs := newController()

			if _, err := c.ippoolclientset.NsxV1alpha1().IPPools(testClusterNS).Create(context.Background(), &testIPPool, metav1.CreateOptions{}); err != nil {
				t.Errorf("failed to create test ippool %s: %w", testIPPool.Name, err)
			}

			if err := c.processIPPoolCreateOrUpdate(&testIPPool); err != nil {
				t.Errorf("failed to processIPPoolCreateOrUpdate %v: %w", testIPPool, err)
			}

			for _, n := range tc.nodes {
				if _, err := c.kubeclientset.CoreV1().Nodes().Create(context.Background(), &n, metav1.CreateOptions{}); err != nil {
					t.Errorf("failed to create test node %s: %w", n.Name, err)
				}
			}

			if err := updateIPPoolCIDRAndVerifyNodeCIDR(tc.subnetAllocated, c); err != nil {
				t.Errorf(err.Error())
			}

			if err := updateIPPoolCIDRAndVerifyNodeCIDR(tc.subnetAfterRevoked, c); err != nil {
				t.Errorf(err.Error())
			}

			// verify the number of patch request to nodes to update cidr section
			actions := cs.Actions()
			numPatches := 0
			for _, a := range actions {
				if a.Matches("patch", "nodes") {
					numPatches++
				}
			}
			if tc.expectedNumPatches != numPatches {
				t.Errorf("expectedPatch %d doesn't match number of patches %d", tc.expectedNumPatches, numPatches)
			}

		})
	}
}
func updateIPPoolCIDRAndVerifyNodeCIDR(srs []ippoolv1alpha1.SubnetResult, c *Controller) error {
	// add subnet allocation result updates to ippool
	ippoolUpdatedSubnet := createIPPool(srs)
	if err := c.processIPPoolCreateOrUpdate(ippoolUpdatedSubnet); err != nil {
		return fmt.Errorf("failed to processIPPoolCreateOrUpdate %v: %w", ippoolUpdatedSubnet, err)
	}

	// check node's cidr
	var updatedNodes *corev1.NodeList
	var err error
	if updatedNodes, err = c.kubeclientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{}); err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	nm := make(map[string]string)
	for _, n := range updatedNodes.Items {
		nm[n.Name] = n.Spec.PodCIDR
		if n.Spec.PodCIDR != "" && len(n.Spec.PodCIDRs) == 0 || len(n.Spec.PodCIDRs) > 0 && n.Spec.PodCIDR != n.Spec.PodCIDRs[0] {
			return fmt.Errorf("n.Spec.PodCIDR doesn't match n.Spec.PodCIDRs: %w", err)
		}
	}
	for _, s := range srs {
		if _, ok := nm[s.Name]; !ok {
			return fmt.Errorf("node not created %s", s.Name)
		}
		if nm[s.Name] != s.CIDR {
			return fmt.Errorf("node %s CIDR didn't get updated to %s ", s.Name, nm[s.Name])
		}
	}

	return nil
}
func createIPPool(srs []ippoolv1alpha1.SubnetResult) *ippoolv1alpha1.IPPool {
	ippool := &ippoolv1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.IppoolNameFromClusterName(testClusterName),
			Namespace: testClusterNS,
		},
		Status: ippoolv1alpha1.IPPoolStatus{
			Subnets: srs,
		},
	}

	return ippool
}

func createNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}
