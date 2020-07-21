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

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
)

type loadBalancerClasses struct {
	size    string
	classes map[string]*loadBalancerClass
}

type loadBalancerClass struct {
	className     string
	ipPool        Reference
	tcpAppProfile Reference
	udpAppProfile Reference

	tags []model.Tag
}

func setupClasses(access NSXTAccess, cfg *config.LBConfig) (*loadBalancerClasses, error) {
	if !config.LoadBalancerSizes.Has(cfg.LoadBalancer.Size) {
		return nil, fmt.Errorf("invalid load balancer size %s", cfg.LoadBalancer.Size)
	}

	lbClasses := &loadBalancerClasses{
		size:    cfg.LoadBalancer.Size,
		classes: map[string]*loadBalancerClass{},
	}

	resolver := &ipPoolResolver{access: access, knownIPPools: map[string]string{}}
	defaultClass, err := newLBClass(config.DefaultLoadBalancerClass, &cfg.LoadBalancer.LoadBalancerClassConfig, nil, resolver)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid LoadBalancerClass %s", config.DefaultLoadBalancerClass)
	}
	if defCfg, ok := cfg.LoadBalancerClass[defaultClass.className]; ok {
		defaultClass, err = newLBClass(config.DefaultLoadBalancerClass, defCfg, defaultClass, resolver)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid LoadBalancerClass %s", config.DefaultLoadBalancerClass)
		}
	} else {
		lbClasses.add(defaultClass)
	}

	for name, classConfig := range cfg.LoadBalancerClass {
		if _, ok := lbClasses.classes[name]; ok {
			return nil, fmt.Errorf("duplicate LoadBalancerClass %s", name)
		}
		class, err := newLBClass(name, classConfig, defaultClass, resolver)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid LoadBalancerClass %s", name)
		}
		lbClasses.add(class)
	}

	return lbClasses, nil
}

func (c *loadBalancerClasses) GetClassNames() []string {
	names := make([]string, 0, len(c.classes))
	for name := range c.classes {
		names = append(names, name)
	}
	return names
}

func (c *loadBalancerClasses) GetClass(name string) *loadBalancerClass {
	return c.classes[name]
}

func (c *loadBalancerClasses) add(class *loadBalancerClass) {
	c.classes[class.className] = class
}

type ipPoolResolver struct {
	access       NSXTAccess
	knownIPPools map[string]string
}

func (r *ipPoolResolver) resolve(ipPool *Reference) error {
	var err error
	ipPoolID := ipPool.Identifier
	if ipPoolID == "" {
		var ok bool
		ipPoolID, ok = r.knownIPPools[ipPool.Name]
		if !ok {
			ipPoolID, err = r.access.FindIPPoolByName(ipPool.Name)
			if err != nil {
				return err
			}
			r.knownIPPools[ipPool.Name] = ipPoolID
		}
		ipPool.Identifier = ipPoolID
	}
	return nil
}

func newLBClass(name string, classConfig *config.LoadBalancerClassConfig, defaults *loadBalancerClass, resolver *ipPoolResolver) (*loadBalancerClass, error) {
	class := loadBalancerClass{
		className: name,
		ipPool: Reference{
			Identifier: classConfig.IPPoolID,
			Name:       classConfig.IPPoolName,
		},
		tcpAppProfile: Reference{
			Identifier: classConfig.TCPAppProfilePath,
			Name:       classConfig.TCPAppProfileName,
		},
		udpAppProfile: Reference{
			Identifier: classConfig.UDPAppProfilePath,
			Name:       classConfig.UDPAppProfileName,
		},
	}
	if defaults != nil {
		if class.ipPool.IsEmpty() {
			class.ipPool = defaults.ipPool
		}
		if class.tcpAppProfile.IsEmpty() {
			class.tcpAppProfile = defaults.tcpAppProfile
		}
		if class.udpAppProfile.IsEmpty() {
			class.udpAppProfile = defaults.udpAppProfile
		}
	}
	if resolver != nil {
		err := resolver.resolve(&class.ipPool)
		if err != nil {
			return nil, err
		}
	} else if class.ipPool.Identifier == "" {
		return nil, fmt.Errorf("ipPoolResolver needed if IP pool ID not provided")
	}
	class.tags = []model.Tag{
		newTag(ScopeIPPoolID, class.ipPool.Identifier),
		newTag(ScopeLBClass, class.className),
	}

	return &class, nil
}

func (c *loadBalancerClass) Tags() []model.Tag {
	return c.tags
}

func (c *loadBalancerClass) AppProfile(protocol corev1.Protocol) (Reference, error) {
	switch protocol {
	case corev1.ProtocolTCP:
		return c.tcpAppProfile, nil
	case corev1.ProtocolUDP:
		return c.udpAppProfile, nil
	default:
		return Reference{}, fmt.Errorf("unexpected protocol: %s", protocol)
	}
}
