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

package loadbalancer

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/vmware/vsphere-automation-sdk-go/lib/vapi/std"
	vapi_errors "github.com/vmware/vsphere-automation-sdk-go/lib/vapi/std/errors"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/bindings"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/protocol/client"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/ip_pools"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/realized_state"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

// NsxtBroker is an internal interface to enable mocking the nsxt backend
type NsxtBroker interface {
	ReadLoadBalancerService(id string) (model.LBService, error)
	CreateLoadBalancerService(service model.LBService) (model.LBService, error)
	ListLoadBalancerServices() ([]model.LBService, error)
	UpdateLoadBalancerService(service model.LBService) (model.LBService, error)
	DeleteLoadBalancerService(id string) error
	CreateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error)
	ListLoadBalancerVirtualServers() ([]model.LBVirtualServer, error)
	UpdateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error)
	DeleteLoadBalancerVirtualServer(id string) error
	CreateLoadBalancerPool(pool model.LBPool) (model.LBPool, error)
	ReadLoadBalancerPool(id string) (model.LBPool, error)
	ListLoadBalancerPools() ([]model.LBPool, error)
	UpdateLoadBalancerPool(pool model.LBPool) (model.LBPool, error)
	DeleteLoadBalancerPool(id string) error
	ListIPPools() ([]model.IpAddressPool, error)
	AllocateFromIPPool(ipPoolID string, allocation model.IpAddressAllocation) (model.IpAddressAllocation, string, error)
	ListIPPoolAllocations(ipPoolID string) ([]model.IpAddressAllocation, error)
	ReleaseFromIPPool(ipPoolID, ipAllocationID string) error
	GetRealizedExternalIPAddress(ipAllocationPath string, timeout time.Duration) (*string, error)
	ListAppProfiles() ([]*data.StructValue, error)

	CreateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error)
	ListLoadBalancerMonitorProfiles() ([]*data.StructValue, error)
	ReadLoadBalancerTCPMonitorProfile(id string) (model.LBTcpMonitorProfile, error)
	UpdateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error)
	DeleteLoadBalancerMonitorProfile(id string) error
}

type nsxtBroker struct {
	lbServicesClient        infra.LbServicesClient
	lbVirtServersClient     infra.LbVirtualServersClient
	lbPoolsClient           infra.LbPoolsClient
	ipPoolsClient           infra.IpPoolsClient
	ipAllocationsClient     ip_pools.IpAllocationsClient
	lbAppProfilesClient     infra.LbAppProfilesClient
	lbMonitorProfilesClient infra.LbMonitorProfilesClient
	realizedEntitiesClient  realized_state.RealizedEntitiesClient
}

// NewNsxtBroker creates a new NsxtBroker using the configuration
func NewNsxtBroker(connector client.Connector) (NsxtBroker, error) {
	// perform API call to check connector
	_, err := infra.NewDefaultLbMonitorProfilesClient(connector).List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Connection to NSX-T API failed. Please check your connection settings.")
	}
	return NewNsxtBrokerFromConnector(connector), nil
}

// NewNsxtBrokerFromConnector creates a new NsxtBroker to the real API
func NewNsxtBrokerFromConnector(connector client.Connector) NsxtBroker {
	return &nsxtBroker{
		lbServicesClient:        infra.NewDefaultLbServicesClient(connector),
		lbVirtServersClient:     infra.NewDefaultLbVirtualServersClient(connector),
		lbPoolsClient:           infra.NewDefaultLbPoolsClient(connector),
		ipPoolsClient:           infra.NewDefaultIpPoolsClient(connector),
		ipAllocationsClient:     ip_pools.NewDefaultIpAllocationsClient(connector),
		lbAppProfilesClient:     infra.NewDefaultLbAppProfilesClient(connector),
		lbMonitorProfilesClient: infra.NewDefaultLbMonitorProfilesClient(connector),
		realizedEntitiesClient:  realized_state.NewDefaultRealizedEntitiesClient(connector),
	}
}

