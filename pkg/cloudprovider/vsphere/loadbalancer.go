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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	"k8s.io/klog"

	nsxt "github.com/vmware/go-vmware-nsxt"
	"github.com/vmware/go-vmware-nsxt/common"
	"github.com/vmware/go-vmware-nsxt/loadbalancer"
	"github.com/vmware/go-vmware-nsxt/manager"
)

// nsxtLB implements cloudprovider.LoadBalancer for vSphere clusters with NSX-T
type nsxtLB struct {
	client *nsxt.APIClient
	// clusterID is a unique identifier of the cluster
	clusterID string
	// routerID is the ID of the NSX-T tier1 router used for creating LoadBalancers
	routerID string
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
		client:    nsxClient,
		routerID:  cfg.Tier1RouterID,
		clusterID: clusterID,
		vipPoolID: cfg.VIPPoolID,

		// only required for raw http requests
		server:   cfg.Server,
		username: cfg.User,
		password: cfg.Password,
		insecure: cfg.Insecure,
	}, nil
}

func (n *nsxtLB) Initialize() error {
	// first look for the Tier1 router by ID provided in config
	router, resp, err := n.client.LogicalRoutingAndServicesApi.ReadLogicalRouter(n.client.Context, n.routerID)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("NSX-T logical router with router ID %q not found", n.routerID)
	}

	if err != nil {
		return err
	}

	// then check for LB Service by cluster ID, if it already exists, we're good to go
	// if it doesn't exist, create one now
	lbServiceName := n.loadBalancerServiceName()
	lbService, exists, err := n.getLBServiceByName(lbServiceName)
	if err != nil {
		return err
	}

	if exists {
		n.lbServiceID = lbService.Id
		return nil
	}

	lbService = loadbalancer.LbService{
		DisplayName: lbServiceName,
		Description: fmt.Sprintf("LoadBalancer Service managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
		Enabled:     true,
		Size:        "SMALL", // TODO: this should be configurable in the config?
		Attachment: &common.ResourceReference{
			TargetType: router.ResourceType,
			TargetId:   router.Id,
		},
	}

	lbService, _, err = n.client.ServicesApi.CreateLoadBalancerService(n.client.Context, lbService)
	if err != nil {
		return err
	}

	n.lbServiceID = lbService.Id
	return nil
}

func (n *nsxtLB) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	virtualServers, err := n.listLoadBalancerVirtualServers()
	if err != nil {
		return nil, false, err
	}

	for _, virtualServer := range virtualServers.Results {
		if virtualServer.DisplayName != lbName {
			continue
		}

		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: virtualServer.IpAddress,
				},
			},
		}, true, nil

	}

	return nil, false, nil
}

func (n *nsxtLB) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	// NSX-T LB name is in the format <service-namespace>-<service-name>-<first-five-chars-service-uuid>.
	// The UUID in the end is the ensure LB names are unique across clusters
	return fmt.Sprintf("%s-%s-%s", service.Namespace, service.Name, service.UID[:5])
}

