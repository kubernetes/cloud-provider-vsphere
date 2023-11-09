package staticroute

import (
	"context"
	"fmt"

	vpcnetworkingapis "github.com/vmware-tanzu/nsx-operator/pkg/apis/nsx.vmware.com/v1alpha1"
	vpcnetworkingclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
)

// RouteManager defines a route manager working with static route CR
type RouteManager struct {
	clients   vpcnetworkingclients.Interface
	namespace string
}

// NewRouteManager initializes a RouteManager
func NewRouteManager(config *rest.Config, clusterNS string) (*RouteManager, error) {
	routeClient, err := vpcnetworkingclients.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create route clients: %w", err)
	}

	return &RouteManager{
		clients:   routeClient,
		namespace: clusterNS,
	}, nil
}

// NewRouteManagerWithClients initializes a RouteManager with clientset
func NewRouteManagerWithClients(clients vpcnetworkingclients.Interface, clusterNS string) (*RouteManager, error) {
	return &RouteManager{
		clients:   clients,
		namespace: clusterNS,
	}, nil
}

// ListRouteCR lists Route CRs belongd to the namespace and the labelselector
func (sr *RouteManager) ListRouteCR(ctx context.Context, ls metav1.LabelSelector) (helper.RouteCRList, error) {
	return sr.clients.NsxV1alpha1().StaticRoutes(sr.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(ls.MatchLabels).String(),
	})
}

// CreateCPRoutes creates cloudprovider Routes based on Route CR
func (sr *RouteManager) CreateCPRoutes(staticroutes helper.RouteCRList) ([]*cloudprovider.Route, error) {
	routeList, ok := staticroutes.(*vpcnetworkingapis.StaticRouteList)
	if !ok {
		return nil, fmt.Errorf("unknow static route list struct")
	}

	var routes []*cloudprovider.Route
	for _, staticroute := range routeList.Items {
		// only return cloudprovider.RouteInfo if RouteSet CR status 'Ready' is true
		condition := GetRouteCRCondition(&(staticroute.Status), vpcnetworkingapis.Ready)
		if condition != nil && condition.Status == v1.ConditionTrue {
			// one RouteSet per node, so we can use nodeName as the name of RouteSet CR
			nodeName := staticroute.Name
			cpRoute := &cloudprovider.Route{
				Name:            staticroute.Name,
				TargetNode:      types.NodeName(nodeName),
				DestinationCIDR: staticroute.Spec.Network,
			}
			routes = append(routes, cpRoute)
		}
	}

	return routes, nil
}

// GetRouteCRCondition extracts the provided condition from the given StaticRouteStatus and returns that.
// Returns nil if the condition is not present.
func GetRouteCRCondition(status *vpcnetworkingapis.StaticRouteStatus, conditionType vpcnetworkingapis.ConditionType) *vpcnetworkingapis.StaticRouteCondition {
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
func (sr *RouteManager) WaitRouteCR(name string) error {
	staticroute, err := sr.clients.NsxV1alpha1().StaticRoutes(sr.namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Route CR %s: %w", name, err)
	}
	condition := GetRouteCRCondition(&(staticroute.Status), vpcnetworkingapis.Ready)
	if condition != nil && condition.Status == v1.ConditionTrue {
		return nil
	}

	return fmt.Errorf("Route CR %s is not ready", name)
}

// CreateRouteCR creates the RouteManager CR
func (sr *RouteManager) CreateRouteCR(ctx context.Context, routeInfo *helper.RouteInfo) (helper.RouteCR, error) {
	staticrouteSpec := vpcnetworkingapis.StaticRouteSpec{
		Network: routeInfo.Cidr,
		NextHops: []vpcnetworkingapis.NextHop{
			{IPAddress: routeInfo.NodeIP},
		},
	}
	staticRoute := &vpcnetworkingapis.StaticRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:            routeInfo.Name,
			OwnerReferences: routeInfo.Owner,
			Namespace:       routeInfo.Namespace,
			Labels:          routeInfo.Labels,
		},
		Spec: staticrouteSpec,
	}

	return sr.clients.NsxV1alpha1().StaticRoutes(sr.namespace).Create(ctx, staticRoute, metav1.CreateOptions{})
}

// DeleteRouteCR deletes corresponding RouteSet CR when there is a node deleted
func (sr *RouteManager) DeleteRouteCR(nodeName string) error {
	return sr.clients.NsxV1alpha1().StaticRoutes(sr.namespace).Delete(context.Background(), nodeName, metav1.DeleteOptions{})
}

// GetClients get clientsets. It's used in unit tests
func (sr *RouteManager) GetClients() vpcnetworkingclients.Interface {
	return sr.clients
}
