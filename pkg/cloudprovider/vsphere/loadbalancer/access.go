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

	"github.com/pkg/errors"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
)

const (
	// ScopeOwner is the owner scope
	ScopeOwner = "owner"
	// ScopeCluster is the cluster scope
	ScopeCluster = "cluster"
	// ScopeService is the service scope
	ScopeService = "service"
	// ScopePort is the port scope
	ScopePort = "port"
	// ScopeIPPoolID is the IP pool id scope
	ScopeIPPoolID = "ippoolid"
	// ScopeLBClass is the load balancer class scope
	ScopeLBClass = "lbclass"
)

type access struct {
	broker       NsxtBroker
	config       *config.LBConfig
	ownerTag     model.Tag
	standardTags Tags
}

var _ NSXTAccess = &access{}

// NewNSXTAccess creates a new NSXTAccess instance
func NewNSXTAccess(broker NsxtBroker, config *config.LBConfig) (NSXTAccess, error) {
	standardTags := Tags{
		ScopeOwner: newTag(ScopeOwner, AppName),
	}
	for k, v := range config.LoadBalancer.AdditionalTags {
		standardTags[k] = newTag(k, v)
	}
	return &access{
		broker:       broker,
		config:       config,
		ownerTag:     standardTags[ScopeOwner],
		standardTags: standardTags,
	}, nil
}

func (a *access) FindIPPoolByName(poolName string) (string, error) {
	list, err := a.broker.ListIPPools()
	if err != nil {
		return "", errors.Wrap(err, "listing IP pools failed")
	}
	for _, item := range list {
		if item.DisplayName != nil && *item.DisplayName == poolName {
			return *item.Id, nil
		}
	}
	return "", fmt.Errorf("load balancer IP pool named %s not found", poolName)
}

func (a *access) CreateLoadBalancerService(clusterName string) (*model.LBService, error) {
	lbService := model.LBService{
		Description:      strptr(fmt.Sprintf("virtual server pool for cluster %s created by %s", clusterName, AppName)),
		DisplayName:      displayName(clusterName),
		Tags:             a.standardTags.Append(clusterTag(clusterName)).Normalize(),
		Size:             strptr(a.config.LoadBalancer.Size),
		Enabled:          boolptr(true),
		ConnectivityPath: strptr(a.config.LoadBalancer.Tier1GatewayPath),
	}
	result, err := a.broker.CreateLoadBalancerService(lbService)
	if err != nil {
		return nil, errors.Wrapf(err, "creating load balancer service failed for cluster %s", clusterName)
	}
	return &result, nil
}

func (a *access) FindLoadBalancerService(clusterName string, id string) (*model.LBService, error) {
	if id == "" {
		return a.findLoadBalancerService(a.ownerTag, clusterTag(clusterName))
	}

	result, err := a.broker.ReadLoadBalancerService(id)
	if err != nil {
		return nil, err
	}
	if a.config.LoadBalancer.Tier1GatewayPath != "" && (result.ConnectivityPath == nil || *result.ConnectivityPath != a.config.LoadBalancer.Tier1GatewayPath) {
		connectivityPath := "nil"
		if result.ConnectivityPath != nil {
			connectivityPath = *result.ConnectivityPath
		}
		return nil, fmt.Errorf("load balancer service %q is configured for router %q not %q",
			*result.Id,
			connectivityPath,
			a.config.LoadBalancer.Tier1GatewayPath,
		)
	}
	return &result, nil
}

func (a *access) findLoadBalancerService(tags ...model.Tag) (*model.LBService, error) {
	list, err := a.broker.ListLoadBalancerServices()
	if err != nil {
		return nil, errors.Wrapf(err, "listing load balancer services failed")
	}
	for _, item := range list {
		if a.config.LoadBalancer.Tier1GatewayPath != "" && item.ConnectivityPath != nil && *item.ConnectivityPath == a.config.LoadBalancer.Tier1GatewayPath {
			return &item, nil
		}
		if checkTags(item.Tags, tags...) {
			return &item, nil
		}
	}
	return nil, nil
}

func (a *access) UpdateLoadBalancerService(lbService *model.LBService) error {
	_, err := a.broker.UpdateLoadBalancerService(*lbService)
	if err != nil {
		return errors.Wrapf(err, "updating load balancer service %s (%s) failed", *lbService.DisplayName, *lbService.Id)
	}
	return nil
}

func (a *access) DeleteLoadBalancerService(id string) error {
	err := a.broker.DeleteLoadBalancerService(id)
	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "deleting load balancer service %s failed", id)
	}
	return nil
}

