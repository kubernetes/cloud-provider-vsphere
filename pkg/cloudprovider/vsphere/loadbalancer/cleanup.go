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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	klog "k8s.io/klog/v2"
)

const maxPeriod = 30 * time.Minute

// cleanup is used to cleanup obsolete and potentially forgotten objects
// created by the loadbalancer controller in NSX-T. This should not
// happen, but if users play with finalizers or some error condition
// appears in the controller there might be orphaned objects in the
// infrastructure. This is important for higher level automations like
// cluster fleet managements that manage the infrastructure of a cluster,
// because various elements cannot be deleted if they are still in use,
// after the cluster has been deleted.
// The controller tags all elements it creates with the cluster name and
// its identity (the app name of the controller, or a dedicated name chosen
// by the config file in the tags section). This tagging can then be used
// to identify all elements originally created by this controller. By
// comparing this set with the actually required objects it is possible
// to identify those that are orphaned and safely delete them.
func (p *lbProvider) cleanup(clusterName string, client clientcorev1.ServiceInterface, stop <-chan struct{}) {
	timer := time.NewTimer(1 * time.Second)
	lastErrNext := 0 * time.Second
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
			var next time.Duration
			err := p.doCleanupStep(clusterName, client)
			if err == nil {
				next = maxPeriod
				lastErrNext = 0
			} else {
				klog.Warningf("cleanup failed with %s", err)
				if lastErrNext == 0 {
					lastErrNext = 500 * time.Millisecond
				} else {
					lastErrNext = 5 * lastErrNext / 4
					if lastErrNext > maxPeriod {
						lastErrNext = maxPeriod
					}
				}
				next = lastErrNext
			}
			timer.Reset(next)
		}
	}
}

func (p *lbProvider) doCleanupStep(clusterName string, client clientcorev1.ServiceInterface) error {
	klog.Infof("starting cleanup...")
	list, err := client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	services := map[types.NamespacedName]corev1.Service{}
	for _, item := range list.Items {
		if item.Spec.Type == corev1.ServiceTypeLoadBalancer {
			services[namespacedNameFromService(&item)] = item
		}
	}

	return p.CleanupServices(clusterName, services)
}

func (p *lbProvider) CleanupServices(clusterName string, validServices map[types.NamespacedName]corev1.Service) error {
	ipPoolIds := sets.NewString()
	for _, name := range p.classes.GetClassNames() {
		class := p.classes.GetClass(name)
		ipPoolIds.Insert(class.ipPool.Identifier)
	}

	lbs := map[types.NamespacedName]struct{}{}
	servers, err := p.access.ListVirtualServers(ClusterName)
	if err != nil {
		return err
	}
	for _, server := range servers {
		tag := getTag(server.Tags, ScopeService)
		if tag != "" {
			lbs[parseNamespacedName(tag)] = struct{}{}
		}
		ipPoolID := getTag(server.Tags, ScopeIPPoolID)
		ipPoolIds.Insert(ipPoolID)
	}
	ipPoolIds.Delete("")

	pools, err := p.access.ListPools(clusterName)
	if err != nil {
		return err
	}
	for _, pool := range pools {
		tag := getTag(pool.Tags, ScopeService)
		if tag != "" {
			lbs[parseNamespacedName(tag)] = struct{}{}
		}
	}

	monitors, err := p.access.ListTCPMonitorProfiles(clusterName)
	if err != nil {
		return err
	}
	for _, pool := range monitors {
		tag := getTag(pool.Tags, ScopeService)
		if tag != "" {
			lbs[parseNamespacedName(tag)] = struct{}{}
		}
	}

	for ipPoolID := range ipPoolIds {
		ipAddressAllocs, err := p.access.ListExternalIPAddresses(ipPoolID, clusterName)
		if err != nil {
			return err
		}
		for _, ipAddressAlloc := range ipAddressAllocs {
			tag := getTag(ipAddressAlloc.Tags, ScopeService)
			if tag != "" {
				lbs[parseNamespacedName(tag)] = struct{}{}
			}
		}
	}

	klog.Infof("cleanup: %d existing services, artefacts for %d services", len(validServices), len(lbs))
	for lb := range lbs {
		if svc, ok := validServices[lb]; !ok || svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: lb.Namespace,
					Name:      lb.Name,
				},
			}
			klog.Infof("deleting artefacts for non-existing service %s/%s", lb.Namespace, lb.Name)
			err = p.EnsureLoadBalancerDeleted(context.TODO(), clusterName, service)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
