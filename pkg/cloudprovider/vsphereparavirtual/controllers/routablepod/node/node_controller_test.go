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

package node

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
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
	n1     = createNode(n1Name)
	n2     = createNode(n2Name)
)

func alwaysReady() bool { return true }

func newController() (*Controller, *faket1networkingclients.Clientset) {
	kubeClient := fake.NewSimpleClientset()
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})

	// testing with non-vpc mode
	ippoolclientset := faket1networkingclients.NewSimpleClientset()
	ippManager, _ := ippmv1alpha1.NewIPPoolManagerWithClients(ippoolclientset, testClusterNS)

	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := informerFactory.Core().V1().Nodes()

	c := &Controller{
		ippoolManager:    ippManager,
		nodesLister:      nodeInformer.Lister(),
		nodeListerSynced: nodeInformer.Informer().HasSynced,

		recorder:  recorder,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Nodes"),

		clusterName: testClusterName,
		clusterNS:   testClusterNS,
	}

	c.nodeListerSynced = alwaysReady
	c.recorder = record.NewFakeRecorder(100)
	c.ownerRef = &metav1.OwnerReference{}
	ippoolclientset.ClearActions()

	return c, ippoolclientset
}

func TestProcessNodeCreateOrUpdate(t *testing.T) {
	testCases := []struct {
		desc               string
		nodes              []corev1.Node
		nodesUpdate        []corev1.Node
		expectedNumNodes   int
		expectedNumPatches int
		expectIPPool       bool
	}{
		{
			desc:               "create 2 nodes",
			nodes:              []corev1.Node{n1, n2},
			nodesUpdate:        []corev1.Node{},
			expectedNumNodes:   2,
			expectedNumPatches: 2,
			expectIPPool:       true,
		},
		{
			desc:               "create 2 node, update 2 node",
			nodes:              []corev1.Node{n1, n2},
			nodesUpdate:        []corev1.Node{n1, n2},
			expectedNumNodes:   2,
			expectedNumPatches: 2,
			expectIPPool:       true,
		},
		{
			desc:               "create 1 node",
			nodes:              []corev1.Node{n1},
			nodesUpdate:        []corev1.Node{},
			expectedNumNodes:   1,
			expectedNumPatches: 1,
			expectIPPool:       true,
		},
		{
			desc:               "create 1 node, update 1 node",
			nodes:              []corev1.Node{n1},
			nodesUpdate:        []corev1.Node{n1},
			expectedNumNodes:   1,
			expectedNumPatches: 1,
			expectIPPool:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ippc, ippcs := newController()
			// create nodes and run process processNodeCreateOrUpdate
			for _, n := range tc.nodes {
				if err := ippc.processNodeCreateOrUpdate(&n); err != nil {
					t.Errorf("failed to create test node %s: %v", n.Name, err)
				}
			}
			for _, n := range tc.nodesUpdate {
				if err := ippc.processNodeCreateOrUpdate(&n); err != nil {
					t.Errorf("failed to create test node %s: %v", n.Name, err)
				}
			}

			// verify the number of patch request to ippool to update spec section
			actions := ippcs.Actions()
			numPatches := 0
			for _, a := range actions {
				if a.Matches("update", "ippools") {
					numPatches++
				}
			}
			if tc.expectedNumPatches != numPatches {
				t.Errorf("expectedPatch %d doesn't match number of patches %d", tc.expectedNumPatches, numPatches)
			}

			// verify ippool
			ippool, err := ippcs.NsxV1alpha1().IPPools(testClusterNS).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("failed to list ippool: %v", err)
			}
			if tc.expectIPPool {
				if len(ippool.Items) != 1 {
					t.Errorf("expect ippool to be created but not")
				}
			} else {
				if len(ippool.Items) != 0 {
					t.Errorf("expect ippool not to be created")
				}
			}

			// verify the request in ippool spec
			if tc.expectIPPool {
				ipp := ippool.Items[0]
				sm := make(map[string]struct{})
				for _, s := range ipp.Spec.Subnets {
					sm[s.Name] = struct{}{}
				}
				if len(sm) != tc.expectedNumNodes {
					t.Errorf("number of request %d doesn match number of nodes %d", len(ipp.Spec.Subnets), tc.expectedNumNodes)
				}
				for _, n := range tc.nodes {
					// process each node request
					if _, ok := sm[n.Name]; !ok {
						t.Errorf("node '%s' request doesn't exist in ippool spec", n.Name)
					}
				}
			}
		})
	}
}

