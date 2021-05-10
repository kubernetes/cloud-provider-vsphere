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
	"k8s.io/client-go/kubernetes/scheme"
	cloudprovider "k8s.io/cloud-provider"
	v1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	fakeClient "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	"k8s.io/cloud-provider-vsphere/pkg/util"
)

const (
	testNodeIP   = "172.50.0.13"
	testCIDR     = "100.96.0.0/24"
	testNodeName = "fakeNode1"
	testNameHint = "62d347a4-1b70-435e-b92a-9a61453843ee"
)

func createTestRouteSet() *v1alpha1.RouteSet {
	return &v1alpha1.RouteSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-name",
			Namespace: testClusterNameSpace,
		},
		Status: v1alpha1.RouteSetStatus{
			Conditions: []v1alpha1.RouteSetCondition{
				{
					Type:    v1alpha1.RouteSetConditionTypeReady,
					Status:  v1.ConditionTrue,
					Reason:  "RouteSetCreated",
					Message: "RouteSet CR created",
				},
			},
		},
	}
}

func initRouteTest() (*routesProvider, *util.FakeRouteSetClientWrapper) {
	testRouteSet := createTestRouteSet()
	v1alpha1.AddToScheme(scheme.Scheme)

	// create the fake client
	fc := fakeClient.NewSimpleClientset(testRouteSet)
	fcw := util.NewFakeRouteSetClientWrapper(fc)

	routesProvider := &routesProvider{
		routeClient: fcw,
		namespace:   testClusterNameSpace,
		nodeMap:     make(map[string]*v1.Node),
		ownerRefs:   []metav1.OwnerReference{},
	}
	return routesProvider, fcw
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

// createFakeRouteSetCR creates RouteSet CR with 'Ready' realized status
func createFakeRouteSetCR(r *routesProvider, clusterName string, nameHint string, nodeName string, cidr string, nodeIP string) (*v1alpha1.RouteSet, error) {
	labels := map[string]string{
		LabelKeyClusterName: clusterName,
	}
	route := v1alpha1.Route{
		Name:        r.GetRouteName(nodeName, cidr, clusterName),
		Destination: cidr,
		Target:      nodeIP,
	}
	routeSetSpec := v1alpha1.RouteSetSpec{
		Routes: []v1alpha1.Route{
			route,
		},
	}
	routeSet := &v1alpha1.RouteSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeName,
			Namespace: r.namespace,
			Labels:    labels,
		},
		Spec: routeSetSpec,
		Status: v1alpha1.RouteSetStatus{
			Conditions: []v1alpha1.RouteSetCondition{
				{
					Type:    v1alpha1.RouteSetConditionTypeReady,
					Status:  v1.ConditionTrue,
					Reason:  "RouteSetCreated",
					Message: "RouteSet CR created",
				},
			},
		},
	}

	routeSet, err := r.routeClient.NsxV1alpha1().RouteSets(r.namespace).Create(context.TODO(), routeSet, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return routeSet, nil
}

func TestGetRouteName(t *testing.T) {
	r, _ := initRouteTest()
	name := r.GetRouteName(testNodeName, testCIDR, testClustername)
	expectedName := testNodeName + "-100.96.0.0-24-" + testClustername
	assert.Equal(t, name, expectedName)
}

func TestListRoutes(t *testing.T) {
	r, _ := initRouteTest()

	// create 3 fake Routes
	fakeNode1 := buildFakeNode("fakeNode1")
	r.AddNode(fakeNode1)
	routeSet1, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, "fakeNode1", "100.96.0.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet1, nil)
	fakeNode2 := buildFakeNode("fakeNode2")
	r.AddNode(fakeNode2)
	routeSet2, err := createFakeRouteSetCR(r, testClustername, testNameHint, "fakeNode2", "100.96.1.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet2, nil)
	fakeNode3 := buildFakeNode("fakeNode3")
	r.AddNode(fakeNode3)
	routeSet3, err := r.createRouteSetCR(context.TODO(), "another-cluster-name", testNameHint, "fakeNode3", "100.96.2.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet3, nil)

	routes, err := r.ListRoutes(context.TODO(), testClustername)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(routes), "Should have 1 route as the realized status of routeSet1 is not updated and routeSet3 belongs to another cluster")

	// clean up
	err = r.DeleteRouteSetCR(fakeNode1.Name)
	assert.NoError(t, err)
	err = r.DeleteRouteSetCR(fakeNode2.Name)
	assert.NoError(t, err)
	err = r.DeleteRouteSetCR(fakeNode3.Name)
	assert.NoError(t, err)
}

func TestListRoutesFailed(t *testing.T) {
	r, fcw := initRouteTest()
	fcw.ListFunc = func(ctx context.Context, opts metav1.ListOptions) (result *v1alpha1.RouteSetList, err error) {
		return nil, fmt.Errorf(ErrListRouteSet.Error())
	}

	routes, err := r.ListRoutes(context.TODO(), testClustername)
	assert.Equal(t, 0, len(routes))
	if err != nil {
		assert.Equal(t, ErrListRouteSet.Error(), err.Error())
	}
}

func TestCreateRouteFailed(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            r.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}

	err := r.CreateRoute(context.TODO(), testClustername, testNameHint, &route)
	assert.NotEqual(t, err, nil)
	// expect the timeout error as the realized status is not updated
	assert.Equal(t, "timed out waiting for static route fakeNode1", err.Error())

	// clean up
	err = r.DeleteRouteSetCR(node.Name)
	assert.NoError(t, err)
}

