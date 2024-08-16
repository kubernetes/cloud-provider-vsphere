package staticroute

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	fakevpcnetworkingclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"

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
	fc := fakevpcnetworkingclients.NewSimpleClientset()

	rs, _ := NewRouteManagerWithClients(fc, testClusterNameSpace)
	return rs
}

func TestCreateRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	routeCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, routeCR, nil)
	staticRouteCR, ok := routeCR.(*vpcapisv1.StaticRoute)
	assert.Equal(t, ok, true)

	expectedStaticRouteSpec := vpcapisv1.StaticRouteSpec{
		Network: testCIDR,
		NextHops: []vpcapisv1.NextHop{
			{
				IPAddress: testNodeIP,
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

	assert.Equal(t, testClusterNameSpace, staticRouteCR.Namespace)
	assert.Equal(t, expectedStaticRouteSpec, staticRouteCR.Spec)
	assert.Equal(t, expectedOwnerRefs, staticRouteCR.OwnerReferences)
	assert.Equal(t, expectedLabels, staticRouteCR.Labels)

	// clean up
	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestDeleteRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	StaticRouteCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, StaticRouteCR, nil)

	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestWaitRouteCR(t *testing.T) {
	rs := initRouteManagerTest()
	fakeRouteInfo := buildFakeRouteInfo(testClustername, testNameHint, testCIDR, testNodeName, testNodeIP)
	StaticRouteCR, err := rs.CreateRouteCR(context.TODO(), fakeRouteInfo)
	assert.NoError(t, err)
	assert.NotEqual(t, StaticRouteCR, nil)

	err = rs.WaitRouteCR(testNodeName)
	assert.Equal(t, "Route CR fakeNode1 is not ready", err.Error())

	err = rs.DeleteRouteCR(testNodeName)
	assert.NoError(t, err)
}

func TestGetRouteCRCondition(t *testing.T) {
	testcases := []struct {
		name              string
		routeStatus       vpcapisv1.StaticRouteStatus
		expectedCondition *vpcapisv1.StaticRouteCondition
	}{
		{
			name: "StaticRoute status with RouteConditionTypeReady",
			routeStatus: vpcapisv1.StaticRouteStatus{
				Conditions: []vpcapisv1.StaticRouteCondition{
					{
						Type:    vpcapisv1.Ready,
						Status:  v1.ConditionTrue,
						Reason:  "StaticRouteCreated",
						Message: "StaticRoute CR created",
					},
				},
			},
			expectedCondition: &vpcapisv1.StaticRouteCondition{
				Type:    vpcapisv1.Ready,
				Status:  v1.ConditionTrue,
				Reason:  "StaticRouteCreated",
				Message: "StaticRoute CR created",
			},
		},
		{
			name: "StaticRoute status with RouteConditionTypeFailure",
			routeStatus: vpcapisv1.StaticRouteStatus{
				Conditions: []vpcapisv1.StaticRouteCondition{
					{
						Type:    "Failure",
						Status:  v1.ConditionFalse,
						Reason:  "StaticRouteFailed",
						Message: "StaticRoute CR creation failed",
					},
				},
			},
			expectedCondition: (*vpcapisv1.StaticRouteCondition)(nil),
		},
		{
			name:              "empty StaticRoute status",
			routeStatus:       vpcapisv1.StaticRouteStatus{},
			expectedCondition: (*vpcapisv1.StaticRouteCondition)(nil),
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedCondition, GetRouteCRCondition(&testCase.routeStatus, vpcapisv1.Ready))
		})
	}
}

func TestCreateCPRoutes(t *testing.T) {
	testcases := []struct {
		name           string
		rs             vpcapisv1.StaticRouteList
		expectedRoutes []*cloudprovider.Route
	}{
		{
			name: "There is 2 ready route",
			rs: vpcapisv1.StaticRouteList{
				Items: []vpcapisv1.StaticRoute{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNodeName,
						},
						Spec: vpcapisv1.StaticRouteSpec{
							Network: testCIDR,
						},
						Status: vpcapisv1.StaticRouteStatus{
							Conditions: []vpcapisv1.StaticRouteCondition{
								{
									Type:    vpcapisv1.Ready,
									Status:  v1.ConditionTrue,
									Reason:  "StaticRouteCreated",
									Message: "StaticRoute CR created",
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNodeName,
						},
						Spec: vpcapisv1.StaticRouteSpec{
							Network: testCIDR,
						},
						Status: vpcapisv1.StaticRouteStatus{
							Conditions: []vpcapisv1.StaticRouteCondition{
								{
									Type:    vpcapisv1.Ready,
									Status:  v1.ConditionTrue,
									Reason:  "StaticRouteCreated",
									Message: "StaticRoute CR created",
								},
							},
						},
					},
				},
			},
			expectedRoutes: []*cloudprovider.Route{{
				Name:            testNodeName,
				TargetNode:      types.NodeName(testNodeName),
				DestinationCIDR: testCIDR,
			},
				{
					Name:            testNodeName,
					TargetNode:      types.NodeName(testNodeName),
					DestinationCIDR: testCIDR,
				},
			},
		},
		{
			name: "There is no ready route",
			rs: vpcapisv1.StaticRouteList{
				Items: []vpcapisv1.StaticRoute{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNodeName,
						},
						Spec: vpcapisv1.StaticRouteSpec{
							Network: testCIDR,
						},
						Status: vpcapisv1.StaticRouteStatus{
							Conditions: []vpcapisv1.StaticRouteCondition{
								{
									Type:    "Failure",
									Status:  v1.ConditionFalse,
									Reason:  "StaticRouteFailed",
									Message: "StaticRoute CR creation failed",
								},
							},
						},
					},
				},
			},
			expectedRoutes: nil,
		},
	}

	for _, testCase := range testcases {
		r := initRouteManagerTest()
		t.Run(testCase.name, func(t *testing.T) {
			rs, err := r.CreateCPRoutes(&(testCase.rs))
			assert.Equal(t, nil, err)
			assert.Equal(t, testCase.expectedRoutes, rs)
		})
	}
}
