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
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
)

const (
	// LoadBalancerClassAnnotation is the optional class annotation at the service
	LoadBalancerClassAnnotation = "loadbalancer.vmware.io/class"
)

var (
	// AppName is set by the main program to the name of the application
	AppName string
	// Version is set by the main program to the version of the application
	Version string
)

type lbProvider struct {
	*lbService
	classes *loadBalancerClasses
	keyLock *keyLock
}

// ClusterName contains the cluster-name flag injected from main, needed for cleanup
var ClusterName string

var _ LBProvider = &lbProvider{}

// NewLBProvider creates a new LBProvider
func NewLBProvider(cfg *config.LBConfig) (LBProvider, error) {
	if cfg == nil {
		return nil, nil
	}
	if !cfg.IsEnabled() {
		return nil, nil
	}

	broker, err := NewNsxtBroker(&cfg.NSXT)
	if err != nil {
		return nil, err
	}
	access, err := NewNSXTAccess(broker, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating access handler failed")
	}
	classes, err := setupClasses(access, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "creating load balancer classes failed")
	}
	return &lbProvider{
		lbService: newLbService(access, cfg.LoadBalancer.LBServiceID),
		classes:   classes,
		keyLock:   newKeyLock(),
	}, nil
}

func (p *lbProvider) Initialize(clusterName string, client clientset.Interface, stop <-chan struct{}) {
	if clusterName != "" {
		go p.cleanup(clusterName, client.CoreV1().Services(""), stop)
	}
}

// GetLoadBalancer returns the LoadBalancerStatus
// Implementations must treat the *corev1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (p *lbProvider) GetLoadBalancer(_ context.Context, clusterName string, service *corev1.Service) (status *corev1.LoadBalancerStatus, exists bool, err error) {
	servers, err := p.access.FindVirtualServers(clusterName, namespacedNameFromService(service))
	if err != nil {
		return nil, false, err
	}
	if len(servers) == 0 {
		return nil, false, nil
	}
	return newLoadBalancerStatus(servers[0].IpAddress), true, nil
}

func newLoadBalancerStatus(ipAddress *string) *corev1.LoadBalancerStatus {
	status := &corev1.LoadBalancerStatus{
		Ingress: []corev1.LoadBalancerIngress{},
	}
	if ipAddress != nil {
		status.Ingress = append(status.Ingress, corev1.LoadBalancerIngress{IP: *ipAddress})
	}
	return status
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *corev1.Service parameter as read-only and not modify it.
func (p *lbProvider) GetLoadBalancerName(_ context.Context, clusterName string, service *corev1.Service) string {
	return *displayNameObject(clusterName, namespacedNameFromService(service))
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *corev1.Service and *corev1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (p *lbProvider) EnsureLoadBalancer(_ context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	key := namespacedNameFromService(service).String()
	p.keyLock.Lock(key)
	defer p.keyLock.Unlock(key)

	class, err := p.classFromService(service)
	if err != nil {
		return nil, err
	}

	state := newState(p.lbService, clusterName, service, nodes)
	err = state.Process(class)
	status, err2 := state.Finish()
	if err != nil {
		return status, err
	}
	return status, err2
}

func (p *lbProvider) classFromService(service *corev1.Service) (*loadBalancerClass, error) {
	annos := service.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
	}
	name, ok := annos[LoadBalancerClassAnnotation]
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		name = config.DefaultLoadBalancerClass
	}

	class := p.classes.GetClass(name)
	if class == nil {
		return nil, fmt.Errorf("invalid load balancer class %s", name)
	}
	return class, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *corev1.Service and *corev1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (p *lbProvider) UpdateLoadBalancer(_ context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	key := namespacedNameFromService(service).String()
	p.keyLock.Lock(key)
	defer p.keyLock.Unlock(key)

	state := newState(p.lbService, clusterName, service, nodes)

	return state.UpdatePoolMembers()
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *corev1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (p *lbProvider) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	emptyService := service.DeepCopy()
	emptyService.Spec.Ports = nil
	_, err := p.EnsureLoadBalancer(ctx, clusterName, emptyService, nil)
	return err
}
