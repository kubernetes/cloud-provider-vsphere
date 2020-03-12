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
	"strings"
	"testing"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

func TestReadConfig(t *testing.T) {
	contents := `
[LoadBalancer]
ipPoolName = pool1
size = MEDIUM
lbServiceId = 4711
tier1GatewayPath = 1234
tcpAppProfileName = default-tcp-lb-app-profile
udpAppProfileName = default-udp-lb-app-profile
tags = {\"tag1\": \"value1\", \"tag2\": \"value 2\"}

[LoadBalancerClass "public"]
ipPoolName = poolPublic

[LoadBalancerClass "private"]
ipPoolName = poolPrivate
tcpAppProfileName = tcp2
udpAppProfileName = udp2

[NSX-T]
user = admin
password = secret
host = nsxt-server
`
	config, err := ReadConfig(strings.NewReader(contents))
	if err != nil {
		t.Error(err)
		return
	}

	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("LoadBalancer.ipPoolName", config.LoadBalancer.IPPoolName, "pool1")
	assertEquals("LoadBalancer.lbServiceId", config.LoadBalancer.LBServiceID, "4711")
	assertEquals("LoadBalancer.tier1GatewayPath", config.LoadBalancer.Tier1GatewayPath, "1234")
	assertEquals("LoadBalancer.tcpAppProfileName", config.LoadBalancer.TCPAppProfileName, "default-tcp-lb-app-profile")
	assertEquals("LoadBalancer.udpAppProfileName", config.LoadBalancer.UDPAppProfileName, "default-udp-lb-app-profile")
	assertEquals("LoadBalancer.size", config.LoadBalancer.Size, "MEDIUM")
	if len(config.LoadBalancerClasses) != 2 {
		t.Errorf("expected two LoadBalancerClass subsections, but got %d", len(config.LoadBalancerClasses))
	}
	assertEquals("LoadBalancerClass.public.ipPoolName", config.LoadBalancerClasses["public"].IPPoolName, "poolPublic")
	assertEquals("LoadBalancerClass.private.tcpAppProfileName", config.LoadBalancerClasses["private"].TCPAppProfileName, "tcp2")
	assertEquals("LoadBalancerClass.private.udpAppProfileName", config.LoadBalancerClasses["private"].UDPAppProfileName, "udp2")
	if len(config.LoadBalancer.AdditionalTags) != 2 || config.LoadBalancer.AdditionalTags["tag1"] != "value1" || config.LoadBalancer.AdditionalTags["tag2"] != "value 2" {
		t.Errorf("unexpected additionalTags %v", config.LoadBalancer.AdditionalTags)
	}
	assertEquals("NSX-T.user", config.NSXT.User, "admin")
	assertEquals("NSX-T.password", config.NSXT.Password, "secret")
	assertEquals("NSX-T.host", config.NSXT.Host, "nsxt-server")
}

func TestReadConfigOnVMC(t *testing.T) {
	contents := `
[LoadBalancer]
ipPoolID = 123-456
size = MEDIUM
tier1GatewayPath = 1234
tcpAppProfilePath = infra/xxx/tcp1234
udpAppProfilePath = infra/xxx/udp1234

[NSX-T]
vmcAccessToken = token123
vmcAuthHost = authHost
host = nsxt-server
insecure-flag = true
`
	config, err := ReadConfig(strings.NewReader(contents))
	if err != nil {
		t.Error(err)
		return
	}
	assertEquals := func(name, left, right string) {
		if left != right {
			t.Errorf("%s %s != %s", name, left, right)
		}
	}
	assertEquals("LoadBalancer.ipPoolID", config.LoadBalancer.IPPoolID, "123-456")
	assertEquals("LoadBalancer.size", config.LoadBalancer.Size, "MEDIUM")
	assertEquals("LoadBalancer.tier1GatewayPath", config.LoadBalancer.Tier1GatewayPath, "1234")
	assertEquals("LoadBalancer.tcpAppProfilePath", config.LoadBalancer.TCPAppProfilePath, "infra/xxx/tcp1234")
	assertEquals("LoadBalancer.udpAppProfilePath", config.LoadBalancer.UDPAppProfilePath, "infra/xxx/udp1234")
	assertEquals("NSX-T.vmcAccessToken", config.NSXT.VMCAccessToken, "token123")
	assertEquals("NSX-T.vmcAuthHost", config.NSXT.VMCAuthHost, "authHost")
	assertEquals("NSX-T.host", config.NSXT.Host, "nsxt-server")
	if !config.NSXT.InsecureFlag {
		t.Errorf("NSX-T.insecure-flag != true")
	}
}
