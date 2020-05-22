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
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
)

type state struct {
	*lbService
	klog.Verbose
	clusterName    string
	objectName     types.NamespacedName
	service        *corev1.Service
	nodes          []*corev1.Node
	servers        []*model.LBVirtualServer
	pools          []*model.LBPool
	tcpMonitors    []*model.LBTcpMonitorProfile
	ipAddressAlloc *model.IpAddressAllocation
	ipAddress      *string
	class          *loadBalancerClass
}

func newState(lbService *lbService, clusterName string, service *corev1.Service, nodes []*corev1.Node) *state {
	return &state{
		lbService:   lbService,
		clusterName: clusterName,
		service:     service,
		nodes:       nodes,
		objectName:  namespacedNameFromService(service),
		Verbose:     klog.V(klog.Level(2)),
	}
}

// CxtInfof logs with object name context
func (s *state) CtxInfof(format string, args ...interface{}) {
	if s.Verbose {
		s.Infof("%s: %s", s.objectName, fmt.Sprintf(format, args...))
	}
}

// Process processes a load balancer and ensures that all needed objects are existing
func (s *state) Process(class *loadBalancerClass) error {
	var err error
	s.ipAddressAlloc, s.ipAddress, err = s.access.FindExternalIPAddressForObject(class.ipPool.Identifier, s.clusterName, s.objectName)
	if err != nil {
		return err
	}
	s.servers, err = s.access.FindVirtualServers(s.clusterName, s.objectName)
	if err != nil {
		return err
	}
	s.pools, err = s.access.FindPools(s.clusterName, s.objectName)
	if err != nil {
		return err
	}
	s.tcpMonitors, err = s.access.FindTCPMonitorProfiles(s.clusterName, s.objectName)
	if err != nil {
		return err
	}
	if len(s.servers) > 0 {
		className := getTag(s.servers[0].Tags, ScopeLBClass)
		ipPoolID := getTag(s.servers[0].Tags, ScopeIPPoolID)
		if class.className != className || class.ipPool.Identifier != ipPoolID {
			classConfig := &config.LoadBalancerClassConfig{
				IPPoolID: ipPoolID,
			}
			class, err = newLBClass(className, classConfig, class, nil)
			if err != nil {
				return err
			}
		}
	}
	s.class = class

	for _, servicePort := range s.service.Spec.Ports {
		mapping := NewMapping(servicePort)

		monitor, err := s.getTCPMonitor(mapping)
		if err != nil {
			return err
		}
		pool, err := s.getPool(mapping, monitor)
		if err != nil {
			return err
		}
		_, err = s.getVirtualServer(mapping, pool.Path)
		if err != nil {
			return err
		}
	}
	validPoolPaths, err := s.deleteOrphanVirtualServers()
	if err != nil {
		return err
	}
	s.CtxInfof("validPoolPaths: %v", validPoolPaths.List())
	validTCPMonitorPaths, err := s.deleteOrphanPools(validPoolPaths)
	if err != nil {
		return err
	}
	s.CtxInfof("validTCPMonitorPaths: %v", validTCPMonitorPaths.List())
	err = s.deleteOrphanTCPMonitors(validTCPMonitorPaths)
	if err != nil {
		return err
	}
	return nil
}

func (s *state) deleteOrphanVirtualServers() (sets.String, error) {
	validPoolPaths := sets.String{}
	for _, server := range s.servers {
		found := false
		for _, servicePort := range s.service.Spec.Ports {
			mapping := NewMapping(servicePort)
			if mapping.MatchVirtualServer(server) {
				if server.PoolPath != nil {
					validPoolPaths.Insert(*server.PoolPath)
				}
				found = true
				break
			}
		}
		if !found {
			err := s.deleteVirtualServer(server)
			if err != nil {
				return nil, err
			}
		}
	}
	return validPoolPaths, nil
}

func (s *state) deleteOrphanPools(validPoolPaths sets.String) (sets.String, error) {
	validTCPMonitorPaths := sets.String{}
	for _, pool := range s.pools {
		found := false
		for _, servicePort := range s.service.Spec.Ports {
			mapping := NewMapping(servicePort)
			if mapping.MatchPool(pool) && validPoolPaths.Has(*pool.Path) {
				if len(pool.ActiveMonitorPaths) > 0 {
					validTCPMonitorPaths.Insert(pool.ActiveMonitorPaths...)
				}
				found = true
				break
			}
		}
		if !found {
			err := s.deletePool(pool)
			if err != nil {
				return nil, err
			}
		}
	}
	return validTCPMonitorPaths, nil
}

