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

package config

import (
	"testing"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

func TestReadYAMLConfig(t *testing.T) {
	contents := `
loadBalancer:
  ipPoolName: pool1
  size: MEDIUM
  lbServiceId: 4711
  tier1GatewayPath: 1234
  tcpAppProfileName: default-tcp-lb-app-profile
  udpAppProfileName: default-udp-lb-app-profile
  tags:
    tag1: value1
    tag2: value 2

loadBalancerClass:
  public:
    ipPoolName: poolPublic
  private:
    ipPoolName: poolPrivate
    tcpAppProfileName: tcp2
    udpAppProfileName: udp2
`
	config, err := ReadRawConfigYAML([]byte(contents))
	if err != nil {
		t.Error(err)
		return
	}

	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("loadBalancer.ipPoolName", config.LoadBalancer.IPPoolName, "pool1")
	assertEquals("loadBalancer.lbServiceId", config.LoadBalancer.LBServiceID, "4711")
	assertEquals("loadBalancer.tier1GatewayPath", config.LoadBalancer.Tier1GatewayPath, "1234")
	assertEquals("loadBalancer.tcpAppProfileName", config.LoadBalancer.TCPAppProfileName, "default-tcp-lb-app-profile")
	assertEquals("loadBalancer.udpAppProfileName", config.LoadBalancer.UDPAppProfileName, "default-udp-lb-app-profile")
	assertEquals("loadBalancer.size", config.LoadBalancer.Size, "MEDIUM")
	if len(config.LoadBalancerClass) != 2 {
		t.Errorf("expected two LoadBalancerClass subsections, but got %d", len(config.LoadBalancerClass))
	}
	assertEquals("loadBalancerClass.public.ipPoolName", config.LoadBalancerClass["public"].IPPoolName, "poolPublic")
	assertEquals("loadBalancerClass.private.tcpAppProfileName", config.LoadBalancerClass["private"].TCPAppProfileName, "tcp2")
	assertEquals("loadBalancerClass.private.udpAppProfileName", config.LoadBalancerClass["private"].UDPAppProfileName, "udp2")
	if len(config.LoadBalancer.AdditionalTags) != 2 || config.LoadBalancer.AdditionalTags["tag1"] != "value1" || config.LoadBalancer.AdditionalTags["tag2"] != "value 2" {
		t.Errorf("unexpected additionalTags %v", config.LoadBalancer.AdditionalTags)
	}
}

func TestReadYAMLConfigOnVMC(t *testing.T) {
	contents := `
loadBalancer:
  ipPoolId: 123-456
  size: MEDIUM
  tier1GatewayPath: 1234
  tcpAppProfilePath: infra/xxx/tcp1234
  udpAppProfilePath: infra/xxx/udp1234
`
	config, err := ReadRawConfigYAML([]byte(contents))
	if err != nil {
		t.Error(err)
		return
	}
	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("loadBalancer.ipPoolId", config.LoadBalancer.IPPoolID, "123-456")
	assertEquals("loadBalancer.size", config.LoadBalancer.Size, "MEDIUM")
	assertEquals("loadBalancer.tier1GatewayPath", config.LoadBalancer.Tier1GatewayPath, "1234")
	assertEquals("loadBalancer.tcpAppProfilePath", config.LoadBalancer.TCPAppProfilePath, "infra/xxx/tcp1234")
	assertEquals("loadBalancer.udpAppProfilePath", config.LoadBalancer.UDPAppProfilePath, "infra/xxx/udp1234")
}
