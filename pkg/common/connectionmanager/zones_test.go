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
	"net/url"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"

	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

func TestWhichVCandDCByZoneSingleDC(t *testing.T) {
	config, cleanup := configFromEnvOrSim(false)
	defer cleanup()

	connMgr := NewConnectionManager(config, nil, nil)
	defer connMgr.Logout()

	// context
	ctx := context.Background()

	zoneInfo, err := connMgr.WhichVCandDCByZone(ctx, config.Labels.Zone, config.Labels.Region, "", "")
	if err != nil {
		t.Fatalf("WhichVCandDCByZone failed err=%v", err)
	}
	if zoneInfo == nil {
		t.Fatalf("WhichVCandDCByZone zoneInfo=nil")
	}

	if !strings.EqualFold("DC0", zoneInfo.DataCenter.Name()) {
		t.Errorf("Datacenter mismatch DC0 != %s", zoneInfo.DataCenter.Name())
	}
}

func TestWhichVCandDCByZoneMultiDC(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(config, nil, nil)
	defer connMgr.Logout()

	// context
	ctx := context.Background()

	// Get the vSphere Instance
	vsi := connMgr.VsphereInstanceMap[config.Global.VCenterIP]

	err := connMgr.Connect(ctx, vsi)
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	// Tag manager instance
	restClient := rest.NewClient(vsi.Conn.Client)
	user := url.UserPassword(vsi.Conn.Username, vsi.Conn.Password)
	if err := restClient.Login(ctx, user); err != nil {
		t.Fatalf("Rest login failed. err=%v", err)
	}

	m := tags.NewManager(restClient)

	/*
	 * START SETUP
	 */
	// Create a region category
	regionID, err := m.CreateCategory(ctx, &tags.Category{Name: config.Labels.Region})
	if err != nil {
		t.Fatal(err)
	}

	// Create a region tag
	regionID, err = m.CreateTag(ctx, &tags.Tag{CategoryID: regionID, Name: "k8s-region-US"})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone category
	zoneID, err := m.CreateCategory(ctx, &tags.Category{Name: config.Labels.Zone})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone tags
	zoneIDwest, err := m.CreateTag(ctx, &tags.Tag{CategoryID: zoneID, Name: "k8s-zone-US-west"})
	if err != nil {
		t.Fatal(err)
	}
	zoneIDeast, err := m.CreateTag(ctx, &tags.Tag{CategoryID: zoneID, Name: "k8s-zone-US-east"})
	if err != nil {
		t.Fatal(err)
	}

	// Setup a multi-DC environment with zones!
	// Setup DC0
	dc0, err := vclib.GetDatacenter(ctx, vsi.Conn, "DC0")
	if err != nil {
		t.Fatal(err)
	}

	// Attach tag to DC0
	if err = m.AttachTag(ctx, regionID, dc0); err != nil {
		t.Fatal(err)
	}
	if err = m.AttachTag(ctx, zoneIDwest, dc0); err != nil {
		t.Fatal(err)
	}

	// Setup DC1
	dc1, err := vclib.GetDatacenter(ctx, vsi.Conn, "DC1")
	if err != nil {
		t.Fatal(err)
	}

	// Attach tag to DC1
	if err = m.AttachTag(ctx, regionID, dc1); err != nil {
		t.Fatal(err)
	}
	if err = m.AttachTag(ctx, zoneIDeast, dc1); err != nil {
		t.Fatal(err)
	}
	/*
	 * END SETUP
	 */

	// Lookup DC by Zone
	lookupRegion := "k8s-region-US"
	lookupZone := "k8s-zone-US-east"

	zoneInfo, err := connMgr.WhichVCandDCByZone(ctx, config.Labels.Zone, config.Labels.Region, lookupZone, lookupRegion)
	if err != nil {
		t.Fatalf("WhichVCandDCByZone failed err=%v", err)
	}
	if zoneInfo == nil {
		t.Fatalf("WhichVCandDCByZone zoneInfo=nil")
	}

	if !strings.EqualFold("DC1", zoneInfo.DataCenter.Name()) {
		t.Errorf("Datacenter mismatch DC1 != %s", zoneInfo.DataCenter.Name())
	}
}

func TestLookupZoneByMoref(t *testing.T) {
	config, cleanup := configFromEnvOrSim(false)
	defer cleanup()

	connMgr := NewConnectionManager(config, nil, nil)
	defer connMgr.Logout()

	// context
	ctx := context.Background()

	// Get the vSphere Instance
	vsi := connMgr.VsphereInstanceMap[config.Global.VCenterIP]

	err := connMgr.Connect(ctx, vsi)
	if err != nil {
		t.Errorf("Failed to Connect to vSphere: %s", err)
	}

	// Tag manager instance
	restClient := rest.NewClient(vsi.Conn.Client)
	user := url.UserPassword(vsi.Conn.Username, vsi.Conn.Password)
	if err := restClient.Login(ctx, user); err != nil {
		t.Fatalf("Rest login failed. err=%v", err)
	}

	m := tags.NewManager(restClient)

	/*
	 * START SETUP
	 */
	// Get a simulator Host
	myHost := simulator.Map.Any("HostSystem").(*simulator.HostSystem)

	// Create a region category
	regionID, err := m.CreateCategory(ctx, &tags.Category{Name: config.Labels.Region})
	if err != nil {
		t.Fatal(err)
	}

	// Create a region tag
	regionID, err = m.CreateTag(ctx, &tags.Tag{CategoryID: regionID, Name: "k8s-region-US"})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone category
	zoneID, err := m.CreateCategory(ctx, &tags.Category{Name: config.Labels.Zone})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone tags
	zoneIDwest, err := m.CreateTag(ctx, &tags.Tag{CategoryID: zoneID, Name: "k8s-zone-US-west"})
	if err != nil {
		t.Fatal(err)
	}
	zoneIDeast, err := m.CreateTag(ctx, &tags.Tag{CategoryID: zoneID, Name: "k8s-zone-US-east"})
	if err != nil {
		t.Fatal(err)
	}

	// Setup a single-DC environment with zones!
	// Setup DC0
	dc0, err := vclib.GetDatacenter(ctx, vsi.Conn, "DC0")
	if err != nil {
		t.Fatal(err)
	}

	// Attach tag to DC0
	if err = m.AttachTag(ctx, regionID, dc0); err != nil {
		t.Fatal(err)
	}
	if err = m.AttachTag(ctx, zoneIDwest, dc0); err != nil {
		t.Fatal(err)
	}

	// Attach tag to HostSystem
	if err = m.AttachTag(ctx, regionID, myHost); err != nil {
		t.Fatal(err)
	}
	if err = m.AttachTag(ctx, zoneIDeast, myHost); err != nil {
		t.Fatal(err)
	}
	/*
	 * END SETUP
	 */

	// Get Host zone found directly on the Host object. Overrides Datacenter. Ancessor enabled but won't use.
	kv, err := connMgr.LookupZoneByMoref(ctx, config.Global.VCenterIP, myHost.Reference(), config.Labels.Zone, config.Labels.Region)
	if err != nil {
		t.Fatalf("[HOST] LookupZoneByMoref failed err=%v", err)
	}

	region := kv[RegionLabel]
	zone := kv[ZoneLabel]
	if !strings.EqualFold("k8s-region-US", region) {
		t.Errorf("Region value mismatch k8s-region-US != %s", region)
	}
	if !strings.EqualFold("k8s-zone-US-east", zone) {
		t.Errorf("Region value mismatch k8s-zone-US-east != %s", zone)
	}
}
