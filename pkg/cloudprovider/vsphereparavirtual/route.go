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
	"net"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
	"k8s.io/cloud-provider-vsphere/pkg/util"
)

// RoutesProvider is the interface definition for Routes functionality
type RoutesProvider interface {
	cloudprovider.Routes
}

type routesProvider struct {
	routeManager routemanager.RouteManager
	ownerRefs    []metav1.OwnerReference
	nodeLister   listerv1.NodeLister
}

var _ RoutesProvider = &routesProvider{}

// NewRoutes returns an implementation of RoutesProvider
func NewRoutes(clusterNS string, kcfg *rest.Config, ownerRef metav1.OwnerReference, vpcModeEnabled bool, nodeLister listerv1.NodeLister) (RoutesProvider, error) {
	routeManager, err := routemanager.GetRouteManager(vpcModeEnabled, kcfg, clusterNS)
	if err != nil {
		return nil, err
	}

	ownerRefs := []metav1.OwnerReference{
		ownerRef,
	}

	return &routesProvider{
		routeManager: routeManager,
		nodeLister:   nodeLister,
		ownerRefs:    ownerRefs,
	}, nil
}

// ListRoutes implements Routes.ListRoutes
// Get RouteSet or StaticRoute CR from SC namespace and then filters routes that belong to the specified clusterName
// Only return cloudprovider.Route if RouteSet CR status 'Ready' is true
func (r *routesProvider) ListRoutes(ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	klog.V(6).Infof("Attempting to list Routes for cluster %s", clusterName)

	// use labelSelector to filter RouteSet CRs that belong to this cluster
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{helper.LabelKeyClusterName: clusterName},
	}
	routes, err := r.routeManager.ListRouteCR(ctx, labelSelector)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []*cloudprovider.Route{}, nil
		}
		klog.ErrorS(helper.ErrListRouteCR, fmt.Sprintf("%v", err))
		return nil, err
	}

	return r.routeManager.CreateCPRoutes(routes)
}

// crNameForRoute returns the StaticRoute CR name for a given Kubernetes node name
// and destination CIDR. IPv4 CIDRs use the bare node name to remain compatible
// with existing StaticRoute CRs created before dual-stack support; changing the
// IPv4 naming scheme would orphan those CRs on upgrade. IPv6 CIDRs append
// helper.SuffixIPv6 so dual-stack nodes can have one CR per address family
// without name collision.
func crNameForRoute(nodeName, destCIDR string) string {
	if util.IsIPv4(destCIDR) {
		return nodeName
	}
	return nodeName + helper.SuffixIPv6
}

// CreateRoute implements Routes.CreateRoute
// Create a RouteSet or StaticRoute CR for a Node
func (r *routesProvider) CreateRoute(ctx context.Context, clusterName string, nameHint string, route *cloudprovider.Route) error {
	nodeName := string(route.TargetNode)
	crName := crNameForRoute(nodeName, route.DestinationCIDR)
	klog.V(6).Infof("Creating Route for node %s (CR name %s) with hint %s in cluster %s", nodeName, crName, nameHint, clusterName)

	nodeIP, err := r.getNodeIPAddress(nodeName, util.IsIPv4(route.DestinationCIDR))
	if err != nil {
		klog.Errorf("getting node %s IP address failed: %v", nodeName, err)
		return err
	}

	labels := map[string]string{
		helper.LabelKeyClusterName: clusterName,
	}
	nodeRef := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       nodeName,
		UID:        types.UID(nameHint),
	}
	owners := make([]metav1.OwnerReference, len(r.ownerRefs))
	copy(owners, r.ownerRefs)
	owners = append(owners, nodeRef)
	routeInfo := &helper.RouteInfo{
		Labels:    labels,
		Owner:     owners,
		Name:      crName,
		Cidr:      route.DestinationCIDR,
		NodeIP:    nodeIP,
		RouteName: helper.GetRouteName(crName, route.DestinationCIDR, clusterName),
	}
	_, err = r.routeManager.CreateRouteCR(ctx, routeInfo)
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			klog.Errorf("Route CR %s is already existing: %v", crName, err)
			return nil
		}
		klog.Errorf("creating Route CR for node %s failed: %s", crName, err)
		return err
	}
	klog.V(6).Infof("Successfully created Route CR %s for node %s", crName, nodeName)
	return r.checkStaticRouteRealizedState(crName)
}

// checkStaticRouteRealizedState checks static route realized state. The ready status is updated to Route CR by ncp/nsx-operator afterwards
// The check happens every 1 second and the default timeout is 10 seconds
func (r *routesProvider) checkStaticRouteRealizedState(crName string) error {
	timeout := time.After(helper.RealizedStateTimeout)
	ticker := time.NewTicker(helper.RealizedStateSleepTime)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for static route %s", crName)
		case <-ticker.C:
			if err := r.routeManager.WaitRouteCR(crName); err == nil {
				return nil
			}
		}
	}
}

// DeleteRoute implements Routes.DeleteRoute
// Delete node's corresponding RouteSet or StaticRoute CR
func (r *routesProvider) DeleteRoute(ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	nodeName := string(route.TargetNode)
	crName := crNameForRoute(nodeName, route.DestinationCIDR)
	klog.V(6).Infof("Deleting Route CR %s in cluster %s", crName, clusterName)
	if err := r.routeManager.DeleteRouteCR(crName); err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(6).Infof("Route CR %s already gone, treating as success", crName)
			return nil
		}
		klog.ErrorS(helper.ErrDeleteRouteCR, fmt.Sprintf("%v", err))
		return err
	}
	klog.V(6).Infof("Successfully deleted Route CR %s for node %s", crName, nodeName)
	return nil
}

// getNodeIPAddress returns the first node address whose IP family matches the
// requested family. wantIPv4 selects IPv4 (true) or IPv6 (false). Internal
// addresses take precedence over external addresses; within each priority
// tier the first matching address wins. For dual-stack nodes, callers invoke
// this function once per route family and create a separate StaticRoute CR
// for each address family.
func (r *routesProvider) getNodeIPAddress(nodeName string, wantIPv4 bool) (string, error) {
	node, err := r.nodeLister.Get(nodeName)
	if err != nil {
		klog.Errorf("getting node %s failed: %v", nodeName, err)
		return "", err
	}

	// Collect all IPs (InternalIP first, then ExternalIP for priority)
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

	wantFamily := "IPv6"
	if wantIPv4 {
		wantFamily = "IPv4"
	}

	for _, ip := range allIPs {
		isIPv4 := ip.To4() != nil
		if isIPv4 == wantIPv4 {
			klog.V(4).Infof("successfully fetched %s address %s for node %s", wantFamily, ip.String(), nodeName)
			return ip.String(), nil
		}
	}

	availableIPs := make([]string, len(allIPs))
	for i, ip := range allIPs {
		availableIPs[i] = ip.String()
	}
	return "", fmt.Errorf("node %s does not have any %s address (available IPs: %v)", nodeName, wantFamily, availableIPs)
}
