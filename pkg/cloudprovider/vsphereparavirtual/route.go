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
	"strings"
	"time"

	"sync"

	"github.com/pkg/errors"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	rest "k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	v1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	client "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	"k8s.io/cloud-provider-vsphere/pkg/util"
	klog "k8s.io/klog/v2"
)

// RoutesProvider is the interface definition for Routes functionality
type RoutesProvider interface {
	cloudprovider.Routes
	AddNode(*v1.Node)
	DeleteNode(*v1.Node)
}

type routesProvider struct {
	routeClient client.Interface
	namespace   string
	nodeMap     map[string]*v1.Node
	nodeMapLock sync.RWMutex
	ownerRefs   []metav1.OwnerReference
}

var _ RoutesProvider = &routesProvider{}

const (
	// LabelKeyClusterName is the label key to specify GC name for RouteSet CR
	LabelKeyClusterName = "clusterName"
	// RealizedStateTimeout is the timeout duration for realized state check
	RealizedStateTimeout = 10 * time.Second
	// RealizedStateSleepTime is the interval between realized state check
	RealizedStateSleepTime = 1 * time.Second
)

// A list of possible RouteSet operation error messages
var (
	ErrGetRouteSet    = errors.New("failed to get RouteSet")
	ErrCreateRouteSet = errors.New("failed to create RouteSet")
	ErrListRouteSet   = errors.New("failed to list RouteSet")
	ErrDeleteRouteSet = errors.New("failed to delete RouteSet")
)

// GetRouteSetClient returns a new RouteSet client that can be used to access SC
func GetRouteSetClient(config *rest.Config) (client.Interface, error) {
	v1alpha1.AddToScheme(scheme.Scheme)
	rClient, err := client.NewForConfig(config)
	if err != nil {
		klog.V(6).Infof("Failed to create RouteSet clientset")
		return nil, err
	}
	return rClient, nil
}

// NewRoutes returns an implementation of RoutesProvider
func NewRoutes(clusterNS string, kcfg *rest.Config, ownerRef metav1.OwnerReference) (RoutesProvider, error) {
	routeClient, err := GetRouteSetClient(kcfg)
	if err != nil {
		return nil, err
	}

	ownerRefs := []metav1.OwnerReference{
		ownerRef,
	}
	return &routesProvider{
		routeClient: routeClient,
		namespace:   clusterNS,
		nodeMap:     make(map[string]*v1.Node),
		ownerRefs:   ownerRefs,
	}, nil
}

// ListRoutes implements Routes.ListRoutes
// Get RouteSet CR from SC namespace and then filters routes that belong to the specified clusterName
// Only return cloudprovider.Route if RouteSet CR status 'Ready' is true
func (r *routesProvider) ListRoutes(ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	klog.V(6).Infof("Attempting to list Routes for cluster %s", clusterName)

	// use labelSelector to filter RouteSet CRs that belong to this cluster
	labelSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{LabelKeyClusterName: clusterName},
	}
	routeSets, err := r.routeClient.NsxV1alpha1().RouteSets(r.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(labelSelector.MatchLabels).String(),
	})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []*cloudprovider.Route{}, nil
		}
		klog.ErrorS(ErrListRouteSet, fmt.Sprintf("%v", err))
		return nil, err
	}
	if len(routeSets.Items) == 0 {
		return []*cloudprovider.Route{}, nil
	}
	return r.createCPRoutes(routeSets), nil
}

// createCPRoutes creates cloudprovider Routes based on RouteSet CR
func (r *routesProvider) createCPRoutes(routeSets *v1alpha1.RouteSetList) []*cloudprovider.Route {
	var routes []*cloudprovider.Route
	for _, routeSet := range routeSets.Items {
		// only return cloudprovider.Route if RouteSet CR status 'Ready' is true
		condition := GetRouteSetCondition(&(routeSet.Status), v1alpha1.RouteSetConditionTypeReady)
		if condition != nil && condition.Status == v1.ConditionTrue {
			// one RouteSet per node, so we can use nodeName as the name of RouteSet CR
			nodeName := routeSet.Name
			for _, route := range routeSet.Spec.Routes {
				cpRoute := &cloudprovider.Route{
					Name:            route.Name,
					TargetNode:      types.NodeName(nodeName),
					DestinationCIDR: route.Destination,
				}
				routes = append(routes, cpRoute)
			}
		}
	}
	return routes
}

// CreateRoute implements Routes.CreateRoute
// Create a RouteSet custom resource for a Node
func (r *routesProvider) CreateRoute(ctx context.Context, clusterName string, nameHint string, route *cloudprovider.Route) error {
	nodeName := string(route.TargetNode)
	klog.V(6).Infof("Creating Route for node %s with hint %s in cluster %s", nodeName, nameHint, clusterName)

	nodeIP, err := r.getNodeIPAddress(nodeName, util.IsIPv4(route.DestinationCIDR))
	if err != nil {
		klog.Errorf("getting node %s IP address failed: %v", nodeName, err)
		return err
	}

	routeSet, err := r.createRouteSetCR(ctx, clusterName, nameHint, nodeName, route.DestinationCIDR, nodeIP)
	if err != nil {
		klog.Errorf("creating RouteSet CR for node %s failed: %s", nodeName, err)
		return err
	}
	// check realized state of static routes
	return r.checkStaticRouteRealizedState(routeSet.Name)
}