func TestProcessNodeDelete(t *testing.T) {
	testCases := []struct {
		desc               string
		nodes              []corev1.Node
		nodesToBeDeleted   []corev1.Node
		expectedNumPatches int
		ippoolExist        bool
	}{
		{
			desc:               "create 2 nodes, delete 2",
			nodes:              []corev1.Node{n1, n2},
			nodesToBeDeleted:   []corev1.Node{n1, n2},
			expectedNumPatches: 2,
			ippoolExist:        true,
		},
		{
			desc:               "create 2 nodes, delete 1",
			nodes:              []corev1.Node{n1, n2},
			nodesToBeDeleted:   []corev1.Node{n1},
			expectedNumPatches: 1,
			ippoolExist:        true,
		},
		{
			desc:               "create 1 nodes, delete 1, ippool doesn't exist",
			nodes:              []corev1.Node{n1},
			nodesToBeDeleted:   []corev1.Node{n1},
			expectedNumPatches: 0,
			ippoolExist:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ippc, ippcs := newController()

			if tc.ippoolExist {
				// pre create test nodes
				if _, err := createIPPool(ippcs, tc.nodes); err != nil {
					t.Errorf("failed to create ippool %v", err)
				}
			}

			// delete node
			for _, n := range tc.nodesToBeDeleted {
				if err := ippc.processNodeDelete(n.Name); err != nil {
					t.Errorf("failed to create test node %s: %v", n.Name, err)
				}
			}

			// verify the number of patch request to ippool to update spec section
			actions := ippcs.Actions()
			numPatches := 0
			for _, a := range actions {
				if a.Matches("update", "ippools") {
					numPatches++
				}
			}
			if tc.expectedNumPatches != numPatches {
				t.Errorf("expectedPatch %d doesn't match number of patches %d", tc.expectedNumPatches, numPatches)
			}

			// skip following test if ippool doesn't exist
			if !tc.ippoolExist {
				return
			}

			// verify ippool
			ippool, err := ippcs.NsxV1alpha1().IPPools(testClusterNS).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				t.Errorf("failed to list ippool: %v", err)
			}
			if len(ippool.Items) < 1 {
				if err != nil {
					t.Errorf("expected to have 1 ippool")
				}
			}
			// verify the request in ippool spec
			ipp := ippool.Items[0]
			sm := make(map[string]struct{})
			for _, s := range ipp.Spec.Subnets {
				sm[s.Name] = struct{}{}
			}
			if len(sm) != len(tc.nodes)-len(tc.nodesToBeDeleted) {
				t.Errorf("number of requests %d doesn't match number of left nodes %d", len(ipp.Spec.Subnets), len(tc.nodes)-len(tc.nodesToBeDeleted))
			}
			for _, n := range tc.nodesToBeDeleted {
				// process each node request
				if _, ok := sm[n.Name]; ok {
					t.Errorf("node '%s' shouldn't be in request", n.Name)
				}
			}
		})
	}
}

func createNode(name string) corev1.Node {
	return corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func createIPPool(ippcs *faket1networkingclients.Clientset, nodes []corev1.Node) (*t1networkingapis.IPPool, error) {
	ipp := &t1networkingapis.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.IppoolNameFromClusterName(testClusterName),
			Namespace: testClusterNS,
		},
		Spec: t1networkingapis.IPPoolSpec{
			Subnets: []t1networkingapis.SubnetRequest{},
		},
	}

	for _, n := range nodes {
		ipp.Spec.Subnets = append(ipp.Spec.Subnets, t1networkingapis.SubnetRequest{
			Name:         n.Name,
			IPFamily:     helper.IPFamilyDefault,
			PrefixLength: helper.PrefixLengthDefault,
		})
	}

	return ippcs.NsxV1alpha1().IPPools(testClusterNS).Create(context.Background(), ipp, metav1.CreateOptions{})
}
