package routeset

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	t1networkingapis "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	t1networkingclients "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
)

// RouteManager defines a route manager working with routeset CR
type RouteManager struct {
	clients   t1networkingclients.Interface
	namespace string
}

// NewRouteManager initializes a RouteManager
func NewRouteManager(config *rest.Config, clusterNS string) (*RouteManager, error) {
	routeClient, err := t1networkingclients.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create route clients: %w", err)
	}

	return &RouteManager{
		clients:   routeClient,
		namespace: clusterNS,
	}, nil
}

// NewRouteManagerWithClients initializes a RouteManager with clientset
func NewRouteManagerWithClients(clients t1networkingclients.Interface, clusterNS string) (*RouteManager, error) {
	return &RouteManager{
		clients:   clients,
		namespace: clusterNS,
	}, nil
}

// ListRouteCR lists Route CRs belongd to the namespace and the labelselector
func (rs *RouteManager) ListRouteCR(ctx context.Context, ls metav1.LabelSelector) (helper.RouteCRList, error) {
	return rs.clients.NsxV1alpha1().RouteSets(rs.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(ls.MatchLabels).String(),
	})
}

// CreateCPRoutes creates cloudprovider Routes based on Route CR
func (rs *RouteManager) CreateCPRoutes(routeSets helper.RouteCRList) ([]*cloudprovider.Route, error) {
	routeList, ok := routeSets.(*t1networkingapis.RouteSetList)
	if !ok {
		return nil, fmt.Errorf("unknow static route list struct")
	}

	var routes []*cloudprovider.Route
	for _, routeSet := range routeList.Items {
		// only return cloudprovider.RouteInfo if RouteManager CR status 'Ready' is true
		condition := GetRouteCRCondition(&(routeSet.Status), t1networkingapis.RouteSetConditionTypeReady)
		if condition != nil && condition.Status == v1.ConditionTrue {
			// one RouteManager per node, so we can use nodeName as the name of RouteManager CR
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
	return routes, nil
}

// GetRouteCRCondition extracts the provided condition from the given RouteSetStatus and returns that.
// Returns nil if the condition is not present.
func GetRouteCRCondition(status *t1networkingapis.RouteSetStatus, conditionType t1networkingapis.RouteSetConditionType) *t1networkingapis.RouteSetCondition {
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

// WaitRouteCR validates if route CR condition is Ready
func (rs *RouteManager) WaitRouteCR(name string) error {
	routeSet, err := rs.clients.NsxV1alpha1().RouteSets(rs.namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to list route set %s: %w", name, err)
	}
	condition := GetRouteCRCondition(&(routeSet.Status), t1networkingapis.RouteSetConditionTypeReady)
	if condition != nil && condition.Status == v1.ConditionTrue {
		return nil
	}

	return fmt.Errorf("route set %s is not ready", name)
}

// CreateRouteCR creates RouteManager CR
func (rs *RouteManager) CreateRouteCR(ctx context.Context, routeInfo *helper.RouteInfo) (helper.RouteCR, error) {
	route := t1networkingapis.Route{
		Name:        routeInfo.RouteName,
		Destination: routeInfo.Cidr,
		Target:      routeInfo.NodeIP,
	}
	routeSetSpec := t1networkingapis.RouteSetSpec{
		Routes: []t1networkingapis.Route{
			route,
		},
	}
	routeSet := &t1networkingapis.RouteSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            routeInfo.Name,
			OwnerReferences: routeInfo.Owner,
			Namespace:       routeInfo.Namespace,
			Labels:          routeInfo.Labels,
		},
		Spec: routeSetSpec,
	}

	return rs.clients.NsxV1alpha1().RouteSets(rs.namespace).Create(ctx, routeSet, metav1.CreateOptions{})
}

// DeleteRouteCR deletes corresponding RouteManager CR when there is a node deleted
func (rs *RouteManager) DeleteRouteCR(nodeName string) error {
	return rs.clients.NsxV1alpha1().RouteSets(rs.namespace).Delete(context.Background(), nodeName, metav1.DeleteOptions{})
}