func (a *access) findAppProfilePathByName(profileName string, resourceType string) (string, error) {
	list, err := a.broker.ListAppProfiles()
	if err != nil {
		return "", err
	}
	path := ""
	for _, item := range list {
		itemResourceType, err := item.String("resource_type")
		if err != nil {
			return "", errors.Wrapf(err, "findAppProfilePathByName cannot find field resource_type")
		}
		itemName, err := item.String("display_name")
		if err != nil {
			return "", errors.Wrapf(err, "findAppProfilePathByName cannot find field name")
		}
		if itemResourceType == resourceType && itemName == profileName {
			if path != "" {
				return "", fmt.Errorf("profile name %s for resource type %s is not unique", profileName, resourceType)
			}
			path, err = item.String("path")
			if err != nil {
				return "", errors.Wrapf(err, "findAppProfilePathByName cannot find field path")
			}
		}
	}
	if path == "" {
		return "", fmt.Errorf("application profile named %s of type %s not found", profileName, resourceType)
	}
	return path, nil
}

func (a *access) GetAppProfilePath(class LBClass, protocol corev1.Protocol) (string, error) {
	profileReference, err := class.AppProfile(protocol)
	if err != nil {
		return "", err
	}
	if profileReference.Identifier != "" {
		return profileReference.Identifier, nil
	}
	resourceType := ""
	switch protocol {
	case corev1.ProtocolTCP:
		resourceType = model.LBAppProfile_RESOURCE_TYPE_LBFASTTCPPROFILE
	case corev1.ProtocolUDP:
		resourceType = model.LBAppProfile_RESOURCE_TYPE_LBFASTUDPPROFILE
	default:
		return "", fmt.Errorf("Unsupported protocol %s", protocol)
	}
	return a.findAppProfilePathByName(profileReference.Name, resourceType)
}

func (a *access) CreateVirtualServer(clusterName string, objectName types.NamespacedName, class LBClass, ipAddress string,
	mapping Mapping, lbServicePath, applicationProfilePath string, poolPath *string) (*model.LBVirtualServer, error) {
	allTags := append(class.Tags(), clusterTag(clusterName), serviceTag(objectName), portTag(mapping))
	virtualServer := model.LBVirtualServer{
		Description: strptr(fmt.Sprintf("virtual server for cluster %s, service %s created by %s",
			clusterName, objectName, AppName)),
		DisplayName:            displayNameObject(clusterName, objectName),
		Tags:                   a.standardTags.Append(allTags...).Normalize(),
		DefaultPoolMemberPorts: []string{fmt.Sprintf("%d", mapping.NodePort)},
		Enabled:                boolptr(true),
		IpAddress:              strptr(ipAddress),
		ApplicationProfilePath: strptr(applicationProfilePath),
		PoolPath:               poolPath,
		Ports:                  []string{fmt.Sprintf("%d", mapping.SourcePort)},
		LbServicePath:          strptr(lbServicePath),
	}
	result, err := a.broker.CreateLoadBalancerVirtualServer(virtualServer)
	if err != nil {
		return nil, errors.Wrapf(err, "creating virtual server failed for %s:%s with IP address %s", clusterName, objectName, ipAddress)
	}
	return &result, nil
}

func (a *access) FindVirtualServers(clusterName string, objectName types.NamespacedName) ([]*model.LBVirtualServer, error) {
	return a.listVirtualServers(a.ownerTag, clusterTag(clusterName), serviceTag(objectName))
}

func (a *access) ListVirtualServers(clusterName string) ([]*model.LBVirtualServer, error) {
	return a.listVirtualServers(a.ownerTag, clusterTag(clusterName))
}

func (a *access) listVirtualServers(tags ...model.Tag) ([]*model.LBVirtualServer, error) {
	list, err := a.broker.ListLoadBalancerVirtualServers()
	if err != nil {
		return nil, errors.Wrapf(err, "listing virtual servers failed")
	}
	var result []*model.LBVirtualServer
	for _, item := range list {
		if checkTags(item.Tags, tags...) {
			itemCopy := item
			result = append(result, &itemCopy)
		}
	}
	return result, nil
}

func (a *access) UpdateVirtualServer(server *model.LBVirtualServer) error {
	_, err := a.broker.UpdateLoadBalancerVirtualServer(*server)
	if err != nil {
		return errors.Wrapf(err, "updating load balancer virtual server %s (%s) failed", *server.DisplayName, *server.Id)
	}
	return nil
}

func (a *access) DeleteVirtualServer(id string) error {
	err := a.broker.DeleteLoadBalancerVirtualServer(id)
	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "deleting virtual server %s failed", id)
	}
	return nil
}