func (b *nsxtBroker) ReadLoadBalancerService(id string) (model.LBService, error) {
	return b.lbServicesClient.Get(id)
}

func (b *nsxtBroker) CreateLoadBalancerService(service model.LBService) (model.LBService, error) {
	id := uuid.New().String()
	result, err := b.lbServicesClient.Update(id, service)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerServices() ([]model.LBService, error) {
	result, err := b.lbServicesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbServicesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerService(service model.LBService) (model.LBService, error) {
	result, err := b.lbServicesClient.Update(*service.Id, service)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerService(id string) error {
	err := b.lbServicesClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) CreateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error) {
	id := uuid.New().String()
	result, err := b.lbVirtServersClient.Update(id, server)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerVirtualServers() ([]model.LBVirtualServer, error) {
	result, err := b.lbVirtServersClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbVirtServersClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error) {
	result, err := b.lbVirtServersClient.Update(*server.Id, server)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerVirtualServer(id string) error {
	err := b.lbVirtServersClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) CreateLoadBalancerPool(pool model.LBPool) (model.LBPool, error) {
	id := uuid.New().String()
	result, err := b.lbPoolsClient.Update(id, pool)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ReadLoadBalancerPool(id string) (model.LBPool, error) {
	result, err := b.lbPoolsClient.Get(id)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerPools() ([]model.LBPool, error) {
	result, err := b.lbPoolsClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbPoolsClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerPool(pool model.LBPool) (model.LBPool, error) {
	result, err := b.lbPoolsClient.Update(*pool.Id, pool)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerPool(id string) error {
	err := b.lbPoolsClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) ListAppProfiles() ([]*data.StructValue, error) {
	result, err := b.lbAppProfilesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbAppProfilesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) CreateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	id := uuid.New().String()
	result, err := b.createOrUpdateLoadBalancerTCPMonitorProfile(id, monitor)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) createOrUpdateLoadBalancerTCPMonitorProfile(id string, monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	monitor.ResourceType = model.LBMonitorProfile_RESOURCE_TYPE_LBTCPMONITORPROFILE
	converter := newNsxtTypeConverter()
	value, err := converter.convertLBTCPMonitorProfileToStructValue(monitor)
	if err != nil {
		return model.LBTcpMonitorProfile{}, errors.Wrapf(err, "converting LBTcpMonitorProfile failed")
	}
	result, err := b.lbMonitorProfilesClient.Update(id, value)
	if err != nil {
		return model.LBTcpMonitorProfile{}, nicerVAPIError(err)
	}
	return converter.convertStructValueToLBTCPMonitorProfile(result)
}

func (b *nsxtBroker) ListLoadBalancerMonitorProfiles() ([]*data.StructValue, error) {
	result, err := b.lbMonitorProfilesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbMonitorProfilesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) ReadLoadBalancerTCPMonitorProfile(id string) (model.LBTcpMonitorProfile, error) {
	itf, err := b.lbMonitorProfilesClient.Get(id)
	if err != nil {
		return model.LBTcpMonitorProfile{}, errors.Wrapf(nicerVAPIError(err), "getting LBTcpMonitorProfile %s failed", id)
	}
	return newNsxtTypeConverter().convertStructValueToLBTCPMonitorProfile(itf)
}

func (b *nsxtBroker) UpdateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	result, err := b.createOrUpdateLoadBalancerTCPMonitorProfile(*monitor.Id, monitor)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerMonitorProfile(id string) error {
	err := b.lbMonitorProfilesClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) ListIPPools() ([]model.IpAddressPool, error) {
	result, err := b.ipPoolsClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.ipPoolsClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) AllocateFromIPPool(ipPoolID string, allocation model.IpAddressAllocation) (model.IpAddressAllocation, string, error) {
	id := uuid.New().String()
	err := b.ipAllocationsClient.Patch(ipPoolID, id, allocation)
	if err != nil {
		return allocation, "", nicerVAPIError(err)
	}
	allocated, err := b.ipAllocationsClient.Get(ipPoolID, id)
	if err != nil {
		return allocation, "", nicerVAPIError(err)
	}
	ipAddress, err := b.GetRealizedExternalIPAddress(*allocated.Path, 15*time.Second)
	if err != nil {
		return allocated, "", nicerVAPIError(err)
	}
	if ipAddress == nil {
		return allocated, "", fmt.Errorf("no IP address allocated for %s", *allocated.Path)
	}
	return allocated, *ipAddress, nil
}

func (b *nsxtBroker) ListIPPoolAllocations(ipPoolID string) ([]model.IpAddressAllocation, error) {
	result, err := b.ipAllocationsClient.List(ipPoolID, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.ipAllocationsClient.List(ipPoolID, result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) ReleaseFromIPPool(ipPoolID, ipAllocationID string) error {
	err := b.ipAllocationsClient.Delete(ipPoolID, ipAllocationID)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) GetRealizedExternalIPAddress(ipAllocationPath string, timeout time.Duration) (*string, error) {
	// wait for realized state
	limit := time.Now().Add(timeout)
	sleepIncr := 100 * time.Millisecond
	sleepMax := 1000 * time.Millisecond
	sleep := sleepIncr
	for time.Now().Before(limit) {
		time.Sleep(sleep)
		sleep += sleepIncr
		if sleep > sleepMax {
			sleep = sleepMax
		}
		list, err := b.realizedEntitiesClient.List(ipAllocationPath, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		for _, realizedResource := range list.Results {
			for _, attr := range realizedResource.ExtendedAttributes {
				if *attr.Key == "allocation_ip" {
					return &attr.Values[0], nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Timeout of wait for realized state of IP allocation")
}

func nicerVAPIError(err error) error {
	switch vapiError := err.(type) {
	case vapi_errors.InvalidRequest:
		// Connection errors end up here
		return nicerVapiErrorData("InvalidRequest", vapiError.Data, vapiError.Messages)
	case vapi_errors.NotFound:
		return nicerVapiErrorData("NotFound", vapiError.Data, vapiError.Messages)
	case vapi_errors.Unauthorized:
		return nicerVapiErrorData("Unauthorized", vapiError.Data, vapiError.Messages)
	case vapi_errors.Unauthenticated:
		return nicerVapiErrorData("Unauthenticated", vapiError.Data, vapiError.Messages)
	case vapi_errors.InternalServerError:
		return nicerVapiErrorData("InternalServerError", vapiError.Data, vapiError.Messages)
	case vapi_errors.ServiceUnavailable:
		return nicerVapiErrorData("ServiceUnavailable", vapiError.Data, vapiError.Messages)
	}

	return err
}

func nicerVapiErrorData(errorMsg string, apiErrorDataValue *data.StructValue, messages []std.LocalizableMessage) error {
	if apiErrorDataValue == nil {
		if len(messages) > 0 {
			return fmt.Errorf("%s (%s)", errorMsg, messages[0].DefaultMessage)
		}
		return fmt.Errorf("%s (no additional details provided)", errorMsg)
	}

	var typeConverter = bindings.NewTypeConverter()
	typeConverter.SetMode(bindings.REST)
	rawData, err := typeConverter.ConvertToGolang(apiErrorDataValue, model.ApiErrorBindingType())

	if err != nil {
		return fmt.Errorf("%s (failed to extract additional details due to %s)", errorMsg, err)
	}
	apiError := rawData.(model.ApiError)
	details := fmt.Sprintf(" %s: %s (code %v)", errorMsg, *apiError.ErrorMessage, *apiError.ErrorCode)

	if len(apiError.RelatedErrors) > 0 {
		details += "\nRelated errors:\n"
		for _, relatedErr := range apiError.RelatedErrors {
			details += fmt.Sprintf("%s (code %v)", *relatedErr.ErrorMessage, relatedErr.ErrorCode)
		}
	}
	return fmt.Errorf(details)
}