func (s *state) deleteOrphanTCPMonitors(validTCPMonitorPaths sets.String) error {
	for _, monitor := range s.tcpMonitors {
		found := false
		for _, servicePort := range s.service.Spec.Ports {
			mapping := NewMapping(servicePort)
			if mapping.MatchTCPMonitor(monitor) && monitor.Path != nil && validTCPMonitorPaths.Has(*monitor.Path) {
				found = true
				break
			}
		}
		if !found {
			err := s.deleteTCPMonitor(monitor)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *state) allocateResources() (allocated bool, err error) {
	if s.ipAddressAlloc == nil {
		ipPoolID := s.class.ipPool.Identifier
		s.ipAddressAlloc, s.ipAddress, err = s.access.AllocateExternalIPAddress(ipPoolID, s.clusterName, s.objectName)
		if err != nil {
			return
		}
		allocated = true
		s.CtxInfof("allocated IP address %s from pool %s", *s.ipAddress, ipPoolID)
	}
	return
}

func (s *state) releaseResources() error {
	if s.ipAddressAlloc != nil {
		ipPoolID := s.class.ipPool.Identifier
		err := s.access.ReleaseExternalIPAddress(ipPoolID, *s.ipAddressAlloc.Id)
		if err != nil {
			return err
		}
		s.ipAddressAlloc = nil
		s.ipAddress = nil
	}
	return nil
}

func (s *state) loggedReleaseResources() {
	ipAddress := s.ipAddress
	err := s.releaseResources()
	if err != nil {
		s.CtxInfof("failed to release IP address %s to pool %s", *ipAddress, s.class.ipPool.Identifier)
	}
}

// Finish performs cleanup after Process
func (s *state) Finish() (*corev1.LoadBalancerStatus, error) {
	if len(s.service.Spec.Ports) == 0 {
		err := s.releaseResources()
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	return newLoadBalancerStatus(s.ipAddress), nil
}

func (s *state) getTCPMonitor(mapping Mapping) (*model.LBTcpMonitorProfile, error) {
	if mapping.Protocol == corev1.ProtocolTCP {
		for _, m := range s.tcpMonitors {
			if mapping.MatchTCPMonitor(m) {
				err := s.updateTCPMonitor(m, mapping)
				if err != nil {
					return nil, err
				}
				return m, nil
			}
		}
		return s.createTCPMonitor(mapping)
	}
	return nil, nil
}

func (s *state) createTCPMonitor(mapping Mapping) (*model.LBTcpMonitorProfile, error) {
	monitor, err := s.access.CreateTCPMonitorProfile(s.clusterName, s.objectName, mapping)
	if err == nil {
		s.CtxInfof("created LbTcpMonitor %s for %s", *monitor.Id, mapping)
		s.tcpMonitors = append(s.tcpMonitors, monitor)
	}
	return monitor, err
}

func (s *state) updateTCPMonitor(monitor *model.LBTcpMonitorProfile, mapping Mapping) error {
	if monitor.MonitorPort != nil && *monitor.MonitorPort == int64(mapping.NodePort) {
		return nil
	}
	monitor.MonitorPort = int64ptr(int64(mapping.NodePort))
	s.CtxInfof("updating LbTcpMonitor %s for %s", *monitor.Id, mapping)
	return s.access.UpdateTCPMonitorProfile(monitor)
}

func (s *state) deleteTCPMonitor(monitor *model.LBTcpMonitorProfile) error {
	s.CtxInfof("deleting LbTcpMonitor %s for %s", *monitor.Id, getTag(monitor.Tags, ScopePort))
	return s.access.DeleteTCPMonitorProfile(*monitor.Id)
}

func (s *state) getPool(mapping Mapping, monitor *model.LBTcpMonitorProfile) (*model.LBPool, error) {
	var activeMonitorPaths []string
	if monitor != nil {
		activeMonitorPaths = []string{*monitor.Path}
	}
	for _, pool := range s.pools {
		if mapping.MatchPool(pool) {
			err := s.updatePool(pool, mapping, activeMonitorPaths)
			return pool, err
		}
	}
	return s.createPool(mapping, activeMonitorPaths)
}

func (s *state) createPool(mapping Mapping, activeMonitorIds []string) (*model.LBPool, error) {
	members, _ := s.updatedPoolMembers(nil)
	pool, err := s.access.CreatePool(s.clusterName, s.objectName, mapping, members, activeMonitorIds)
	if err == nil {
		s.CtxInfof("created LbPool %s for %s", *pool.Id, mapping)
		s.pools = append(s.pools, pool)
	}
	return pool, err
}

func (s *state) UpdatePoolMembers() error {
	pools, err := s.access.FindPools(s.clusterName, s.objectName)
	if err != nil {
		return err
	}
	for _, servicePort := range s.service.Spec.Ports {
		mapping := NewMapping(servicePort)
		for _, pool := range pools {
			if mapping.MatchPool(pool) {
				err = s.updatePool(pool, mapping, pool.ActiveMonitorPaths)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *state) updatePool(pool *model.LBPool, mapping Mapping, activeMonitorPaths []string) error {
	newMembers, modified := s.updatedPoolMembers(pool.Members)
	if modified || !reflect.DeepEqual(activeMonitorPaths, pool.ActiveMonitorPaths) {
		pool.Members = newMembers
		pool.ActiveMonitorPaths = activeMonitorPaths
		s.CtxInfof("updating LbPool %s for %s, #members=%d", *pool.Id, mapping, len(pool.Members))
		err := s.access.UpdatePool(pool)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *state) updatedPoolMembers(oldMembers []model.LBPoolMember) ([]model.LBPoolMember, bool) {
	modified := false
	nodeIPAddresses := collectNodeInternalAddresses(s.nodes)
	newMembers := []model.LBPoolMember{}
	for _, member := range oldMembers {
		if member.IpAddress == nil {
			continue
		}
		if _, ok := nodeIPAddresses[*member.IpAddress]; ok {
			newMembers = append(newMembers, member)
		} else {
			modified = true
		}
	}
	if len(nodeIPAddresses) > len(newMembers) {
		for nodeIPAddress, nodeName := range nodeIPAddresses {
			found := false
			for _, member := range oldMembers {
				if member.IpAddress != nil && *member.IpAddress == nodeIPAddress {
					found = true
					break
				}
			}
			if !found {
				member := model.LBPoolMember{
					AdminState:  strptr("ENABLED"),
					DisplayName: strptr(fmt.Sprintf("%s:%s", s.clusterName, nodeName)),
					IpAddress:   strptr(nodeIPAddress),
				}
				newMembers = append(newMembers, member)
				modified = true
			}
		}
	}
	return newMembers, modified
}

func (s *state) deletePool(pool *model.LBPool) error {
	s.CtxInfof("deleting LbPool %s for %s", *pool.Id, getTag(pool.Tags, ScopePort))
	return s.access.DeletePool(*pool.Id)
}

func (s *state) getVirtualServer(mapping Mapping, poolPath *string) (*model.LBVirtualServer, error) {
	for _, server := range s.servers {
		if mapping.MatchVirtualServer(server) {
			err := s.updateVirtualServer(server, mapping, poolPath)
			if err != nil {
				return nil, err
			}
			return server, nil
		}
	}

	return s.createVirtualServer(mapping, poolPath)
}

func (s *state) createVirtualServer(mapping Mapping, poolPath *string) (*model.LBVirtualServer, error) {
	allocated, err := s.allocateResources()
	if err != nil {
		return nil, err
	}

	lbServicePath, err := s.lbService.getOrCreateLoadBalancerService(s.clusterName)
	if err != nil {
		return nil, errors.Wrapf(err, "get or create LBService failed")
	}

	applicationProfilePath, err := s.access.GetAppProfilePath(s.class, mapping.Protocol)
	if err != nil {
		return nil, errors.Wrapf(err, "Lookup of application profile failed for %s", mapping.Protocol)
	}

	server, err := s.access.CreateVirtualServer(s.clusterName, s.objectName, s.class, *s.ipAddress, mapping,
		lbServicePath, applicationProfilePath, poolPath)
	if err != nil {
		if allocated {
			s.loggedReleaseResources()
		}
		return nil, err
	}
	s.CtxInfof("created LBVirtualServer %s for %s", *server.Id, mapping)
	s.servers = append(s.servers, server)
	return server, nil
}

func (s *state) updateVirtualServer(server *model.LBVirtualServer, mapping Mapping, poolPath *string) error {
	applicationProfilePath, err := s.access.GetAppProfilePath(s.class, mapping.Protocol)
	if err != nil {
		return errors.Wrapf(err, "Lookup of application profile failed for %s", mapping.Protocol)
	}
	if !mapping.MatchNodePort(server) || !safeEquals(server.PoolPath, poolPath) || !safeEquals(server.ApplicationProfilePath, &applicationProfilePath) {
		server.ApplicationProfilePath = strptr(applicationProfilePath)
		server.DefaultPoolMemberPorts = []string{formatPort(mapping.NodePort)}
		server.PoolPath = poolPath
		s.CtxInfof("updating LbVirtualServer %s for %s", *server.Id, mapping)
		err = s.access.UpdateVirtualServer(server)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *state) deleteVirtualServer(server *model.LBVirtualServer) error {
	port := "?"
	if len(server.DefaultPoolMemberPorts) > 0 {
		port = server.DefaultPoolMemberPorts[0]
	}
	s.CtxInfof("deleting LbVirtualServer %s for %s->%s", *server.Id, getTag(server.Tags, ScopePort), port)
	err := s.access.DeleteVirtualServer(*server.Id)
	if err != nil {
		return err
	}
	return s.lbService.removeLoadBalancerServiceIfUnused(s.clusterName)
}