func (a *access) CreatePool(clusterName string, objectName types.NamespacedName, mapping Mapping, members []model.LBPoolMember, activeMonitorPaths []string) (*model.LBPool, error) {
	snatTranslation, err := newNsxtTypeConverter().createLBSnatAutoMap()
	if err != nil {
		return nil, errors.Wrapf(err, "creating pool failed on preparing LBSnatAutoMap failed")
	}
	pool := model.LBPool{
		Description:        strptr(fmt.Sprintf("pool for cluster %s, service %s created by %s", clusterName, objectName, AppName)),
		DisplayName:        displayNameObject(clusterName, objectName),
		Tags:               a.standardTags.Append(clusterTag(clusterName), serviceTag(objectName), portTag(mapping)).Normalize(),
		SnatTranslation:    snatTranslation,
		Members:            members,
		ActiveMonitorPaths: activeMonitorPaths,
	}
	result, err := a.broker.CreateLoadBalancerPool(pool)
	if err != nil {
		return nil, errors.Wrapf(err, "creating pool failed for %s:%s", clusterName, objectName)
	}
	return &result, nil
}

func (a *access) GetPool(id string) (*model.LBPool, error) {
	pool, err := a.broker.ReadLoadBalancerPool(id)
	if err != nil {
		return nil, err
	}
	return &pool, nil
}

func (a *access) FindPool(clusterName string, objectName types.NamespacedName, mapping Mapping) (*model.LBPool, error) {
	list, err := a.broker.ListLoadBalancerPools()
	if err != nil {
		return nil, errors.Wrapf(err, "listing load balancer pools failed")
	}
	tags := []model.Tag{a.ownerTag, clusterTag(clusterName), serviceTag(objectName), portTag(mapping)}
	for _, item := range list {
		if checkTags(item.Tags, tags...) {
			return &item, nil
		}
	}
	return nil, nil
}

func (a *access) FindPools(clusterName string, objectName types.NamespacedName) ([]*model.LBPool, error) {
	return a.listPools(a.ownerTag, clusterTag(clusterName), serviceTag(objectName))
}

func (a *access) ListPools(clusterName string) ([]*model.LBPool, error) {
	return a.listPools(a.ownerTag, clusterTag(clusterName))
}

func (a *access) listPools(tags ...model.Tag) ([]*model.LBPool, error) {
	list, err := a.broker.ListLoadBalancerPools()
	if err != nil {
		return nil, errors.Wrapf(err, "listing pools failed")
	}
	var result []*model.LBPool
	for _, item := range list {
		if checkTags(item.Tags, tags...) {
			itemCopy := item
			result = append(result, &itemCopy)
		}
	}
	return result, nil
}

func (a *access) UpdatePool(pool *model.LBPool) error {
	_, err := a.broker.UpdateLoadBalancerPool(*pool)
	if err != nil {
		return errors.Wrapf(err, "updating load balancer pool %s (%s) failed", *pool.DisplayName, *pool.Id)
	}
	return nil
}

func (a *access) DeletePool(id string) error {
	err := a.broker.DeleteLoadBalancerPool(id)
	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "deleting load balancer pool %s failed", id)
	}
	return nil
}

func (a *access) CreateTCPMonitorProfile(clusterName string, objectName types.NamespacedName, mapping Mapping) (*model.LBTcpMonitorProfile, error) {
	profile := model.LBTcpMonitorProfile{
		Description: strptr(fmt.Sprintf("tcp monitor for cluster %s, service %s, port %d created by %s",
			clusterName, objectName, mapping.NodePort, AppName)),
		DisplayName: displayNameMapping(clusterName, objectName, mapping),
		Tags:        a.standardTags.Append(clusterTag(clusterName), serviceTag(objectName), portTag(mapping)).Normalize(),
		MonitorPort: int64ptr(int64(mapping.NodePort)),
	}
	monitor, err := a.broker.CreateLoadBalancerTCPMonitorProfile(profile)
	if err != nil {
		return nil, errors.Wrapf(err, "creating tcp monitor failed for %s:%s:%d", clusterName, objectName, mapping.NodePort)
	}
	return &monitor, nil
}

func (a *access) GetTCPMonitorProfile(id string) (*model.LBTcpMonitorProfile, error) {
	monitor, err := a.broker.ReadLoadBalancerTCPMonitorProfile(id)
	if err != nil {
		return nil, errors.Wrapf(err, "reading tcp monitor %s failed", id)
	}
	return &monitor, nil
}

func (a *access) FindTCPMonitorProfiles(clusterName string, objectName types.NamespacedName) ([]*model.LBTcpMonitorProfile, error) {
	return a.listTCPMonitorProfiles(a.ownerTag, clusterTag(clusterName), serviceTag(objectName))
}

