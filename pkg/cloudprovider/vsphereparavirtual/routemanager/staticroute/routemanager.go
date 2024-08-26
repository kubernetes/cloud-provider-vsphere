package staticroute

import (
	"context"
	"fmt"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
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
	clients   nsxclients.Interface
	namespace string
}

// NewRouteManager initializes a RouteManager
func NewRouteManager(config *rest.Config, clusterNS string) (*RouteManager, error) {
	routeClient, err := nsxclients.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create route clients: %w", err)
	}

	return &RouteManager{
		clients:   routeClient,
		namespace: clusterNS,
	}, nil
}

// NewRouteManagerWithClients initializes a RouteManager with clientset
func NewRouteManagerWithClients(clients nsxclients.Interface, clusterNS string) (*RouteManager, error) {
	return &RouteManager{
		clients:   clients,
		namespace: clusterNS,
	}, nil
}

// ListRouteCR lists Route CRs belongd to the namespace and the labelselector
func (sr *RouteManager) ListRouteCR(ctx context.Context, ls metav1.LabelSelector) (helper.RouteCRList, error) {
	return sr.clients.CrdV1alpha1().StaticRoutes(sr.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.Set(ls.MatchLabels).String(),
	})
}

// CreateCPRoutes creates cloudprovider Routes based on Route CR
func (sr *RouteManager) CreateCPRoutes(staticroutes helper.RouteCRList) ([]*cloudprovider.Route, error) {
	routeList, ok := staticroutes.(*vpcapisv1.StaticRouteList)
	if !ok {
		return nil, fmt.Errorf("unknow static route list struct")
	}

	var routes []*cloudprovider.Route
	for _, staticroute := range routeList.Items {
		// only return cloudprovider.RouteInfo if RouteSet CR status 'Ready' is true
		condition := GetRouteCRCondition(&(staticroute.Status), vpcapisv1.Ready)
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
func GetRouteCRCondition(status *vpcapisv1.StaticRouteStatus, conditionType vpcapisv1.ConditionType) *vpcapisv1.StaticRouteCondition {
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
	staticroute, err := sr.clients.CrdV1alpha1().StaticRoutes(sr.namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to list Route CR %s: %w", name, err)
	}
	condition := GetRouteCRCondition(&(staticroute.Status), vpcapisv1.Ready)
	if condition != nil && condition.Status == v1.ConditionTrue {
		return nil
	}

	return fmt.Errorf("Route CR %s is not ready", name)
}

// CreateRouteCR creates the RouteManager CR
func (sr *RouteManager) CreateRouteCR(ctx context.Context, routeInfo *helper.RouteInfo) (helper.RouteCR, error) {
	staticrouteSpec := vpcapisv1.StaticRouteSpec{
		Network: routeInfo.Cidr,
		NextHops: []vpcapisv1.NextHop{
			{IPAddress: routeInfo.NodeIP},
		},
	}
	staticRoute := &vpcapisv1.StaticRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:            routeInfo.Name,
			OwnerReferences: routeInfo.Owner,
			Namespace:       routeInfo.Namespace,
			Labels:          routeInfo.Labels,
		},
		Spec: staticrouteSpec,
	}

	return sr.clients.CrdV1alpha1().StaticRoutes(sr.namespace).Create(ctx, staticRoute, metav1.CreateOptions{})
}

// DeleteRouteCR deletes corresponding RouteSet CR when there is a node deleted
func (sr *RouteManager) DeleteRouteCR(nodeName string) error {
	return sr.clients.CrdV1alpha1().StaticRoutes(sr.namespace).Delete(context.Background(), nodeName, metav1.DeleteOptions{})
}

// GetClients get clientsets. It's used in unit tests
func (sr *RouteManager) GetClients() nsxclients.Interface {
	return sr.clients
}
