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
	t1networkingapis "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	faket1networkingclients "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/helper"
	ippmv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/v1alpha1"
	"k8s.io/klog/v2"
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

func newController() (*Controller, *fake.Clientset) {
	kubeClient := fake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})

	// testing with non-vpc mode
	ippoolclientset := faket1networkingclients.NewSimpleClientset()
	ippManager, _ := ippmv1alpha1.NewIPPoolManagerWithClients(ippoolclientset, testClusterNS)

	c := &Controller{
		kubeclientset: kubeClient,
		ippoolManager: ippManager,

		recorder:  recorder,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "IPPools"),
	}

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
		ippool             t1networkingapis.IPPool
		nodes              []corev1.Node
		subnetAllocated    []t1networkingapis.SubnetResult
		subnetAfterRevoked []t1networkingapis.SubnetResult
		expectedNumPatches int
	}{
		{
			desc:   "update 2 nodes' cidr, then 2 nodes are removed",
			ippool: testIPPool,
			nodes:  []corev1.Node{n1, n2, n3},
			subnetAllocated: []t1networkingapis.SubnetResult{
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
			subnetAllocated: []t1networkingapis.SubnetResult{
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
			subnetAllocated: []t1networkingapis.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
				{
					Name: n2.Name,
					CIDR: "",
				},
			},
			subnetAfterRevoked: []t1networkingapis.SubnetResult{
				{
					Name: n1.Name,
					CIDR: "10.0.0.1/24",
				},
			},
			expectedNumPatches: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			c, cs := newController()
			if _, err := c.ippoolManager.CreateIPPool(testClusterNS, testClusterName, &metav1.OwnerReference{}); err != nil {
				t.Errorf("failed to create test ippool %s: %v", testIPPool.Name, err)
			}

			subs, _ := c.ippoolManager.GetIPPoolSubnets(&testIPPool)
			if err := c.processIPPoolCreateOrUpdate(subs); err != nil {
				t.Errorf("failed to processIPPoolCreateOrUpdate %v: %v", testIPPool, err)
			}

			for _, n := range tc.nodes {
				if _, err := c.kubeclientset.CoreV1().Nodes().Create(context.Background(), &n, metav1.CreateOptions{}); err != nil {
					t.Errorf("failed to create test node %s: %v", n.Name, err)
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
func updateIPPoolCIDRAndVerifyNodeCIDR(srs []t1networkingapis.SubnetResult, c *Controller) error {
	// add subnet allocation result updates to ippool
	ippoolUpdatedSubnet := createIPPool(srs)
	subs, _ := c.ippoolManager.GetIPPoolSubnets(ippoolUpdatedSubnet)
	if err := c.processIPPoolCreateOrUpdate(subs); err != nil {
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
func createIPPool(srs []t1networkingapis.SubnetResult) *t1networkingapis.IPPool {
	ippool := &t1networkingapis.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.IppoolNameFromClusterName(testClusterName),
			Namespace: testClusterNS,
		},
		Status: t1networkingapis.IPPoolStatus{
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
