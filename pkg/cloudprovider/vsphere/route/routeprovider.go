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
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	gonsxt "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/route/config"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
	"k8s.io/cloud-provider-vsphere/pkg/nsxt"
	"k8s.io/klog"
)

// RoutesProvider is the interface for route functionality
type RoutesProvider interface {
	cloudprovider.Routes
	Initialize(*k8s.InformerManager) error
	AddNode(*v1.Node)
	DeleteNode(*v1.Node)
}

type routeProvider struct {
	routerPath  string
	broker      NsxtBroker
	mpClient    *gonsxt.APIClient
	nodeMap     map[string]*v1.Node
	nodeMapLock sync.RWMutex
}

var _ RoutesProvider = &routeProvider{}

// NewRouteProvider creates a new RouteProvider
func NewRouteProvider(cfg *config.Config) (RoutesProvider, error) {
	if cfg == nil {
		return nil, nil
	}
	nsxtbroker, err := NewNsxtBroker(&cfg.NSXT)
	if err != nil {
		return nil, errors.Wrap(err, "creating nsxt broker failed")
	}
	mpclient, err := nsxt.NewManagerClient(&cfg.NSXT)
	if err != nil {
		return nil, errors.Wrap(err, "creating nsxt manager client failed")
	}

	return &routeProvider{
		broker:     nsxtbroker,
		mpClient:   mpclient,
		routerPath: cfg.Route.RouterPath,
		nodeMap:    make(map[string]*v1.Node),
	}, nil
}

// Initialize tries to find node router on NSXT automatically
func (p *routeProvider) Initialize(im *k8s.InformerManager) error {
	if p.routerPath == "" {
		nodeList, err := im.GetNodeList()
		if err != nil {
			return errors.Wrap(err, "failed to get node list")
		}
		routerPath, err := p.findNodeRouter(nodeList)
		if err != nil {
			return errors.Wrap(err, "failed to find node router")
		}
		klog.Infof("Finding node router %s", routerPath)
		p.routerPath = routerPath
	}
	return nil
}

// findNodeRouter finds node's router via providerID
// The finding path is providerID -> externalID -> vifID -> segment port -> segment -> router
func (p *routeProvider) findNodeRouter(nodeList []*v1.Node) (string, error) {
	for _, node := range nodeList {
		providerID := node.Spec.ProviderID
		if !strings.HasPrefix(providerID, config.ProviderIDPrefix) {
			continue
		}
		providerID = providerID[len(config.ProviderIDPrefix):]
		externalID, err := p.getNodeExternalID(providerID)
		if err != nil || externalID == "" {
			continue
		}
		vifID, err := p.getNodeVifID(externalID)
		if err != nil || vifID == "" {
			continue
		}
		segmentPath, err := p.getNodeSegmentPath(vifID)
		if err != nil || segmentPath == "" {
			continue
		}
		routerPath, err := p.getNodeRouterPath(segmentPath)
		if err != nil || routerPath == "" {
			continue
		}
		return routerPath, nil
	}
	return "", errors.New("Finding node router failed")
}

func (p *routeProvider) getNodeExternalID(providerID string) (string, error) {
	vms, _, err := p.mpClient.FabricApi.ListVirtualMachines(p.mpClient.Context, nil)
	if err != nil {
		return "", err
	}
	for _, vm := range vms.Results {
		for _, computeID := range vm.ComputeIds {
			if !strings.HasPrefix(computeID, config.BiosUuidPrefix) {
				continue
			}
			if providerID == computeID[len(config.BiosUuidPrefix):] {
				return vm.ExternalId, nil
			}
		}
	}
	return "", nil
}

