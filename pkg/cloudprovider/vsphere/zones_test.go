/*
Copyright 2016 The Kubernetes Authors.
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
	"net/url"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vmware/govmomi/property"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"

	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

func TestZones(t *testing.T) {
	// Any context will do
	ctx := context.Background()

	// Create a vcsim instance
	cfg, close := configFromEnvOrSim(false)
	defer close()

	// Configure for SAML token auth
	cfg.Global.User = localhostCert
	cfg.Global.Password = localhostKey

	// Create configuration object
	connMgr := cm.NewConnectionManager(cfg, nil, nil)
	defer connMgr.Logout()

	nm := newNodeManager(connMgr, nil)
	zones := newZones(nm, cfg.Labels.Zone, cfg.Labels.Region)

	// Create vSphere client
	err := connMgr.Connect(ctx, connMgr.VsphereInstanceMap[cfg.Global.VCenterIP])
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vsi := connMgr.VsphereInstanceMap[cfg.Global.VCenterIP]

	// Get a simulator VM
	myvm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := myvm.Name
	UUID := myvm.Config.Uuid
	k8sUUID := ConvertK8sUUIDtoNormal(UUID)

	// Get a simulator DC
	mydc := simulator.Map.Any("Datacenter").(*simulator.Datacenter)

	// Add the node to the NodeManager
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: k8sUUID,
			},
		},
	}

	nm.RegisterNode(node)

	if len(nm.nodeNameMap) != 1 {
		t.Fatalf("Failed: nodeNameMap should be a length of 1")
	}
	if len(nm.nodeUUIDMap) != 1 {
		t.Fatalf("Failed: nodeUUIDMap should be a length of  1")
	}

	// Get vclib DC
	dc, err := vclib.GetDatacenter(ctx, vsi.Conn, mydc.Name)
	if err != nil {
		t.Fatal(err)
	}

	// Lookup vclib VM
	vm, err := dc.GetVMByUUID(ctx, UUID)
	if err != nil {
		t.Fatal(err)
	}

	// Lookup vclib Host
	host, err := vm.HostSystem(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Property Collector instance
	pc := property.DefaultCollector(vsi.Conn.Client)

	// Tag manager instance
	c := rest.NewClient(vsi.Conn.Client)
	user := url.UserPassword(vsi.Conn.Username, vsi.Conn.Password)
	if err := c.Login(ctx, user); err != nil {
		t.Fatalf("Rest login failed. err=%v", err)
	}

	m := tags.NewManager(c)

	// Create a region category
	regionID, err := m.CreateCategory(ctx, &tags.Category{Name: cfg.Labels.Region})
	if err != nil {
		t.Fatal(err)
	}

	// Create a region tag
	regionID, err = m.CreateTag(ctx, &tags.Tag{CategoryID: regionID, Name: "k8s-region-US"})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone category
	zoneID, err := m.CreateCategory(ctx, &tags.Category{Name: cfg.Labels.Zone})
	if err != nil {
		t.Fatal(err)
	}

	// Create a zone tag
	zoneID, err = m.CreateTag(ctx, &tags.Tag{CategoryID: zoneID, Name: "k8s-zone-US-CA1"})
	if err != nil {
		t.Fatal(err)
	}

	// Create a random category
	randomID, err := m.CreateCategory(ctx, &tags.Category{Name: "random-cat"})
	if err != nil {
		t.Fatal(err)
	}

	// Create a random tag
	randomID, err = m.CreateTag(ctx, &tags.Tag{CategoryID: randomID, Name: "random-tag"})
	if err != nil {
		t.Fatal(err)
	}

	// Attach a random tag to VM's host
	if err = m.AttachTag(ctx, randomID, host); err != nil {
		t.Fatal(err)
	}

	// GetZone() tests, covering error and success paths
	tests := []struct {
		name string // name of the test for logging
		fail bool   // expect GetZone() to return error if true
		prep func() // prepare vCenter state for the test
	}{
		{"no tags", true, func() {
			// no prep
		}},
		{"no zone tag", true, func() {
			if err = m.AttachTag(ctx, regionID, host); err != nil {
				t.Fatal(err)
			}
		}},
		{"host tags set", false, func() {
			if err = m.AttachTag(ctx, zoneID, host); err != nil {
				t.Fatal(err)
			}
		}},
		{"host tags removed", true, func() {
			if err = m.DetachTag(ctx, zoneID, host); err != nil {
				t.Fatal(err)
			}
			if err = m.DetachTag(ctx, regionID, host); err != nil {
				t.Fatal(err)
			}
		}},
		{"dc region, cluster zone", false, func() {
			var h mo.HostSystem
			if err = pc.RetrieveOne(ctx, host.Reference(), []string{"parent"}, &h); err != nil {
				t.Fatal(err)
			}
			// Attach region tag to Datacenter
			if err = m.AttachTag(ctx, regionID, dc); err != nil {
				t.Fatal(err)
			}
			// Attach zone tag to Cluster
			if err = m.AttachTag(ctx, zoneID, h.Parent); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, test := range tests {
		test.prep()

		zone, err := zones.GetZoneByProviderID(ctx, UUID)
		if test.fail {
			if err == nil {
				t.Errorf("%s: expected error", test.name)
			} else {
				t.Logf("%s: expected error=%s", test.name, err)
			}
		} else {
			if err != nil {
				t.Errorf("%s: %s", test.name, err)
			}
			t.Logf("zone=%#v", zone)
		}
	}
}
