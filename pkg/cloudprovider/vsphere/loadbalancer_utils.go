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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	"github.com/vmware/go-vmware-nsxt/loadbalancer"
)

func generateVirtualServerName(lbName string, port int32) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

func (n *nsxtLB) addVirtualServersToLoadBalancer(virtualServerIDs []string) error {
	if len(virtualServerIDs) == 0 {
		return nil
	}

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

func (n *nsxtLB) getVirtualServers(service *v1.Service) ([]loadbalancer.LbVirtualServer, error) {
	lbName := n.GetLoadBalancerName(context.TODO(), "", service)

	allVirtualServers, err := n.listLoadBalancerVirtualServers()
	if err != nil {
		return nil, err
	}

	virtualServerNames := sets.NewString()
	for _, port := range service.Spec.Ports {
		virtualServerNames.Insert(generateVirtualServerName(lbName, port.Port))
	}

	virtualServers := []loadbalancer.LbVirtualServer{}
	for _, virtualServer := range allVirtualServers {
		if !virtualServerNames.Has(virtualServer.DisplayName) {
			continue
		}

		virtualServers = append(virtualServers, virtualServer)
	}

	return virtualServers, nil
}

func (n *nsxtLB) getUniqueIPsFromVirtualServers(lbs []loadbalancer.LbVirtualServer) []string {
	ipSet := sets.NewString()
	for _, lb := range lbs {
		if ipSet.Has(lb.IpAddress) {
			continue
		}

		ipSet.Insert(lb.IpAddress)
	}

	return ipSet.List()
}

func (n *nsxtLB) getLBServiceByName(name string) (loadbalancer.LbService, bool, error) {
	lbs, err := n.listLoadBalancerServices()
	if err != nil {
		return loadbalancer.LbService{}, false, err
	}

	for _, lbSvc := range lbs {
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

	for _, virtualServer := range virtualServers {
		if virtualServer.DisplayName != name {
			continue
		}

		return virtualServer, true, nil
	}

	return loadbalancer.LbVirtualServer{}, false, nil
}

func (n *nsxtLB) createOrUpdateLBPool(lbName string, lbMembers []loadbalancer.PoolMember) (loadbalancer.LbPool, error) {
	lbPool, exists, err := n.getLBPoolByName(lbName)
	if err != nil {
		return loadbalancer.LbPool{}, err
	}

	lbPoolID := lbPool.Id
	lbPool = loadbalancer.LbPool{
		//  TODO: LB pool algorithm should be configurable via an annotation on the Service
		Algorithm:        "ROUND_ROBIN",
		DisplayName:      lbName,
		Description:      fmt.Sprintf("LoadBalancer Pool managed by Kubernetes vSphere Cloud Provider (%s)", n.clusterID),
		Members:          lbMembers,
		MinActiveMembers: 1,
	}

	if !exists {
		lbPool, _, err = n.client.ServicesApi.CreateLoadBalancerPool(n.client.Context, lbPool)
		if err != nil {
			return loadbalancer.LbPool{}, err
		}
	} else {
		lbPool, _, err = n.client.ServicesApi.UpdateLoadBalancerPool(n.client.Context, lbPoolID, lbPool)
		if err != nil {
			return loadbalancer.LbPool{}, err
		}
	}

	return lbPool, nil
}

func (n *nsxtLB) getLBPoolByName(name string) (loadbalancer.LbPool, bool, error) {
	lbPools, err := n.listLoadBalancerPool()
	if err != nil {
		return loadbalancer.LbPool{}, false, err
	}

	for _, lbPool := range lbPools {
		if lbPool.DisplayName != name {
			continue
		}

		return lbPool, true, nil
	}

	return loadbalancer.LbPool{}, false, nil
}

func (n *nsxtLB) nodesToLBMembers(nodes []*v1.Node) []loadbalancer.PoolMember {
	var lbMembers []loadbalancer.PoolMember
	for _, node := range nodes {
		// TODO: don't always assume InternalIP from node addresses
		nodeIP := getInternalIP(node)
		if nodeIP == "" {
			klog.Warningf("node %s has no addresses assigned", node.Name)
			continue
		}

		member := loadbalancer.PoolMember{
			DisplayName: node.Name,
			Weight:      1,
			IpAddress:   nodeIP,
		}

		lbMembers = append(lbMembers, member)
	}

	return lbMembers
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
func (n *nsxtLB) listLoadBalancerVirtualServers() ([]loadbalancer.LbVirtualServer, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/virtual-servers"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerVirtualServers
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}

// ListLoadBalancerService represents the http response from list load balancer service request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerService struct {
	ResultCount int                      `json:"result_count"`
	Results     []loadbalancer.LbService `json:"results"`
}

// listLoadBalancers makes an http request for listing load balancer services
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *nsxtLB) listLoadBalancerServices() ([]loadbalancer.LbService, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/services"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerService
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}

// ListLoadBalancerPool represents the http response from list load balancer pools request
// TODO: remove when NSX-T client adds ListLoadBalancer* methods
type ListLoadBalancerPool struct {
	ResultCount int                   `json:"result_count"`
	Results     []loadbalancer.LbPool `json:"results"`
}

// listLoadBalancerPool makes an http request for listing load balancer pools
// TODO: remove this once the go-vmware-nsxt client supports this call
func (n *nsxtLB) listLoadBalancerPool() ([]loadbalancer.LbPool, error) {
	// set default transport to skip verifiy
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + n.server + "/api/v1/loadbalancer/pools"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(n.username, n.password)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var results ListLoadBalancerPool
	err = json.Unmarshal(body, &results)
	if err != nil {
		return nil, err
	}

	return results.Results, nil
}
