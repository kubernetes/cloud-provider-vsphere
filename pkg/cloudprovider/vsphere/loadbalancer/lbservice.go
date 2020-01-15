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
	"sync"
)

type lbService struct {
	access      NSXTAccess
	lbServiceID string
	managed     bool
	lbLock      sync.Mutex
}

func newLbService(access NSXTAccess, lbServiceID string) *lbService {
	return &lbService{access: access, lbServiceID: lbServiceID, managed: lbServiceID == ""}
}

func (s *lbService) getOrCreateLoadBalancerService(clusterName string) (string, error) {
	s.lbLock.Lock()
	defer s.lbLock.Unlock()

	lbService, err := s.access.FindLoadBalancerService(clusterName, s.lbServiceID)
	if err != nil {
		return "", err
	}
	if lbService != nil {
		return *lbService.Path, nil
	}
	if s.managed {
		lbService, err = s.access.CreateLoadBalancerService(clusterName)
		if err != nil {
			return "", err
		}
		s.lbServiceID = *lbService.Id
		return *lbService.Path, nil
	}
	return "", fmt.Errorf("no load balancer service found with id %s", s.lbServiceID)
}

func (s *lbService) removeLoadBalancerServiceIfUnused(clusterName string) error {
	s.lbLock.Lock()
	defer s.lbLock.Unlock()

	if !s.managed {
		return nil
	}

	lbService, err := s.access.FindLoadBalancerService(clusterName, s.lbServiceID)
	if err != nil {
		return err
	}
	if lbService == nil {
		return nil
	}
	virtualServers, err := s.access.ListVirtualServers(clusterName)
	if err != nil {
		return err
	}
	if len(virtualServers) == 0 {
		err := s.access.DeleteLoadBalancerService(*lbService.Id)
		if err != nil {
			return err
		}
	}
	return nil
}
