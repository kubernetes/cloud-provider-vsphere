/*
Copyright 2020 The Kubernetes Authors.

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

package route

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/route/config"

	"github.com/vmware/vsphere-automation-sdk-go/runtime/bindings"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data/serializers/cleanjson"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

func TestListRoutes(t *testing.T) {
	response := `
{
  "results" : [ {
    "unique_id" : "be13d703-97c9-49f3-818b-91280151b71e",
    "_last_modified_user" : "admin",
    "_revision" : 0,
    "_system_owned" : false,
    "resource_type" : "StaticRoutes",
    "_protection" : "NOT_PROTECTED",
    "_last_modified_time" : 1606960153899,
    "overridden" : false,
    "display_name" : "62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24",
    "_create_user" : "admin",
    "network" : "100.96.0.0/24",
    "tags" : [ {
      "scope" : "vsphere.k8s.io/cluster-name",
      "tag" : "kubernetes"
    }, {
      "scope" : "vsphere.k8s.io/node-name",
      "tag" : "node1"
    } ],
    "_create_time" : 1606960153895,
    "path" : "/infra/tier-1s/test-t1/static-routes/62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24",
    "marked_for_delete" : false,
    "enabled_on_secondary" : false,
    "parent_path" : "/infra/tier-1s/test-t1",
    "id" : "62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24",
    "relative_path" : "62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24",
    "next_hops" : [ {
      "ip_address" : "172.50.0.13",
      "admin_distance" : 1
    } ],
    "status" : {
      "consolidated_status_per_enforcement_point" : [ {
        "consolidated_status" : {
          "consolidated_status" : "SUCCESS"
        },
        "resource_type" : "ConsolidatedStatusPerEnforcementPoint",
        "enforcement_point_id" : "default"
      } ],
      "intent_version" : "0",
      "consolidated_status" : {
        "consolidated_status" : "SUCCESS"
      },
      "intent_path" : "/infra/tier-1s/test-t1/static-routes/62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24",
      "publish_status" : "REALIZED"
    }
  }, {
    "unique_id" : "92cab961-2178-430c-9626-1f87a4448960",
    "_last_modified_user" : "admin",
    "_revision" : 0,
    "_system_owned" : false,
    "resource_type" : "StaticRoutes",
    "_protection" : "NOT_PROTECTED",
    "_last_modified_time" : 1606960153899,
    "overridden" : false,
    "display_name" : "a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "_create_user" : "admin",
    "network" : "100.96.1.0/24",
    "tags" : [ {
      "scope" : "vsphere.k8s.io/cluster-name",
      "tag" : "kubernetes"
    }, {
      "scope" : "vsphere.k8s.io/node-name",
      "tag" : "node2"
    } ],
    "_create_time" : 1606960153896,
    "path" : "/infra/tier-1s/test-t1/static-routes/a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "marked_for_delete" : false,
    "enabled_on_secondary" : false,
    "parent_path" : "/infra/tier-1s/test-t1",
    "id" : "a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "relative_path" : "a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "next_hops" : [ {
      "ip_address" : "172.50.0.137",
      "admin_distance" : 1
    } ],
    "status" : {
      "consolidated_status_per_enforcement_point" : [ {
        "consolidated_status" : {
          "consolidated_status" : "SUCCESS"
        },
        "resource_type" : "ConsolidatedStatusPerEnforcementPoint",
        "enforcement_point_id" : "default"
      } ],
      "intent_version" : "0",
      "consolidated_status" : {
        "consolidated_status" : "SUCCESS"
      },
      "intent_path" : "/infra/tier-1s/test-t1/static-routes/a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
      "publish_status" : "REALIZED"
    }
  } ],
  "result_count" : 2,
  "cursor" : "2"
}
`
	d := json.NewDecoder(strings.NewReader(response))
	d.UseNumber()
	var jsondata interface{}
	d.Decode(&jsondata)
	decoder := cleanjson.NewJsonToDataValueDecoder()
	dataValue, _ := decoder.Decode(jsondata)
	typeConverter := bindings.NewTypeConverter()
	output, _ := typeConverter.ConvertToGolang(dataValue, bindings.NewReferenceType(model.SearchResponseBindingType))
	queryParam := "resource_type:StaticRoutes AND tags.scope:vsphere.k8s.io/cluster-name AND tags.tag:kubernetes"

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBroker := NewMockNsxtBroker(ctrl)
	mockBroker.EXPECT().QueryEntities(queryParam).Return(output, nil)
	p := &routeProvider{
		routerPath: "/infra/tier-1s/test-t1",
		broker:     mockBroker,
	}
	routes, err := p.ListRoutes(context.TODO(), "kubernetes")

	assert.Equal(t, nil, err, "Should not return error")
	assert.Equal(t, 2, len(routes), "Should have 2 routes")
	route := routes[0]
	assert.Equal(t, "62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24", route.Name,
		"Route name should be 62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24")
	assert.Equal(t, types.NodeName("node1"), route.TargetNode, "Node name should be node1")
	assert.Equal(t, "100.96.0.0/24", route.DestinationCIDR, "DestinationCIDR should be 100.96.0.0/24")

	route = routes[1]
	assert.Equal(t, "a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24", route.Name,
		"Route name should be a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24")
	assert.Equal(t, types.NodeName("node2"), route.TargetNode, "Node name should be node2")
	assert.Equal(t, "100.96.1.0/24", route.DestinationCIDR, "DestinationCIDR should be 100.96.      1.0/24")
}

func TestCreateRoute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBroker := NewMockNsxtBroker(ctrl)
	route := cloudprovider.Route{
		TargetNode:      types.NodeName("node1"),
		DestinationCIDR: "100.96.0.0/24",
	}
	clusterNameScope := config.ClusterNameTagScope
	nodeNameScope := config.NodeNameTagScope
	clusterName := "kubernetes"
	nodeName := "node1"
	var tags []model.Tag
	tags = append(tags, model.Tag{Scope: &clusterNameScope, Tag: &clusterName})
	tags = append(tags, model.Tag{Scope: &nodeNameScope, Tag: &nodeName})
	network := "100.96.0.0/24"
	nodeIP := "172.50.0.13"
	routeName := "kubernetes_node1_100.96.0.0/24"
	staticRoute := model.StaticRoutes{
		DisplayName: &routeName,
		Network:     &network,
		NextHops:    []model.RouterNexthop{{IpAddress: &nodeIP}},
		Tags:        tags,
	}
	nameHint := "62d347a4-1b70-435e-b92a-9a61453843ee"
	routeID := "62d347a4-1b70-435e-b92a-9a61453843ee_100.96.0.0_24"
	p := &routeProvider{
		routerPath: "/infra/tier-1s/test-t1",
		broker:     mockBroker,
		nodeMap:    make(map[string]*v1.Node),
	}
	node := buildFakeNode(nodeName)
	p.nodeMap[nodeName] = node

	mockBroker.EXPECT().CreateStaticRoute(p.routerPath, routeID, staticRoute).Return(errors.New("mock error"))
	p.CreateRoute(context.TODO(), clusterName, nameHint, &route)
	mockBroker.EXPECT().ListRealizedEntities(routeID).Times(0)
}

func TestCheckStaticRouteRealizedState(t *testing.T) {
	response := `
{
  "results" : [ {
    "extended_attributes" : [ {
      "data_type" : "STRING",
      "multivalue" : false,
      "values" : [ "47cb5dd7-7f2a-41e5-886d-b36ef8c31bf4" ],
      "key" : "logical-router-id"
    } ],
    "intent_paths" : [ "/infra/tier-1s/test-t1/static-routes/a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24" ],
    "resource_type" : "GenericPolicyRealizedResource",
    "id" : "test-t1-a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "display_name" : "test-t1-a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "path" : "/infra/realized-state/enforcement-points/default/tier-1-static-routes/test-t1-a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "relative_path" : "test-t1-a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24",
    "parent_path" : "/infra/realized-state/enforcement-points/default",
    "unique_id" : "7111cdae-6d03-434a-846f-7685eab433e2",
    "intent_reference" : [ "/infra/tier-1s/test-t1/static-routes/a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24" ],
    "realization_specific_identifier" : "3598b02f-4fe6-47f0-89b8-494a035c10ff",
    "realization_api" : "/api/v1/logical-routers/47cb5dd7-7f2a-41e5-886d-b36ef8c31bf4/routing/static-routes/3598b02f-4fe6-47f0-89b8-494a035c10ff",
    "state" : "REALIZED",
    "alarms" : [ ],
    "runtime_status" : "UNINITIALIZED",
    "_create_user" : "system",
    "_create_time" : 1606960154089,
    "_last_modified_user" : "system",
    "_last_modified_time" : 1606960154204,
    "_system_owned" : false,
    "_protection" : "NOT_PROTECTED",
    "_revision" : 1
  } ],
  "result_count" : 1
}
`
	d := json.NewDecoder(strings.NewReader(response))
	d.UseNumber()
	var jsondata interface{}
	d.Decode(&jsondata)
	decoder := cleanjson.NewJsonToDataValueDecoder()
	dataValue, _ := decoder.Decode(jsondata)
	typeConverter := bindings.NewTypeConverter()
	output, _ := typeConverter.ConvertToGolang(dataValue,
		bindings.NewReferenceType(model.GenericPolicyRealizedResourceListResultBindingType))

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBroker := NewMockNsxtBroker(ctrl)
	routePath := "/infra/tier-1s/test-t1/static-routes/a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24"
	mockBroker.EXPECT().ListRealizedEntities(routePath).Return(output, nil)
	p := &routeProvider{
		routerPath: "/infra/tier-1s/test-t1",
		broker:     mockBroker,
	}

	err := p.checkStaticRouteRealizedState("a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24")
	assert.Equal(t, nil, err, "Should not return error")
}

func TestDeleteRoute(t *testing.T) {
	clusterName := "kubernetes"
	routerName := "a4775ec4-8b68-42ea-86fc-d17390e4c373_100.96.1.0_24"
	route := cloudprovider.Route{
		Name: routerName,
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockBroker := NewMockNsxtBroker(ctrl)
	p := &routeProvider{
		routerPath: "/infra/tier-1s/test-t1",
		broker:     mockBroker,
	}
	mockBroker.EXPECT().DeleteStaticRoute(p.routerPath, routerName).Return(nil)

	err := p.DeleteRoute(context.TODO(), clusterName, &route)
	assert.Equal(t, nil, err, "Should not return error")
}

func buildFakeNode(nodeName string) *v1.Node {
	addresses := make([]v1.NodeAddress, 2)
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: nodeName})
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeInternalIP, Address: "172.50.0.13"})
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

func TestGetNodeIPAddress(t *testing.T) {
	p := &routeProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	nodeName := "node1"
	node := buildFakeNode(nodeName)
	p.nodeMap[nodeName] = node
	ip, err := p.getNodeIPAddress(nodeName, true)
	assert.Equal(t, "172.50.0.13", ip, "Node IP address should be 172.50.0.13")
	assert.Equal(t, nil, err, "Should not return error")
}

func TestIsIPv4(t *testing.T) {
	str := "100.96.1.0/24"
	assert.Equal(t, true, IsIPv4(str))

	str = "fe80::20c:29ff:fe0b:b407/64"
	assert.Equal(t, false, IsIPv4(str))
}

func TestAddNode(t *testing.T) {
	p := &routeProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	nodeName := "node1"
	node := buildFakeNode(nodeName)
	p.AddNode(node)
	assert.Equal(t, node, p.nodeMap[nodeName], "Node should be in nodeMap")
}

func TestDeleteNode(t *testing.T) {
	p := &routeProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	nodeName := "node1"
	node := buildFakeNode(nodeName)
	p.nodeMap[nodeName] = node
	p.DeleteNode(node)
	assert.Equal(t, (*v1.Node)(nil), p.nodeMap[nodeName], "Node should not be in nodeMap")
}

func TestGetNode(t *testing.T) {
	p := &routeProvider{
		nodeMap: make(map[string]*v1.Node),
	}
	nodeName := "node1"
	node := buildFakeNode(nodeName)
	p.nodeMap[nodeName] = node
	nodeInMap, err := p.getNode(nodeName)
	assert.Equal(t, node, nodeInMap, "Node should be the same")
	assert.Equal(t, nil, err, "Should not return any error")
}
