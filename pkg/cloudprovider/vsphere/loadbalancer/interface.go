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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

// LBProvider is the interface used call the load balancer functionality
// It extends the cloud controller manager LoadBalancer interface by an
// initialization function
type LBProvider interface {
	cloudprovider.LoadBalancer
	Initialize(clusterName string, client clientset.Interface, stop <-chan struct{})
	CleanupServices(clusterName string, services map[types.NamespacedName]corev1.Service) error
}

// NSXTAccess provides methods for dealing with NSX-T objects
type NSXTAccess interface {
	// CreateLoadBalancerService creates a LbService
	CreateLoadBalancerService(clusterName string) (*model.LBService, error)
	// FindLoadBalancerService finds a LbService by cluster name and LB service id
	FindLoadBalancerService(clusterName string, lbServiceID string) (lbService *model.LBService, err error)
	// UpdateLoadBalancerService updates a LbService
	UpdateLoadBalancerService(lbService *model.LBService) error
	// DeleteLoadBalancerService deletes a LbService by id
	DeleteLoadBalancerService(id string) error

	// CreateVirtualServer creates a virtual server
	CreateVirtualServer(clusterName string, objectName types.NamespacedName, class LBClass, ipAddress string, mapping Mapping,
		lbServicePath, applicationProfilePath string, poolPath *string) (*model.LBVirtualServer, error)
	// FindVirtualServers finds a virtual server by cluster and object name
	FindVirtualServers(clusterName string, objectName types.NamespacedName) ([]*model.LBVirtualServer, error)
	// ListVirtualServers finds all virtual servers for a cluster
	ListVirtualServers(clusterName string) ([]*model.LBVirtualServer, error)
	// UpdateVirtualServer updates a virtual server
	UpdateVirtualServer(server *model.LBVirtualServer) error
	// DeleteVirtualServer deletes a virtual server by id
	DeleteVirtualServer(id string) error

	// CreatePool creates a LbPool
	CreatePool(clusterName string, objectName types.NamespacedName, mapping Mapping, members []model.LBPoolMember,
		activeMonitorPaths []string) (*model.LBPool, error)
	// GetPool gets a LbPool by id
	GetPool(id string) (*model.LBPool, error)
	// FindPool finds a LbPool for a mapping
	FindPool(clusterName string, objectName types.NamespacedName, mapping Mapping) (*model.LBPool, error)
	// FindPools finds a LbPool by cluster and object name
	FindPools(clusterName string, objectName types.NamespacedName) ([]*model.LBPool, error)
	// ListPools lists all LbPool for a cluster
	ListPools(clusterName string) ([]*model.LBPool, error)
	// UpdatePool updates a LbPool
	UpdatePool(*model.LBPool) error
	// DeletePool deletes a LbPool by id
	DeletePool(id string) error

	// FindIPPoolByName finds an IP pool by name
	FindIPPoolByName(poolName string) (string, error)

	// GetAppProfilePath gets the application profile for given loadbalancer class and protocol
	GetAppProfilePath(class LBClass, protocol corev1.Protocol) (string, error)

	// AllocateExternalIPAddress allocates an IP address from the given IP pool
	AllocateExternalIPAddress(ipPoolID string, clusterName string, objectName types.NamespacedName) (allocation *model.IpAddressAllocation, ipAddress *string, err error)
	// ListExternalIPAddresses finds all IP addresses belonging to a clusterName from the given IP pool
	ListExternalIPAddresses(ipPoolID string, clusterName string) ([]*model.IpAddressAllocation, error)
	// FindExternalIPAddressForObject finds an IP address belonging to an object
	FindExternalIPAddressForObject(ipPoolID string, clusterName string, objectName types.NamespacedName) (allocation *model.IpAddressAllocation, ipAddress *string, err error)
	// ReleaseExternalIPAddress releases an allocated IP address
	ReleaseExternalIPAddress(ipPoolID string, id string) error

	// CreateTCPMonitorProfile creates a LBTcpMonitorProfile
	CreateTCPMonitorProfile(clusterName string, objectName types.NamespacedName, mapping Mapping) (*model.LBTcpMonitorProfile, error)
	// FindTCPMonitors finds a LBTcpMonitorProfile by cluster and object name
	FindTCPMonitorProfiles(clusterName string, objectName types.NamespacedName) ([]*model.LBTcpMonitorProfile, error)
	// ListTCPMonitorProfile lists LBTcpMonitorProfile by cluster
	ListTCPMonitorProfiles(clusterName string) ([]*model.LBTcpMonitorProfile, error)
	// UpdateTCPMonitorProfile updates a LBTcpMonitorProfile
	UpdateTCPMonitorProfile(monitor *model.LBTcpMonitorProfile) error
	// DeleteTCPMonitorProfile deletes a LBTcpMonitorProfile by id
	DeleteTCPMonitorProfile(id string) error
}

// Reference references an object either by identifier or name
type Reference struct {
	Identifier string
	Name       string
}

// IsEmpty returns true if neither identifier and name is set.
func (r *Reference) IsEmpty() bool {
	return r.Identifier == "" && r.Name == ""
}

// LBClass is an interface to retrieve settings of load balancer class.
type LBClass interface {
	// Tags retrieves tags of an object
	Tags() []model.Tag
	// AppProfile retrieves application profile either by path (stored in Reference.Identifier) or by name
	AppProfile(protocol corev1.Protocol) (Reference, error)
}
