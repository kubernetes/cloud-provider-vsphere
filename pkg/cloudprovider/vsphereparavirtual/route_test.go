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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	t1networkingapis "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	faket1networkingclients "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/routeset"
	"k8s.io/cloud-provider-vsphere/pkg/util"
)

const (
	testNodeIP   = "172.50.0.13"
	testCIDR     = "100.96.0.0/24"
	testNodeName = "fakeNode1"
	testNameHint = "62d347a4-1b70-435e-b92a-9a61453843ee"
)

func initRouteTest() (*routesProvider, *util.FakeRouteSetClientWrapper, *faket1networkingclients.Clientset) {
	// create the fake client
	// test with non-vpc mode
	fc := faket1networkingclients.NewSimpleClientset()
	fcw := util.NewFakeRouteSetClientWrapper(fc)

	routeManager, _ := routeset.NewRouteManagerWithClients(fc, testClusterNameSpace)

	routesProvider := &routesProvider{
		routeManager: routeManager,
		nodeMap:      make(map[string]*v1.Node),
		ownerRefs:    []metav1.OwnerReference{},
	}
	return routesProvider, fcw, fc
}

func buildFakeNode(nodeName string) *v1.Node {
	addresses := make([]v1.NodeAddress, 2)
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: nodeName})
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: testNodeIP})
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: "fe80::20c:29ff:fe0b:b407"})
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: v1.NodeStatus{
			Addresses: addresses,
		},
	}
	return node
}

func buildFakeRouteInfo(clusterName, nameHint, dstCIDR, nodeName, nodeIP string) *helper.RouteInfo {
	labels := map[string]string{
		helper.LabelKeyClusterName: clusterName,
	}
	nodeRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       nodeName,
		UID:        types.UID(nameHint),
	}
	routeInfo := &helper.RouteInfo{
		Labels:    labels,
		Owner:     []metav1.OwnerReference{nodeRef},
		Name:      nodeName,
		Cidr:      dstCIDR,
		NodeIP:    nodeIP,
		RouteName: helper.GetRouteName(nodeName, dstCIDR, clusterName),
	}
	return routeInfo
}

// createFakeRouteSetCR creates RouteSet CR with 'Ready' realized status
func createFakeRouteSetCR(fc *faket1networkingclients.Clientset, clusterName string, nameHint string, nodeName string, cidr string, nodeIP string) (*t1networkingapis.RouteSet, error) {
	labels := map[string]string{
		helper.LabelKeyClusterName: clusterName,
	}
	route := t1networkingapis.Route{
		Name:        helper.GetRouteName(nodeName, cidr, clusterName),
		Destination: cidr,
		Target:      nodeIP,
	}
	routeSetSpec := t1networkingapis.RouteSetSpec{
		Routes: []t1networkingapis.Route{
			route,
		},
	}
	routeSet := &t1networkingapis.RouteSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: testClusterNameSpace,
			Labels:    labels,
		},
		Spec: routeSetSpec,
		Status: t1networkingapis.RouteSetStatus{
			Conditions: []t1networkingapis.RouteSetCondition{
				{
					Type:    t1networkingapis.RouteSetConditionTypeReady,
					Status:  v1.ConditionTrue,
					Reason:  "RouteSetCreated",
					Message: "RouteSet CR created",
				},
			},
		},
	}

	routeSet, err := fc.NsxV1alpha1().RouteSets(testClusterNameSpace).Create(context.TODO(), routeSet, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return routeSet, nil
}

func TestListRoutes(t *testing.T) {
	r, _, fc := initRouteTest()

	// create 3 fake Routes
	fakeNode1 := buildFakeNode("fakeNode1")
	r.AddNode(fakeNode1)
	fakeRouteInfo1 := buildFakeRouteInfo(testClustername, testNameHint, "100.96.0.0/24", "fakeNode1", testNodeIP)
	routeSet1, err := r.routeManager.CreateRouteCR(context.TODO(), fakeRouteInfo1)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet1, nil)
	fakeNode2 := buildFakeNode("fakeNode2")
	r.AddNode(fakeNode2)
	routeSet2, err := createFakeRouteSetCR(fc, testClustername, testNameHint, "fakeNode2", "100.96.1.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet2, nil)
	fakeNode3 := buildFakeNode("fakeNode3")
	r.AddNode(fakeNode3)
	fakeRouteInfo3 := buildFakeRouteInfo("another-cluster-name", testNameHint, "100.96.2.0/24", "fakeNode3", testNodeIP)
	routeSet3, err := r.routeManager.CreateRouteCR(context.TODO(), fakeRouteInfo3)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet3, nil)

	routes, err := r.ListRoutes(context.TODO(), testClustername)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(routes), "Should have 1 route as the realized status of routeSet1 is not updated and routeSet3 belongs to another cluster")

	// clean up
	err = r.routeManager.DeleteRouteCR(fakeNode1.Name)
	assert.NoError(t, err)
	err = r.routeManager.DeleteRouteCR(fakeNode2.Name)
	assert.NoError(t, err)
	err = r.routeManager.DeleteRouteCR(fakeNode3.Name)
	assert.NoError(t, err)
}