func (p *routeProvider) getNodeVifID(externalID string) (string, error) {
	localVarOptionals := make(map[string]interface{})
	localVarOptionals["ownerVmId"] = externalID
	vifs, _, err := p.mpClient.FabricApi.ListVifs(p.mpClient.Context, localVarOptionals)
	if err != nil {
		return "", err
	}
	if len(vifs.Results) == 0 {
		return "", errors.Errorf("VIF for VM %s not found", externalID)
	}
	vifID := ""
	for _, vif := range vifs.Results {
		// Node only has one interface attached to NSXT segment
		if vif.LportAttachmentId != "" {
			vifID = vif.LportAttachmentId
			break
		}
	}
	return vifID, nil
}

func (p *routeProvider) getNodeSegmentPath(vifID string) (string, error) {
	queryParam := fmt.Sprintf("resource_type:SegmentPort AND attachment.id:%s", vifID)
	ports, err := p.broker.QueryEntities(queryParam)
	if err != nil {
		return "", err
	}
	if len(ports.Results) == 0 {
		return "", nil
	}
	port := ports.Results[0]
	pathField, _ := port.Field("parent_path")
	parentPath := (pathField).(*data.StringValue).Value()
	return parentPath, nil
}

func (p *routeProvider) getNodeRouterPath(segmentPath string) (string, error) {
	segment, err := p.broker.GetSegment(segmentPath)
	if err != nil {
		return "", nil
	}
	routerPath := *segment.ConnectivityPath
	return routerPath, nil
}

// ListRoutes returns a list of routes which have static routes on NSXT
func (p *routeProvider) ListRoutes(ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	queryParam := fmt.Sprintf("resource_type:StaticRoutes AND tags.scope:%s AND tags.tag:%s",
		config.ClusterNameTagScope, clusterName)
	staticRoutes, err := p.broker.QueryEntities(queryParam)
	if err != nil {
		klog.Errorf("querying static routes for cluster %s failed", clusterName)
		return nil, err
	}

	var routes []*cloudprovider.Route
	for _, item := range staticRoutes.Results {
		nodeName := ""
		tagsField, _ := item.Field("tags")
		tags := (tagsField).(*data.ListValue).List()
		for _, tItem := range tags {
			scope, _ := (tItem).(*data.StructValue).Field("scope")
			if (scope).(*data.StringValue).Value() == config.NodeNameTagScope {
				tag, _ := (tItem).(*data.StructValue).Field("tag")
				nodeName = (tag).(*data.StringValue).Value()
				break
			}
		}
		routeID, _ := item.Field("id")
		network, _ := item.Field("network")
		route := &cloudprovider.Route{
			Name:            (routeID).(*data.StringValue).Value(),
			TargetNode:      types.NodeName(nodeName),
			DestinationCIDR: (network).(*data.StringValue).Value(),
		}
		routes = append(routes, route)
	}
	return routes, nil
}

// CreateRoute creates a static route on NSXT for a Node
func (p *routeProvider) CreateRoute(ctx context.Context, clusterName string, nameHint string, route *cloudprovider.Route) error {
	nodeName := string(route.TargetNode)
	klog.V(6).Infof("Creating static route for node %s", nodeName)

	nodeIP, err := p.getNodeIPAddress(nodeName, IsIPv4(route.DestinationCIDR))
	if err != nil {
		klog.Errorf("getting node %s IP address failed: %v", nodeName, err)
		return err
	}

	var tags []model.Tag
	clusterNameScope := config.ClusterNameTagScope
	nodeNameScope := config.NodeNameTagScope
	tags = append(tags, model.Tag{Scope: &clusterNameScope, Tag: &clusterName})
	tags = append(tags, model.Tag{Scope: &nodeNameScope, Tag: &nodeName})
	var nexthops []model.RouterNexthop
	nexthops = append(nexthops, model.RouterNexthop{IpAddress: &nodeIP})
	routeID := nameHint + "_" + route.DestinationCIDR
	routeID = strings.ReplaceAll(routeID, "/", "_")
	routeName := clusterName + "_" + nodeName + "_" + route.DestinationCIDR
	staticRoute := model.StaticRoutes{
		DisplayName: &routeName,
		Network:     &route.DestinationCIDR,
		NextHops:    nexthops,
		Tags:        tags,
	}
	err = p.broker.CreateStaticRoute(p.routerPath, routeID, staticRoute)
	if err != nil {
		klog.Errorf("creating static route %s for node %s failed: %s", routeID, nodeName, err)
		return err
	}

	// Get realized state
	return p.checkStaticRouteRealizedState(routeID)
}