func (a *access) ListTCPMonitorProfiles(clusterName string) ([]*model.LBTcpMonitorProfile, error) {
	return a.listTCPMonitorProfiles(a.ownerTag, clusterTag(clusterName))
}

func (a *access) listTCPMonitorProfiles(tags ...model.Tag) ([]*model.LBTcpMonitorProfile, error) {
	list, err := a.broker.ListLoadBalancerMonitorProfiles()
	if err != nil {
		return nil, errors.Wrapf(err, "listing load balancer monitors failed")
	}
	result := []*model.LBTcpMonitorProfile{}
	converter := newNsxtTypeConverter()
	for _, item := range list {
		resourceType, err := item.String("resource_type")
		if err != nil || resourceType != model.LBMonitorProfile_RESOURCE_TYPE_LBTCPMONITORPROFILE {
			continue
		}
		profile, err := converter.convertStructValueToLBTCPMonitorProfile(item)
		if err != nil {
			return nil, err
		}
		if checkTags(profile.Tags, tags...) {
			result = append(result, &profile)
		}
	}
	return result, nil
}

func (a *access) UpdateTCPMonitorProfile(monitor *model.LBTcpMonitorProfile) error {
	_, err := a.broker.UpdateLoadBalancerTCPMonitorProfile(*monitor)
	if err != nil {
		return errors.Wrapf(err, "updating load balancer TCP monitor %s (%s) failed", *monitor.DisplayName, *monitor.Id)
	}
	return nil
}

func (a *access) DeleteTCPMonitorProfile(id string) error {
	err := a.broker.DeleteLoadBalancerMonitorProfile(id)
	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "deleting monitor %s failed", id)
	}
	return nil
}

func (a *access) AllocateExternalIPAddress(ipPoolID string, clusterName string, objectName types.NamespacedName) (*model.IpAddressAllocation, *string, error) {
	allocation := model.IpAddressAllocation{
		Tags: a.standardTags.Append(clusterTag(clusterName), serviceTag(objectName)).Normalize(),
	}
	allocated, ipAdress, err := a.broker.AllocateFromIPPool(ipPoolID, allocation)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "allocating external IP address failed")
	}
	return &allocated, &ipAdress, nil
}

func (a *access) FindExternalIPAddressForObject(ipPoolID string, clusterName string, objectName types.NamespacedName) (*model.IpAddressAllocation, *string, error) {
	results, err := a.findExternalIPAddresses(ipPoolID, a.ownerTag, clusterTag(clusterName), serviceTag(objectName))
	if err != nil {
		return nil, nil, err
	}
	if len(results) == 0 {
		return nil, nil, nil
	}
	if len(results) > 1 {
		return nil, nil, fmt.Errorf("Multiple IP address allocations")
	}

	item := results[0]
	ipAddress := item.AllocationIp
	if ipAddress == nil {
		ipAddress, err = a.broker.GetRealizedExternalIPAddress(*item.Path, 5*time.Second)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "GetReleaziedExternalIPAddress failed for allocation %s IP pool %s failed", *item.Path, ipPoolID)
		}
	}

	return item, ipAddress, nil
}

func (a *access) ListExternalIPAddresses(ipPoolID string, clusterName string) ([]*model.IpAddressAllocation, error) {
	return a.findExternalIPAddresses(ipPoolID, a.ownerTag, clusterTag(clusterName))
}

func (a *access) findExternalIPAddresses(ipPoolID string, tags ...model.Tag) ([]*model.IpAddressAllocation, error) {
	list, err := a.broker.ListIPPoolAllocations(ipPoolID)
	if err != nil {
		return nil, errors.Wrapf(err, "listing IP address allocations from IP pool %s failed", ipPoolID)
	}
	results := []*model.IpAddressAllocation{}
	for _, item := range list {
		if checkTags(item.Tags, tags...) {
			itemCopy := item
			results = append(results, &itemCopy)
		}
	}
	return results, nil
}

func (a *access) ReleaseExternalIPAddress(ipPoolID string, id string) error {
	err := a.broker.ReleaseFromIPPool(ipPoolID, id)
	if isNotFoundError(err) {
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "releasing external IP address allocation id=%s failed", id)
	}
	return nil
}

func displayName(clusterName string) *string {
	return strptr(fmt.Sprintf("cluster:%s", clusterName))
}

func displayNameObject(clusterName string, objectName types.NamespacedName) *string {
	return strptr(fmt.Sprintf("cluster:%s:%s", clusterName, objectName))
}

func displayNameMapping(clusterName string, objectName types.NamespacedName, mapping Mapping) *string {
	return strptr(fmt.Sprintf("cluster:%s:%s:%d", clusterName, objectName, mapping.NodePort))
}
