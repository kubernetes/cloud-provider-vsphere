/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"
	"net/url"
	"os"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"

	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
	"k8s.io/kubernetes/pkg/cloudprovider"
)

func newZones(nodeManager *NodeManager, zone string, region string) cloudprovider.Zones {
	return &zones{
		nodeManager: nodeManager,
		zone:        zone,
		region:      region,
	}
}

// GetZone implements Zones.GetZone for In-Tree providers
func (z *zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	glog.V(4).Info("zones.GetZone() called")

	nodeName, err := os.Hostname()
	if err != nil {
		glog.V(2).Info("Failed to get hostname. Err: ", err)
		return cloudprovider.Zone{}, err
	}

	node, ok := z.nodeManager.nodeNameMap[nodeName]
	if !ok {
		glog.V(2).Info("zones.GetZone() NOT FOUND with ", nodeName)
		return cloudprovider.Zone{}, ErrVMNotFound
	}

	return z.getZoneByVM(ctx, node)
}

// GetZone implements Zones.GetZone for In-Tree providers

// GetZoneByNodeName implements Zones.GetZone for Out-Tree providers
func (z *zones) GetZoneByNodeName(ctx context.Context, nodeName k8stypes.NodeName) (cloudprovider.Zone, error) {
	glog.V(4).Info("zones.GetZoneByNodeName() called with ", string(nodeName))

	node, ok := z.nodeManager.nodeNameMap[string(nodeName)]
	if !ok {
		glog.V(2).Info("zones.GetZoneByNodeName() NOT FOUND with ", string(nodeName))
		return cloudprovider.Zone{}, ErrVMNotFound
	}

	return z.getZoneByVM(ctx, node)
}

// GetZoneByProviderID implements Zones.GetZone for Out-Tree providers
func (z *zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	glog.V(4).Info("zones.GetZoneByProviderID() called with ", providerID)

	uid := GetUUIDFromProviderID(providerID)

	node, ok := z.nodeManager.nodeUUIDMap[uid]
	if !ok {
		glog.V(2).Info("zones.GetZoneByProviderID() NOT FOUND with ", uid)
		return cloudprovider.Zone{}, ErrVMNotFound
	}

	return z.getZoneByVM(ctx, node)
}

func withTagsClient(ctx context.Context, connection *vclib.VSphereConnection, f func(c *rest.Client) error) error {
	c := rest.NewClient(connection.Client)
	user := url.UserPassword(connection.Username, connection.Password)
	if err := c.Login(ctx, user); err != nil {
		return err
	}
	defer c.Logout(ctx)
	return f(c)
}

func (z *zones) getZoneByVM(ctx context.Context, node *NodeInfo) (cloudprovider.Zone, error) {
	glog.V(4).Infof("getZoneByVM for %s on VC: %s in DC: %s", node.NodeName, node.vcServer, node.dataCenter.Name())

	zone := cloudprovider.Zone{}

	vmHost, err := node.vm.HostSystem(ctx)
	if err != nil {
		glog.Errorf("Failed to get host system for VM: %q. err: %+v", node.vm.InventoryPath, err)
		return zone, err
	}

	var oHost mo.HostSystem
	err = vmHost.Properties(ctx, vmHost.Reference(), []string{"summary"}, &oHost)
	if err != nil {
		glog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}
	glog.V(4).Infof("Host owning VM is %s", oHost.Summary.Config.Name)

	pc := node.vm.Client().ServiceContent.PropertyCollector
	err = withTagsClient(ctx, z.nodeManager.connectionManager.VsphereInstanceMap[node.vcServer].Conn, func(c *rest.Client) error {
		client := tags.NewManager(c)
		// example result: ["Folder", "Datacenter", "Cluster", "Host"]
		objects, err := mo.Ancestors(ctx, node.vm.Client(), pc, vmHost.Reference())
		if err != nil {
			return err
		}

		// search the hierarchy, example order: ["Host", "Cluster", "Datacenter", "Folder"]
		for i := range objects {
			obj := objects[len(objects)-1-i]
			tags, err := client.ListAttachedTags(ctx, obj)
			if err != nil {
				glog.Errorf("Cannot list attached tags. Get zone for node %s: %s", node.NodeName, err)
				return err
			}
			for _, value := range tags {
				tag, err := client.GetTag(ctx, value)
				if err != nil {
					glog.Errorf("Zones Get tag %s: %s", value, err)
					return err
				}
				category, err := client.GetCategory(ctx, tag.CategoryID)
				if err != nil {
					glog.Errorf("Zones Get category %s error", value)
					return err
				}

				found := func() {
					glog.Infof("Found %q tag (%s) for %s attached to %s", category.Name, tag.Name, node.UUID, obj.Reference())
				}
				switch {
				case category.Name == z.zone:
					zone.FailureDomain = tag.Name
					found()
				case category.Name == z.region:
					zone.Region = tag.Name
					found()
				}

				if zone.FailureDomain != "" && zone.Region != "" {
					return nil
				}
			}
		}

		if zone.Region == "" {
			if z.region != "" {
				return fmt.Errorf("vSphere region category %q does not match any tags for node %s [%s]", z.region, node.NodeName, node.UUID)
			}
		}
		if zone.FailureDomain == "" {
			if z.zone != "" {
				return fmt.Errorf("vSphere zone category %q does not match any tags for node %s [%s]", z.region, node.NodeName, node.UUID)
			}
		}

		return nil
	})
	if err != nil {
		glog.Errorf("Get zone for node %s: %s", node.NodeName, err)
		return cloudprovider.Zone{}, err
	}
	return zone, nil
}