func TestCreateRouteSetCR(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	routeSetCR, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	expectedRouteSetSpec := v1alpha1.RouteSetSpec{
		Routes: []v1alpha1.Route{
			{
				Name:        r.GetRouteName(testNodeName, testCIDR, testClustername),
				Destination: testCIDR,
				Target:      testNodeIP,
			},
		},
	}
	expectedLabels := map[string]string{
		LabelKeyClusterName: testClustername,
	}
	expectedOwnerRefs := []metav1.OwnerReference{
		{
			APIVersion: "v1",
			Kind:       "Node",
			Name:       testNodeName,
			UID:        types.UID(testNameHint),
		},
	}

	assert.Equal(t, testNodeName, routeSetCR.Name)
	assert.Equal(t, r.namespace, routeSetCR.Namespace)
	assert.Equal(t, expectedRouteSetSpec, routeSetCR.Spec)
	assert.Equal(t, expectedOwnerRefs, routeSetCR.OwnerReferences)
	assert.Equal(t, expectedLabels, routeSetCR.Labels)

	// clean up
	err = r.DeleteRouteSetCR(node.Name)
	assert.NoError(t, err)
}

func TestCreateRouteSetCRFailed(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	routeSetCR, err := createFakeRouteSetCR(r, testClustername, testNameHint, testNodeName, "100.96.1.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	route := cloudprovider.Route{
		Name:            r.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}
	err = r.CreateRoute(context.TODO(), testClustername, testNameHint, &route)
	if err != nil {
		// expect 'route already exists' error
		assert.Equal(t, "routesets.nsx.vmware.com \"fakeNode1\" already exists", err.Error())
	}

	// clean up
	err = r.DeleteRouteSetCR(node.Name)
	assert.NoError(t, err)
}

func TestDeleteRoute(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            r.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}

	routeSetCR, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	// clean up
	err = r.DeleteRoute(context.TODO(), testClustername, &route)
	assert.NoError(t, err)
}

func TestDeleteRouteFailed(t *testing.T) {
	r, fcw := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	route := cloudprovider.Route{
		Name:            r.GetRouteName(testNodeName, testCIDR, testClustername),
		TargetNode:      types.NodeName(testNodeName),
		DestinationCIDR: testCIDR,
	}

	routeSetCR, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	fcw.DeleteFunc = func(ctx context.Context, name string, opts metav1.DeleteOptions) error {
		return fmt.Errorf(ErrDeleteRouteSet.Error())
	}
	err = r.DeleteRoute(context.TODO(), testClustername, &route)
	if err != nil {
		assert.Equal(t, ErrDeleteRouteSet.Error(), err.Error())
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
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	r.DeleteNode(node)
	assert.Equal(t, (*v1.Node)(nil), r.nodeMap[testNodeName])
}

func TestDeleteRouteSetCR(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node

	routeSetCR, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	err = r.DeleteRouteSetCR(node.Name)
	assert.NoError(t, err)
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

func TestGetRouteSetCondition(t *testing.T) {
	testcases := []struct {
		name              string
		routeStatus       v1alpha1.RouteSetStatus
		expectedCondition *v1alpha1.RouteSetCondition
	}{
		{
			name: "RouteSet status with RouteConditionTypeReady",
			routeStatus: v1alpha1.RouteSetStatus{
				Conditions: []v1alpha1.RouteSetCondition{
					{
						Type:    v1alpha1.RouteSetConditionTypeReady,
						Status:  v1.ConditionTrue,
						Reason:  "RouteSetCreated",
						Message: "RouteSet CR created",
					},
				},
			},
			expectedCondition: &v1alpha1.RouteSetCondition{
				Type:    v1alpha1.RouteSetConditionTypeReady,
				Status:  v1.ConditionTrue,
				Reason:  "RouteSetCreated",
				Message: "RouteSet CR created",
			},
		},
		{
			name: "RouteSet status with RouteConditionTypeFailure",
			routeStatus: v1alpha1.RouteSetStatus{
				Conditions: []v1alpha1.RouteSetCondition{
					{
						Type:    "Failure",
						Status:  v1.ConditionTrue,
						Reason:  "RouteSetFailed",
						Message: "RouteSet CR creation failed",
					},
				},
			},
			expectedCondition: (*v1alpha1.RouteSetCondition)(nil),
		},
		{
			name:              "empty RouteSet status",
			routeStatus:       v1alpha1.RouteSetStatus{},
			expectedCondition: (*v1alpha1.RouteSetCondition)(nil),
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedCondition, GetRouteSetCondition(&testCase.routeStatus, v1alpha1.RouteSetConditionTypeReady))
		})
	}
}

func TestCheckStaticRouteRealizedState(t *testing.T) {
	r, _ := initRouteTest()
	node := buildFakeNode(testNodeName)
	r.nodeMap[testNodeName] = node
	routeSet1, err := r.createRouteSetCR(context.TODO(), testClustername, testNameHint, testNodeName, testCIDR, testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet1, nil)

	err = r.checkStaticRouteRealizedState(routeSet1.Name)
	assert.NotEqual(t, err, nil)
	assert.Equal(t, "timed out waiting for static route fakeNode1", err.Error())

	fakeNode2 := buildFakeNode("fakeNode2")
	r.AddNode(fakeNode2)
	routeSet2, err := createFakeRouteSetCR(r, testClustername, testNameHint, "fakeNode2", "100.96.1.0/24", testNodeIP)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSet2, nil)
	err = r.checkStaticRouteRealizedState(routeSet2.Name)
	assert.NoError(t, err)

	// clean up
	err = r.DeleteRouteSetCR(node.Name)
	assert.NoError(t, err)
	err = r.DeleteRouteSetCR(fakeNode2.Name)
	assert.NoError(t, err)
}
