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
	"strings"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/realized_state"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/tier_1s"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/search"

	"k8s.io/cloud-provider-vsphere/pkg/nsxt"
	nsxtcfg "k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
)

// NsxtBroker is an internal interface to access nsxt backend
type NsxtBroker interface {
	QueryEntities(queryParam string) (model.SearchResponse, error)
	CreateStaticRoute(routerPath string, staticRouteID string, staticRoute model.StaticRoutes) error
	DeleteStaticRoute(routerPath string, staticRouteID string) error
	ListRealizedEntities(path string) (model.GenericPolicyRealizedResourceListResult, error)
	GetSegment(path string) (model.Segment, error)
}

// nsxtBroker includes NSXT API clients
type nsxtBroker struct {
	// TODO: will add tier0 static routes client
	tier1StaticRoutesClient tier_1s.StaticRoutesClient
	realizedEntitiesClient  realized_state.RealizedEntitiesClient
	queryClient             *search.DefaultQueryClient
	segmentClient           *infra.DefaultSegmentsClient
}

// NewNsxtBroker creates a new NsxtBroker to the NSXT API
func NewNsxtBroker(nsxtConfig *nsxtcfg.NsxtConfig) (NsxtBroker, error) {
	connector, err := nsxt.NewNsxtConnector(nsxtConfig)
	if err != nil {
		return nil, err
	}
	return &nsxtBroker{
		tier1StaticRoutesClient: tier_1s.NewDefaultStaticRoutesClient(connector),
		realizedEntitiesClient:  realized_state.NewDefaultRealizedEntitiesClient(connector),
		queryClient:             search.NewDefaultQueryClient(connector),
		segmentClient:           infra.NewDefaultSegmentsClient(connector),
	}, nil
}

func (b *nsxtBroker) QueryEntities(queryParam string) (model.SearchResponse, error) {
	queryParam = strings.ReplaceAll(queryParam, "/", "\\/")
	return b.queryClient.List(queryParam, nil, nil, nil, nil, nil)
}

func (b *nsxtBroker) CreateStaticRoute(routerPath string, staticRouteID string, staticRoute model.StaticRoutes) error {
	routerID := getResourceID(routerPath)
	return b.tier1StaticRoutesClient.Patch(routerID, staticRouteID, staticRoute)
}

func (b *nsxtBroker) DeleteStaticRoute(routerPath string, staticRouteID string) error {
	routerID := getResourceID(routerPath)
	return b.tier1StaticRoutesClient.Delete(routerID, staticRouteID)
}

func (b *nsxtBroker) ListRealizedEntities(path string) (model.GenericPolicyRealizedResourceListResult, error) {
	return b.realizedEntitiesClient.List(path, nil)
}

func (b *nsxtBroker) GetSegment(path string) (model.Segment, error) {
	segmentID := getResourceID(path)
	return b.segmentClient.Get(segmentID)
}

// getResourceID returns ID from path
func getResourceID(resourcePath string) string {
	path := strings.Split(resourcePath, "/")
	return path[len(path)-1]
}