func TestListRoutesFailed(t *testing.T) {
	r, fcw, _ := initRouteTest()
	fcw.ListFunc = func(ctx context.Context, opts metav1.ListOptions) (result *t1networkingapis.RouteSetList, err error) {
		return nil, fmt.Errorf(helper.ErrListRouteCR.Error())
	}

	routes, err := r.ListRoutes(context.TODO(), testClustername)
	assert.Equal(t, 0, len(routes))
	if err != nil {
		assert.Equal(t, helper.ErrListRouteCR.Error(), err.Error())
	}
}

func TestCreateRouteFailed(t *testing.T) {
	r, _, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            helper.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}

	err := r.CreateRoute(context.TODO(), testClustername, testNameHint, &route)
	assert.NotEqual(t, err, nil)
	// expect the timeout error as the realized status is not updated
	assert.Equal(t, "timed out waiting for static route fakeNode1", err.Error())

	// clean up
	err = r.routeManager.DeleteRouteCR(node.Name)
	assert.NoError(t, err)
}

func TestCreateRouteFailedWithAlreadyExisting(t *testing.T) {
	r, _, fc := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	routeSetCR, err := createFakeRouteSetCR(fc, testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	route := cloudprovider.Route{
		Name:            helper.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}
	err = r.CreateRoute(context.TODO(), testClustername, testNameHint, &route)
	if err != nil {
		// expect 'route already exists' error
		assert.Equal(t, "routesets.nsx.vmware.com \"fakeNode1\" already exists", err.Error())
	}

	// clean up
	err = r.routeManager.DeleteRouteCR(node.Name)
	assert.NoError(t, err)
}

func TestDeleteRoute(t *testing.T) {
	r, _, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            helper.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeSetCR, err := r.routeManager.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	// clean up
	err = r.DeleteRoute(context.TODO(), testClustername, &route)
	assert.NoError(t, err)
}

func TestDeleteRouteFailed(t *testing.T) {
	r, fcw, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            helper.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}

	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeSetCR, err := r.routeManager.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	fcw.DeleteFunc = func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return fmt.Errorf(helper.ErrDeleteRouteCR.Error())
	}
	err = r.DeleteRoute(context.TODO(), testClustername, &route)
	if err != nil {
		assert.Equal(t, helper.ErrDeleteRouteCR.Error(), err.Error())
	}
}

func TestAddNode(t *testing.T) {
	r := &routesProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	node := buildFakeNode(testNodeName)
	r.AddNode(node)
	assert.Equal(t, node, r.nodeMap[testNodeName])
}

func TestDeleteNode(t *testing.T) {
	r, _, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	r.DeleteNode(node)
	assert.Equal(t, (*v1.Node)(nil), r.nodeMap[testNodeName])
}

func TestGetNode(t *testing.T) {
	r := &routesProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	nodeInMap, err := r.getNode(testNodeName)
	assert.Equal(t, node, nodeInMap)
	assert.NoError(t, err)
}

func TestCheckStaticRouteRealizedState(t *testing.T) {
	r, _, fc := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeSet1, err := r.routeManager.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet1, nil)

	err = r.checkStaticRouteRealizedState(testNodeName)
	assert.NotEqual(t, err, nil)
	assert.Equal(t, "timed out waiting for static route fakeNode1", err.Error())

	fakeNode2 := buildFakeNode("fakeNode2")
	r.AddNode(fakeNode2)
	routeSet2, err := createFakeRouteSetCR(fc, testClustername, testNameHint, "fakeNode2", "100.96.1.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet2, nil)
	err = r.checkStaticRouteRealizedState(routeSet2.Name)
	assert.NoError(t, err)

	// clean up
	err = r.routeManager.DeleteRouteCR(node.Name)
	assert.NoError(t, err)
	err = r.routeManager.DeleteRouteCR(fakeNode2.Name)
	assert.NoError(t, err)
}