// DeleteRoute deletes Node's static route on NSXT
func (p *routeProvider) DeleteRoute(ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	klog.V(6).Infof("Deleting static route %s on router %s in cluster %s",
		route.Name, p.routerPath, clusterName)
	err := p.broker.DeleteStaticRoute(p.routerPath, route.Name)
	if err != nil {
		klog.Errorf("deleting static route %s failed: %s", route.Name, err)
		return err
	}
	return nil
}

// checkStaticRouteRealizedState checks static route realized state
// The check happends every 1 second and the default timeout is 10 seconds
// Do not delete the creating static route after the timeout
func (p *routeProvider) checkStaticRouteRealizedState(routeID string) error {
	path := p.routerPath + "/static-routes/" + routeID
	limit := time.Now().Add(config.RealizedStateTimeout)
	for time.Now().Before(limit) {
		list, err := p.broker.ListRealizedEntities(path)
		if err != nil {
			return fmt.Errorf("get route %s realized state failed: %s", path, err)
		}
		for _, resource := range list.Results {
			if len(resource.IntentPaths) == 0 {
				continue
			}
			if resource.IntentPaths[0] == path && *resource.State == config.RealizedState {
				return nil
			}
		}
		time.Sleep(config.RealizedStateSleepTime)
	}
	return fmt.Errorf("timed out waiting for static route %s", path)
}

// getNodeIPAddress gets node IP address
// The order is to choose node internal IP first, then external IP.
// Return the first IP address as node IP.
func (p *routeProvider) getNodeIPAddress(nodeName string, isIPv4 bool) (string, error) {
	node, err := p.getNode(nodeName)
	if err != nil {
		klog.Errorf("getting node %s failed: %v", nodeName, err)
		return "", err
	}

	allIPs := make([]net.IP, 0, len(node.Status.Addresses))
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeInternalIP {
			ip := net.ParseIP(addr.Address)
			if ip != nil {
				allIPs = append(allIPs, ip)
			}
		}
	}
	for _, addr := range node.Status.Addresses {
		if addr.Type == v1.NodeExternalIP {
			ip := net.ParseIP(addr.Address)
			if ip != nil {
				allIPs = append(allIPs, ip)
			}
		}
	}
	if len(allIPs) == 0 {
		return "", fmt.Errorf("node %s has neither InternalIP nor ExternalIP", nodeName)
	}
	for _, ip := range allIPs {
		if (ip.To4() != nil) == isIPv4 {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("node %s does not have the same IP family with podCIDR", nodeName)
}

// AddNode adds v1.Node in nodeMap
func (p *routeProvider) AddNode(node *v1.Node) {
	p.nodeMapLock.Lock()
	p.nodeMap[node.Name] = node
	p.nodeMapLock.Unlock()
}

// DeleteNode deletes v1.Node from nodeMap
func (p *routeProvider) DeleteNode(node *v1.Node) {
	p.nodeMapLock.Lock()
	delete(p.nodeMap, node.Name)
	p.nodeMapLock.Unlock()
}

// getNode returns v1.Node from nodeMap
func (p *routeProvider) getNode(name string) (*v1.Node, error) {
	p.nodeMapLock.Lock()
	defer p.nodeMapLock.Unlock()
	if p.nodeMap[name] != nil {
		return p.nodeMap[name], nil
	}
	return nil, errors.New("Node not found")
}

// IsIPv4 checks whether IP address is IPv4
func IsIPv4(str string) bool {
	str = strings.Split(str, "/")[0]
	ip := net.ParseIP(str)
	return ip != nil && ip.To4() != nil
}
