/*
Copyright 2019 The Kubernetes Authors.

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

package vsphere

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	"k8s.io/klog"

	nsxt "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/loadbalancer"
	"github.com/vmware/go-vmware-nsxt/manager"
)

// nsxtLB implements cloudprovider.LoadBalancer for vSphere clusters with NSX-T
type nsxtLB struct {
	client *nsxt.APIClient
	// clusterID is a unique identifier of the cluster
	clusterID string
	// lbServiceID is the ID of the NSX-T LoadBalancer Service where virtual servers
	// for Service Type=LoadBalancer are created
	lbServiceID string
	// vipPoolID is the ID of the IP Pool where VIPs will be allocated
	vipPoolID string

	// required raw http requests for listing load balancer resources
	// TODO: remove this once go-vmware-nsxt can support ListLoadBalancer* methods
	server   string
	username string
	password string
	insecure bool
}

func newNSXTLoadBalancer(clusterID string, cfg *vcfg.LoadbalancerNSXT) (cloudprovider.LoadBalancer, error) {
	nsxtCfg := &nsxt.Configuration{
		BasePath:  "/api/v1",
		Host:      cfg.Server,
		Scheme:    "https",
		UserAgent: "kubernetes/cloud-provider-vsphere",
		UserName:  cfg.User,
		Password:  cfg.Password,
		Insecure:  cfg.Insecure,
	}

	nsxClient, err := nsxt.NewAPIClient(nsxtCfg)
	if err != nil {
		return nil, err
	}

	// TODO: validate config values
	return &nsxtLB{
		client:      nsxClient,
		clusterID:   clusterID,
		lbServiceID: cfg.LBServiceID,
		vipPoolID:   cfg.VIPPoolID,

		// only required for raw http requests
		server:   cfg.Server,
		username: cfg.User,
		password: cfg.Password,
		insecure: cfg.Insecure,
	}, nil
}

func (n *nsxtLB) Initialize() error {
	_, resp, err := n.client.ServicesApi.ReadLoadBalancerService(n.client.Context, n.lbServiceID)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("load balancer service with ID %s does not exist", n.lbServiceID)
	}

	if err != nil {
		return fmt.Errorf("error looking for load balancer service with ID %s: %v", n.lbServiceID, err)
	}

	return nil
}

func (n *nsxtLB) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	virtualServers, err := n.getVirtualServers(service)
	if err != nil {
		return nil, false, err
	}

	if len(virtualServers) == 0 {
		return nil, false, nil
	}

	// get unique IPs
	ips := n.getUniqueIPsFromVirtualServers(virtualServers)
	if len(ips) == 0 {
		return nil, false, fmt.Errorf("error getting unique IPs of virtual servers for service %s", service.Name)
	}
	if len(ips) > 1 {
		return nil, false, fmt.Errorf("more than virtual IP was associated with service %s", service.Name)
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: ips[0],
			},
		},
	}, true, nil
}

func (n *nsxtLB) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	// NSX-T LB name is in the format <service-namespace>-<service-name>-<first-five-chars-service-uuid>.
	// The UUID in the end is the ensure LB names are unique across clusters
	return fmt.Sprintf("%s-%s-%s", service.Namespace, service.Name, service.UID[:5])
}

func (n *nsxtLB) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	virtualServers, err := n.getVirtualServers(service)
	if err != nil {
		return nil, err
	}

	var vip string
	releaseAllocatedVIP := true

	ips := n.getUniqueIPsFromVirtualServers(virtualServers)
	if len(ips) == 0 {
		allocation, _, err := n.client.PoolManagementApi.AllocateOrReleaseFromIpPool(n.client.Context, n.vipPoolID,
			manager.AllocationIpAddress{}, "ALLOCATE")
		if err != nil {
			return nil, err
		}

		vip := allocation.AllocationId
		defer func() {
			if !releaseAllocatedVIP {
				return
			}

			// release VIP from pool if load balancer was not created successfully
			_, _, err := n.client.PoolManagementApi.AllocateOrReleaseFromIpPool(n.client.Context, n.vipPoolID,
				manager.AllocationIpAddress{AllocationId: vip}, "RELEASE")
			if err != nil {
				klog.Errorf("error releasing VIP %s after load balancer failed to provision", vip)
			}

		}()
	} else if len(ips) == 1 {
		vip = ips[0]
	} else {
		return nil, fmt.Errorf("got more than 1 VIP for service %s", service.Name)
	}

	lbMembers := n.nodesToLBMembers(nodes)
	lbPool, err := n.createOrUpdateLBPool(lbName, lbMembers)
	if err != nil {
		return nil, err
	}

	var newVirtualServerIDs []string
	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		virtualServerID := ""
		virtualServerExists := false
		for _, virtualServer := range virtualServers {
			if virtualServer.DisplayName != generateVirtualServerName(lbName, port.Port) {
				continue
			}

			virtualServerExists = true
			virtualServerID = virtualServer.Id
			break
		}

		virtualServer := loadbalancer.LbVirtualServer{
			DisplayName:            generateVirtualServerName(lbName, port.Port),
			Description:            fmt.Sprintf("LoadBalancer VirtualServer managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			IpProtocol:             string(port.Protocol),
			DefaultPoolMemberPorts: []string{strconv.Itoa(int(port.NodePort))},
			IpAddress:              vip,
			Ports:                  []string{strconv.Itoa(int(port.Port))},
			Enabled:                true,
			PoolId:                 lbPool.Id,
		}

		if !virtualServerExists {
			virtualServer, _, err = n.client.ServicesApi.CreateLoadBalancerVirtualServer(n.client.Context, virtualServer)
			if err != nil {
				return nil, err
			}
		} else {
			virtualServer, _, err = n.client.ServicesApi.UpdateLoadBalancerVirtualServer(n.client.Context, virtualServerID, virtualServer)
			if err != nil {
				return nil, err
			}

		}

		newVirtualServerIDs = append(newVirtualServerIDs, virtualServer.Id)
	}

	err = n.addVirtualServersToLoadBalancer(newVirtualServerIDs)
	if err != nil {
		return nil, err
	}

	releaseAllocatedVIP = false
	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: vip,
			},
		},
	}, nil
}

func (n *nsxtLB) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	virtualServers, err := n.getVirtualServers(service)
	if err != nil {
		return err
	}

	ips := n.getUniqueIPsFromVirtualServers(virtualServers)
	if len(ips) != 1 {
		return fmt.Errorf("expected exactly 1 VIP for service %s, got %v", service.Name, ips)
	}

	vip := ips[0]

	lbMembers := n.nodesToLBMembers(nodes)
	lbPool, err := n.createOrUpdateLBPool(lbName, lbMembers)
	if err != nil {
		return err
	}

	var newVirtualServerIDs []string
	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		virtualServerID := ""
		virtualServerExists := false
		for _, virtualServer := range virtualServers {
			if virtualServer.DisplayName != generateVirtualServerName(lbName, port.Port) {
				continue
			}

			virtualServerExists = true
			virtualServerID = virtualServer.Id
			break
		}

		virtualServer := loadbalancer.LbVirtualServer{
			DisplayName:            generateVirtualServerName(lbName, port.Port),
			Description:            fmt.Sprintf("LoadBalancer VirtualServer managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			IpProtocol:             string(port.Protocol),
			DefaultPoolMemberPorts: []string{strconv.Itoa(int(port.NodePort))},
			IpAddress:              vip,
			Ports:                  []string{strconv.Itoa(int(port.Port))},
			Enabled:                true,
			PoolId:                 lbPool.Id,
		}

		if !virtualServerExists {
			virtualServer, _, err = n.client.ServicesApi.CreateLoadBalancerVirtualServer(n.client.Context, virtualServer)
			if err != nil {
				return err
			}
		} else {
			virtualServer, _, err = n.client.ServicesApi.UpdateLoadBalancerVirtualServer(n.client.Context, virtualServerID, virtualServer)
			if err != nil {
				return err
			}

		}

		newVirtualServerIDs = append(newVirtualServerIDs, virtualServer.Id)
	}

	return n.addVirtualServersToLoadBalancer(newVirtualServerIDs)
}

func (n *nsxtLB) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	lbPool, exists, err := n.getLBPoolByName(lbName)
	if err != nil {
		return err
	}

	if exists {
		_, err := n.client.ServicesApi.DeleteLoadBalancerPool(n.client.Context, lbPool.Id)
		if err != nil {
			return err
		}
	}

	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		virtualServerName := generateVirtualServerName(lbName, port.Port)
		virtualServer, exists, err := n.getVirtualServerByName(virtualServerName)
		if err != nil {
			return err
		}

		if !exists {
			continue
		}

		_, err = n.client.ServicesApi.DeleteLoadBalancerVirtualServer(n.client.Context, virtualServer.Id, nil)
		if err != nil {
			return err
		}
	}

	return nil
}
