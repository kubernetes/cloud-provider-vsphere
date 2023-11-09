package routemanager

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/routeset"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/staticroute"
)

// RouteManager defines an interface that can interact with nsx.vmware.com route CR
type RouteManager interface {
	ListRouteCR(ctx context.Context, ls metav1.LabelSelector) (helper.RouteCRList, error)
	CreateRouteCR(ctx context.Context, routeInfo *helper.RouteInfo) (helper.RouteCR, error)
	DeleteRouteCR(route string) error
	WaitRouteCR(crName string) error

	CreateCPRoutes(routes helper.RouteCRList) ([]*cloudprovider.Route, error)
}

// GetRouteManager gets an RouteManager
func GetRouteManager(vpcModeEnabled bool, config *rest.Config, clusterNS string) (RouteManager, error) {
	if vpcModeEnabled {
		return staticroute.NewRouteManager(config, clusterNS)
	}

	return routeset.NewRouteManager(config, clusterNS)
}
