package routeset

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	t1networkingapis "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	faket1networkingclients "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/fake"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
)

const (
	testClusterNameSpace = "test-guest-cluster-ns"
	testClustername      = "test-cluster"
	testNodeIP           = "172.50.0.13"
	testCIDR             = "100.96.0.0/24"
	testNodeName         = "fakeNode1"
	testNameHint         = "62d347a4-1b70-435e-b92a-9a61453843ee"
)

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

func initRouteManagerTest() *RouteManager {
	// create the fake client
	fc := faket1networkingclients.NewSimpleClientset()

	rs, _ := NewRouteManagerWithClients(fc, testClusterNameSpace)
	return rs
}

func TestCreateRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeCR, nil)
	routeSetCR, ok := routeCR.(*t1networkingapis.RouteSet)
	assert.Equal(t, ok, true)

	expectedRouteSetSpec := t1networkingapis.RouteSetSpec{
		Routes: []t1networkingapis.Route{
			{
				Name:        helper.GetRouteName(testNodeName, testCIDR, testClustername),
				Destination: testCIDR,
				Target:      testNodeIP,
			},
		},
	}
	expectedLabels := map[string]string{
		helper.LabelKeyClusterName: testClustername,
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
	assert.Equal(t, testClusterNameSpace, routeSetCR.Namespace)
	assert.Equal(t, expectedRouteSetSpec, routeSetCR.Spec)
	assert.Equal(t, expectedOwnerRefs, routeSetCR.OwnerReferences)
	assert.Equal(t, expectedLabels, routeSetCR.Labels)

	// clean up
	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestDeleteRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeSetCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestWaitRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeSetCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeSetCR, nil)

	err = rs.WaitRouteCR(testNodeName)
	assert.Equal(t, "route set fakeNode1 is not ready", err.Error())

	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestGetRouteCondition(t *testing.T) {
	testcases := []struct {
		name              string
		routeStatus       t1networkingapis.RouteSetStatus
		expectedCondition *t1networkingapis.RouteSetCondition
	}{
		{
			name: "RouteSet status with RouteConditionTypeReady",
			routeStatus: t1networkingapis.RouteSetStatus{
				Conditions: []t1networkingapis.RouteSetCondition{
					{
						Type:    t1networkingapis.RouteSetConditionTypeReady,
						Status:  v1.ConditionTrue,
						Reason:  "RouteSetCreated",
						Message: "RouteSet CR created",
					},
				},
			},
			expectedCondition: &t1networkingapis.RouteSetCondition{
				Type:    t1networkingapis.RouteSetConditionTypeReady,
				Status:  v1.ConditionTrue,
				Reason:  "RouteSetCreated",
				Message: "RouteSet CR created",
			},
		},
		{
			name: "RouteSet status with RouteConditionTypeFailure",
			routeStatus: t1networkingapis.RouteSetStatus{
				Conditions: []t1networkingapis.RouteSetCondition{
					{
						Type:    "Failure",
						Status:  v1.ConditionTrue,
						Reason:  "RouteSetFailed",
						Message: "RouteSet CR creation failed",
					},
				},
			},
			expectedCondition: (*t1networkingapis.RouteSetCondition)(nil),
		},
		{
			name:              "empty RouteSet status",
			routeStatus:       t1networkingapis.RouteSetStatus{},
			expectedCondition: (*t1networkingapis.RouteSetCondition)(nil),
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedCondition, GetRouteCRCondition(&testCase.routeStatus, t1networkingapis.RouteSetConditionTypeReady))
		})
	}
}
