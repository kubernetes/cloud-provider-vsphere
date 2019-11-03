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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	v1 "k8s.io/api/core/v1"

	"github.com/vmware/go-vmware-nsxt/loadbalancer"
)

func generatePoolName(lbName string, port int) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

func generateVirtualServerName(lbName string, port int) string {
	return fmt.Sprintf("%s-port-%d", lbName, port)
}

func (n *nsxtLB) loadBalancerServiceName() string {
	return fmt.Sprintf("kubernetes-cpi-vsphere-%s", n.clusterID)
}

func (n *nsxtLB) addVirtualServersToLoadBalancer(virtualServerIDs []string) error {
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
