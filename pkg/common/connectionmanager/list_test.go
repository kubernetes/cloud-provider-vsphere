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

package connectionmanager

import (
	"context"
	"crypto/tls"
	"log"
	"strings"
	"testing"

	_ "github.com/vmware/govmomi/lookup/simulator"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/sts/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// configFromSim starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with an empty tls.Config.
func configFromSim(multiDc bool) (*vcfg.Config, func()) {
	return configFromSimWithTLS(new(tls.Config), true, multiDc)
}

// configFromSimWithTLS starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with a tls.Config. The returned client
// config can be configured to allow/decline insecure connections.
func configFromSimWithTLS(tlsConfig *tls.Config, insecureAllowed bool, multiDc bool) (*vcfg.Config, func()) {
	cfg := &vcfg.Config{}
	model := simulator.VPX()

	if multiDc {
		model.Datacenter = 2
		model.Datastore = 1
		model.Cluster = 1
		model.Host = 0
	}

	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}

	// Adds vAPI, STS, Lookup Service endpoints to vcsim
	model.Service.RegisterEndpoints = true

	model.Service.TLS = tlsConfig
	s := model.Service.NewServer()

	cfg.Global.InsecureFlag = insecureAllowed
	cfg.Global.VCenterIP = s.URL.Hostname()
	cfg.Global.VCenterPort = s.URL.Port()
	cfg.Global.User = s.URL.User.Username()
	cfg.Global.Password, _ = s.URL.User.Password()

	if multiDc {
		cfg.Global.Datacenters = "DC0,DC1"
	} else {
		cfg.Global.Datacenters = vclib.TestDefaultDatacenter
	}
	cfg.VirtualCenter = make(map[string]*vcfg.VirtualCenterConfig)
	cfg.VirtualCenter[cfg.Global.VCenterIP] = &vcfg.VirtualCenterConfig{
		User:             cfg.Global.User,
		Password:         cfg.Global.Password,
		TenantRef:        cfg.Global.VCenterIP,
		VCenterIP:        cfg.Global.VCenterIP,
		VCenterPort:      cfg.Global.VCenterPort,
		InsecureFlag:     cfg.Global.InsecureFlag,
		Datacenters:      cfg.Global.Datacenters,
		IPFamilyPriority: []string{vcfg.DefaultIPFamily},
	}

	// Configure region and zone categories
	cfg.Labels.Region = "k8s-region"
	cfg.Labels.Zone = "k8s-zone"

	return cfg, func() {
		s.Close()
		model.Remove()
	}
}

// configFromEnvOrSim builds a config from configFromSim and overrides using configFromEnv
func configFromEnvOrSim(multiDc bool) (*vcfg.Config, func()) {
	cfg, fin := configFromSim(multiDc)
	if err := cfg.FromEnv(); err != nil {
		return nil, nil
	}
	return cfg, fin
}

func TestListAllVcPairs(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(config, nil, nil)
	defer connMgr.Logout()

	// context
	ctx := context.Background()

	items, err := connMgr.ListAllVCandDCPairs(ctx)
	if err != nil {
		t.Fatalf("ListAllVCandDCPairs err=%v", err)
	}
	if len(items) != 2 {
		t.Fatalf("ListAllVCandDCPairs items should be 2 but count=%d", len(items))
	}

	// item 0
	if !strings.EqualFold(items[0].VcServer, config.Global.VCenterIP) {
		t.Errorf("item[0].VcServer mismatch %s!=%s", items[0].VcServer, config.Global.VCenterIP)
	}
	if !strings.EqualFold(items[0].DataCenter.Name(), "DC0") && !strings.EqualFold(items[0].DataCenter.Name(), "DC1") {
		t.Errorf("item[0].Datacenter.Name() name=%s should either be DC0 or DC1", items[0].DataCenter.Name())
	}

	// item 1
	if !strings.EqualFold(items[1].VcServer, config.Global.VCenterIP) {
		t.Errorf("item[1].VcServer mismatch %s!=%s", items[1].VcServer, config.Global.VCenterIP)
	}
	if !strings.EqualFold(items[1].DataCenter.Name(), "DC0") && !strings.EqualFold(items[1].DataCenter.Name(), "DC1") {
		t.Errorf("item[1].Datacenter.Name() name=%s should either be DC0 or DC1", items[1].DataCenter.Name())
	}
}