func (n *nsxtLB) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	virtualServer, exists, err := n.getVirtualServerByName(lbName)
	if err != nil {
		return nil, err
	}

	if exists {
		return &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{
				{
					IP: virtualServer.IpAddress,
				},
			},
		}, nil

	}

	// TODO: do actual IPAM allocation to get virtual server IP
	// For now use cluster IP

	// we can re-use the same LB pool members since we can rely on DefaultPoolMemberPort to indicate the node port
	// as the target port for each VirtualServer
	var lbMembers []loadbalancer.PoolMember
	for _, node := range nodes {
		// TODO: don't always assume InternalIP from node addresses
		ip := getInternalIP(node)
		if ip == "" {
			klog.Warningf("node %s has no addresses assigned", node.Name)
			continue
		}

		member := loadbalancer.PoolMember{
			DisplayName: node.Name,
			Weight:      1,
			IpAddress:   ip,
		}

		lbMembers = append(lbMembers, member)
	}

	// allocate VIP from pool provided in config
	allocation, _, err := n.client.PoolManagementApi.AllocateOrReleaseFromIpPool(n.client.Context, n.vipPoolID, manager.AllocationIpAddress{}, "ALLOCATE")
	if err != nil {
		return nil, err
	}

	vip := allocation.AllocationId

	success := false
	defer func() {
		if success {
			return
		}

		// release VIP from pool if load balancer was not created successfully
		_, _, err := n.client.PoolManagementApi.AllocateOrReleaseFromIpPool(n.client.Context, n.vipPoolID,
			manager.AllocationIpAddress{AllocationId: vip}, "RELEASE")
		if err != nil {
			klog.Errorf("error releasing VIP %s after load balancer failed to provision", vip)
		}

	}()

	var newVirtualServerIDs []string

	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		// create a pool ID using this port's node port
		lbPool := loadbalancer.LbPool{
			//  TODO: make this configurable
			Algorithm:        "ROUND_ROBIN",
			DisplayName:      fmt.Sprintf("%s-port-%d", lbName, port.Port),
			Description:      fmt.Sprintf("LoadBalancer Pool managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			Members:          lbMembers,
			MinActiveMembers: 1,
		}

		lbPool, _, err := n.client.ServicesApi.CreateLoadBalancerPool(n.client.Context, lbPool)
		if err != nil {
			return nil, err
		}

		// virtual server doesn't exist.. update node pools? or should that only happen on update?
		virtualServer := loadbalancer.LbVirtualServer{
			DisplayName:           fmt.Sprintf("%s-port-%d", lbName, port.Port),
			Description:           fmt.Sprintf("LoadBalancer VirtualServer managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			IpProtocol:            string(port.Protocol),
			DefaultPoolMemberPort: strconv.Itoa(int(port.NodePort)),
			IpAddress:             vip,
			Ports:                 []string{strconv.Itoa(int(port.Port))},
			Enabled:               true,
			PoolId:                lbPool.Id,
		}

		virtualServer, _, err = n.client.ServicesApi.CreateLoadBalancerVirtualServer(n.client.Context, virtualServer)
		if err != nil {
			return nil, err
		}

		newVirtualServerIDs = append(newVirtualServerIDs, virtualServer.Id)
	}

	success = true

	err = n.AddVirtualServersToLoadBalancer(newVirtualServerIDs)
	if err != nil {
		return nil, err
	}

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

	// we can re-use the same LB pool members since we can rely on DefaultPoolMemberPort to indicate the node port
	// as the target port for each VirtualServer
	var lbMembers []loadbalancer.PoolMember
	for _, node := range nodes {
		// TODO: don't always assume InternalIP from node addresses
		ip := getInternalIP(node)
		if ip == "" {
			klog.Warningf("node %s has no addresses assigned", node.Name)
			continue
		}

		member := loadbalancer.PoolMember{
			DisplayName: node.Name,
			Weight:      1,
			IpAddress:   ip,
		}

		lbMembers = append(lbMembers, member)
	}

	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		poolName := generatePoolName(lbName, int(port.Port))
		lbPool, exists, err := n.getLBPoolByName(poolName)
		if err != nil {
			return err
		}

		if !exists {
			return fmt.Errorf("error updating LB pool %s because it doesn't exist", poolName)
		}

		lbPoolID := lbPool.Id

		// create a pool ID using this port's node port
		lbPool = loadbalancer.LbPool{
			//  TODO: make this configurable
			Algorithm:        "ROUND_ROBIN",
			DisplayName:      fmt.Sprintf("%s-port-%d", lbName, port.Port),
			Description:      fmt.Sprintf("LoadBalancer Pool managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			Members:          lbMembers,
			MinActiveMembers: 1,
		}

		lbPool, _, err = n.client.ServicesApi.UpdateLoadBalancerPool(n.client.Context, lbPoolID, lbPool)
		if err != nil {
			return err
		}

		virtualServerName := generateVirtualServerName(lbName, int(port.Port))
		virtualServer, exists, err := n.getVirtualServerByName(virtualServerName)
		if err != nil {
			return err
		}

		if !exists {
			return fmt.Errorf("error updating Virtual Server %s because it doesn't exist", virtualServerName)
		}

		virtualServerID := virtualServer.Id

		// virtual server doesn't exist.. update node pools? or should that only happen on update?
		virtualServer = loadbalancer.LbVirtualServer{
			DisplayName:           fmt.Sprintf("%s-port-%d", lbName, port.Port),
			Description:           fmt.Sprintf("LoadBalancer VirtualServer managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
			IpProtocol:            string(port.Protocol),
			DefaultPoolMemberPort: strconv.Itoa(int(port.NodePort)),
			IpAddress:             virtualServer.IpAddress,
			Ports:                 []string{strconv.Itoa(int(port.Port))},
			Enabled:               true,
			PoolId:                lbPool.Id,
		}

		virtualServer, _, err = n.client.ServicesApi.UpdateLoadBalancerVirtualServer(n.client.Context, virtualServerID, virtualServer)
		if err != nil {
			return err
		}
	}

	return nil
}

func (n *nsxtLB) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbName := n.GetLoadBalancerName(ctx, clusterName, service)

	// Create a new virtual server per port in the Service since LB pools only support single ports
	// and each Service Port has a dedicated node port
	for _, port := range service.Spec.Ports {
		poolName := generatePoolName(lbName, int(port.Port))
		lbPool, exists, err := n.getLBPoolByName(poolName)
		if err != nil {
			return err
		}

		if exists {
			_, err := n.client.ServicesApi.DeleteLoadBalancerPool(n.client.Context, lbPool.Id)
			if err != nil {
				return err
			}
		}

		virtualServerName := generateVirtualServerName(lbName, int(port.Port))
		virtualServer, exists, err := n.getVirtualServerByName(virtualServerName)
		if err != nil {
			return err
		}

		if exists {
			_, err := n.client.ServicesApi.DeleteLoadBalancerVirtualServer(n.client.Context, virtualServer.Id, nil)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (n *nsxtLB) loadBalancerServiceName() string {
	return fmt.Sprintf("kubernetes-cpi-vsphere-%s", n.clusterID)
}

func generatePoolName(lbName string, port int) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

func generateVirtualServerName(lbName string, port int) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

func (n *nsxtLB) AddVirtualServersToLoadBalancer(virtualServerIDs []string) error {
	// first read load balancer service
	lbService, _, err := n.client.ServicesApi.ReadLoadBalancerService(n.client.Context, n.lbServiceID)
	if err != nil {
		return err
	}

	newVirtualServerIDs := append(lbService.VirtualServerIds, virtualServerIDs...)
	lbService.VirtualServerIds = newVirtualServerIDs

	_, _, err = n.client.ServicesApi.UpdateLoadBalancerService(n.client.Context, lbService.Id, lbService)
	return err
}

func (n *nsxtLB) getLBServiceByName(name string) (loadbalancer.LbService, bool, error) {
	lbs, err := n.listLoadBalancerServices()
	if err != nil {
		return loadbalancer.LbService{}, false, err
	}

	for _, lbSvc := range lbs.Results {
		if lbSvc.DisplayName != name {
			continue
		}

		return lbSvc, true, nil
	}

	return loadbalancer.LbService{}, false, nil
}

func (n *nsxtLB) getVirtualServerByName(name string) (loadbalancer.LbVirtualServer, bool, error) {
	virtualServers, err := n.listLoadBalancerVirtualServers()
	if err != nil {
		return loadbalancer.LbVirtualServer{}, false, err
	}

	for _, virtualServer := range virtualServers.Results {
		if virtualServer.DisplayName != name {
			continue
		}

		return virtualServer, true, nil
	}

	return loadbalancer.LbVirtualServer{}, false, nil
}

func (n *nsxtLB) getLBPoolByName(name string) (loadbalancer.LbPool, bool, error) {
	lbPools, err := n.listLoadBalancerPool()
	if err != nil {
		return loadbalancer.LbPool{}, false, err
	}

	for _, lbPool := range lbPools.Results {
		if lbPool.DisplayName != name {
			continue
		}

		return lbPool, true, nil
	}

	return loadbalancer.LbPool{}, false, nil
}

func getInternalIP(node *v1.Node) string {
	for _, address := range node.Status.Addresses {
		if address.Type != v1.NodeInternalIP {
			continue
		}

		return address.Address
	}

	return ""
}

// ListLoadBalancerVirtualServers represents the http response from list load balancer virtual server request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerVirtualServers struct {
	ResultCount int                            `json:"result_count"`
	Results     []loadbalancer.LbVirtualServer `json:"results"`
}

// listLoadBalancerVirtualServers makes an http request for listing load balancer virtual servers
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *nsxtLB) listLoadBalancerVirtualServers() (ListLoadBalancerVirtualServers, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/virtual-servers"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ListLoadBalancerVirtualServers{}, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return ListLoadBalancerVirtualServers{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ListLoadBalancerVirtualServers{}, err
	}

	var results ListLoadBalancerVirtualServers
	err = json.Unmarshal(body, &results)
	if err != nil {
		return ListLoadBalancerVirtualServers{}, err
	}

	return results, nil
}

// ListLoadBalancerService represents the http response from list load balancer service request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerService struct {
	ResultCount int                      `json:"result_count"`
	Results     []loadbalancer.LbService `json:"results"`
}

// listLoadBalancers makes an http request for listing load balancer services
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *nsxtLB) listLoadBalancerServices() (ListLoadBalancerService, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/services"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ListLoadBalancerService{}, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return ListLoadBalancerService{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ListLoadBalancerService{}, err
	}

	var results ListLoadBalancerService
	err = json.Unmarshal(body, &results)
	if err != nil {
		return ListLoadBalancerService{}, err
	}

	return results, nil
}

// ListLoadBalancerPool represents the http response from list load balancer pools request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerPool struct {
	ResultCount int                   `json:"result_count"`
	Results     []loadbalancer.LbPool `json:"results"`
}

// listLoadBalancerPool makes an http request for listing load balancer pools
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *nsxtLB) listLoadBalancerPool() (ListLoadBalancerPool, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/pools"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ListLoadBalancerPool{}, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return ListLoadBalancerPool{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ListLoadBalancerPool{}, err
	}

	var results ListLoadBalancerPool
	err = json.Unmarshal(body, &results)
	if err != nil {
		return ListLoadBalancerPool{}, err
	}

	return results, nil
}