// createRouteSetCR creates RouteSet CR through RouteSet client
func (r *routesProvider) createRouteSetCR(ctx context.Context, clusterName string, nameHint string, nodeName string, cidr string, nodeIP string) (*v1alpha1.RouteSet, error) {
	labels := map[string]string{
		LabelKeyClusterName: clusterName,
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
			Name:            nodeName,
			OwnerReferences: owners,
			Namespace:       r.namespace,
			Labels:          labels,
		},
		Spec: routeSetSpec,
	}

	_, err := r.routeClient.NsxV1alpha1().RouteSets(r.namespace).Create(ctx, routeSet, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return routeSet, nil
		}
		klog.ErrorS(ErrCreateRouteSet, fmt.Sprintf("%v", err))
		return nil, err
	}

	klog.V(6).Infof("Successfully created RouteSet CR for node %s", nodeName)
	return routeSet, nil
}

// checkStaticRouteRealizedState checks static route realized state
// The check happens every 1 second and the default timeout is 10 seconds
func (r *routesProvider) checkStaticRouteRealizedState(routeSetName string) error {
	timeout := time.After(RealizedStateTimeout)
	ticker := time.NewTicker(RealizedStateSleepTime)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return fmt.Errorf("timed out waiting for static route %s", routeSetName)
		case <-ticker.C:
			routeSet, err := r.routeClient.NsxV1alpha1().RouteSets(r.namespace).Get(context.Background(), routeSetName, metav1.GetOptions{})
			if err != nil {
				klog.ErrorS(ErrListRouteSet, fmt.Sprintf("%v", err))
				return err
			}
			condition := GetRouteSetCondition(&(routeSet.Status), v1alpha1.RouteSetConditionTypeReady)
			if condition != nil && condition.Status == v1.ConditionTrue {
				return nil
			}
		}
	}
}

// DeleteRoute implements Routes.DeleteRoute
// Delete node's corresponding RouteSet CR
func (r *routesProvider) DeleteRoute(ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	routeSetName := string(route.TargetNode)
	klog.V(6).Infof("Deleting RouteSet CR %s in cluster %s", routeSetName, clusterName)
	return r.DeleteRouteSetCR(routeSetName)
}

// GetRouteName returns Route name as <nodeName>-<cidr>-<clusterName>
// e.g. nodeName-100.96.0.0-24-clusterName
func (r *routesProvider) GetRouteName(nodeName string, cidr string, clusterName string) string {
	return strings.Replace(nodeName+"-"+cidr+"-"+clusterName, "/", "-", -1)
}

// getNodeIPAddress gets node IP address
// IP family of node address and podCIDR should be the same
// The order is to choose node internal IP first, then external IP
// Return the first IP address as node IP
func (r *routesProvider) getNodeIPAddress(nodeName string, isIPv4 bool) (string, error) {
	node, err := r.getNode(nodeName)
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
			klog.V(4).Info("successfully fetching node %s IP address", node.Name)
			return ip.String(), nil
		}
	}

	return "", fmt.Errorf("node %s does not have the same IP family with podCIDR", nodeName)
}

// AddNode adds v1.Node in nodeMap
func (r *routesProvider) AddNode(node *v1.Node) {
	r.nodeMapLock.Lock()
	r.nodeMap[node.Name] = node
	klog.V(6).Infof("Added node %s into nodeMap", node.Name)
	r.nodeMapLock.Unlock()
}

// DeleteNode deletes v1.Node from nodeMap and removes corresponding RouteSet CR
func (r *routesProvider) DeleteNode(node *v1.Node) {
	r.nodeMapLock.Lock()
	delete(r.nodeMap, node.Name)
	klog.V(6).Infof("Deleted node %s from nodeMap", node.Name)
	r.nodeMapLock.Unlock()

	err := r.DeleteRouteSetCR(node.Name)
	if err != nil {
		klog.Errorf("failed to delete RouteSet CR for node %s: %v", node.Name, err)
	}
}

// DeleteRouteSetCR deletes corresponding RouteSet CR when there is a node deleted
func (r *routesProvider) DeleteRouteSetCR(nodeName string) error {
	if err := r.routeClient.NsxV1alpha1().RouteSets(r.namespace).Delete(context.Background(), nodeName, metav1.DeleteOptions{}); err != nil {
		klog.ErrorS(ErrDeleteRouteSet, fmt.Sprintf("%v", err))
		return err
	}
	klog.V(6).Infof("Successfully deleted RouteSet CR for node %s", nodeName)
	return nil
}

// getNode returns v1.Node from nodeMap
func (r *routesProvider) getNode(nodeName string) (*v1.Node, error) {
	r.nodeMapLock.Lock()
	defer r.nodeMapLock.Unlock()
	if r.nodeMap[nodeName] != nil {
		return r.nodeMap[nodeName], nil
	}
	return nil, fmt.Errorf("node %s not found", nodeName)
}

// GetRouteSetCondition extracts the provided condition from the given RouteSetStatus and returns that.
// Returns nil if the condition is not present.
func GetRouteSetCondition(status *v1alpha1.RouteSetStatus, conditionType v1alpha1.RouteSetConditionType) *v1alpha1.RouteSetCondition {
	if status == nil {
		return nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return &status.Conditions[i]
		}
	}
	return nil
}
