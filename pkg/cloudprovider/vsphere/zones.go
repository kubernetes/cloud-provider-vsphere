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
	"os"

	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/klog"

	k8stypes "k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"

	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
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
	klog.V(4).Info("zones.GetZone() called")

	zone := cloudprovider.Zone{}

	if len(z.region) == 0 || len(z.zone) == 0 {
		return zone, nil
	}

	nodeName, err := os.Hostname()
	if err != nil {
		klog.V(2).Info("Failed to get hostname. Err: ", err)
		return zone, err
	}

	node, ok := z.nodeManager.nodeNameMap[nodeName]
	if !ok {
		klog.V(2).Info("zones.GetZone() NOT FOUND with ", nodeName)
		return zone, ErrVMNotFound
	}

	vmHost, err := node.vm.HostSystem(ctx)
	if err != nil {
		klog.Errorf("Failed to get host system for VM: %q. err: %+v", node.vm.InventoryPath, err)
		return zone, err
	}

	var oHost mo.HostSystem
	err = vmHost.Properties(ctx, vmHost.Reference(), []string{"summary"}, &oHost)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}
	klog.V(4).Infof("Host owning VM is %s", oHost.Summary.Config.Name)

	zoneResult, err := z.nodeManager.connectionManager.LookupZoneByMoref(
		ctx, node.tenantRef, vmHost.Reference(), z.zone, z.region)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}

	zone.FailureDomain = zoneResult[cm.ZoneLabel]
	zone.Region = zoneResult[cm.RegionLabel]

	return zone, nil
}

// GetZone implements Zones.GetZone for In-Tree providers

// GetZoneByNodeName implements Zones.GetZone for Out-Tree providers
func (z *zones) GetZoneByNodeName(ctx context.Context, nodeName k8stypes.NodeName) (cloudprovider.Zone, error) {
	klog.V(4).Info("zones.GetZoneByNodeName() called with ", string(nodeName))

	zone := cloudprovider.Zone{}

	if len(z.region) == 0 || len(z.zone) == 0 {
		return zone, nil
	}

	node, ok := z.nodeManager.nodeNameMap[string(nodeName)]
	if !ok {
		klog.V(2).Info("zones.GetZoneByNodeName() NOT FOUND with ", string(nodeName))
		return zone, ErrVMNotFound
	}
	klog.V(4).Infof("Getting zone/region for VM %s", node.NodeName)

	vmHost, err := node.vm.HostSystem(ctx)
	if err != nil {
		klog.Errorf("Failed to get host system for VM: %q. err: %+v", node.vm.InventoryPath, err)
		return zone, err
	}
	vmRP, err := node.vm.ResourcePool(ctx)
	if err != nil {
		klog.Warningf("Failed to get resource pool for VM: %q. err: %+v", node.vm.InventoryPath, err)
		vmRP = nil
	}

	var oHost mo.HostSystem
	err = vmHost.Properties(ctx, vmHost.Reference(), []string{"summary"}, &oHost)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}
	klog.V(4).Infof("Host owning VM is %s", oHost.Summary.Config.Name)

	// Look down the compute resources
	zoneResult, err := z.nodeManager.connectionManager.LookupZoneByMoref(
		ctx, node.tenantRef, vmHost.Reference(), z.zone, z.region)
	if err == nil {
		zone.FailureDomain = zoneResult[cm.ZoneLabel]
		zone.Region = zoneResult[cm.RegionLabel]
		return zone, nil
	}

	// Look down the resource pools
	if vmRP != nil {
		zoneResult, err := z.nodeManager.connectionManager.LookupZoneByMoref(
			ctx, node.tenantRef, vmRP.Reference(), z.zone, z.region)
		if err == nil {
			zone.FailureDomain = zoneResult[cm.ZoneLabel]
			zone.Region = zoneResult[cm.RegionLabel]
			return zone, nil
		}
	}

	// Look down the folders path
	zoneResult, err = z.nodeManager.connectionManager.LookupZoneByMoref(
		ctx, node.tenantRef, node.vm.Reference(), z.zone, z.region)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}

	zone.FailureDomain = zoneResult[cm.ZoneLabel]
	zone.Region = zoneResult[cm.RegionLabel]

	return zone, nil
}

// GetZoneByProviderID implements Zones.GetZone for Out-Tree providers
func (z *zones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	klog.V(4).Info("zones.GetZoneByProviderID() called with ", providerID)

	zone := cloudprovider.Zone{}

	if len(z.region) == 0 || len(z.zone) == 0 {
		return zone, nil
	}

	uid := GetUUIDFromProviderID(providerID)
	node, ok := z.nodeManager.nodeUUIDMap[uid]
	if !ok {
		klog.V(2).Info("zones.GetZoneByProviderID() NOT FOUND with ", uid)
		return zone, ErrVMNotFound
	}
	klog.V(4).Infof("Getting zone/region for VM %s", node.NodeName)

	vmHost, err := node.vm.HostSystem(ctx)
	if err != nil {
		klog.Errorf("Failed to get host system for VM: %q. err: %+v", node.vm.InventoryPath, err)
		return zone, err
	}
	vmRP, err := node.vm.ResourcePool(ctx)
	if err != nil {
		klog.Warningf("Failed to get resource pool for VM: %q. err: %+v", node.vm.InventoryPath, err)
		vmRP = nil
	}

	var oHost mo.HostSystem
	err = vmHost.Properties(ctx, vmHost.Reference(), []string{"summary"}, &oHost)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}
	klog.V(4).Infof("Host owning VM is %s", oHost.Summary.Config.Name)

	// Look down the compute resources
	zoneResult, err := z.nodeManager.connectionManager.LookupZoneByMoref(
		ctx, node.tenantRef, vmHost.Reference(), z.zone, z.region)
	if err == nil {
		zone.FailureDomain = zoneResult[cm.ZoneLabel]
		zone.Region = zoneResult[cm.RegionLabel]
		return zone, nil
	}

	// Look down the resource pools
	if vmRP != nil {
		zoneResult, err := z.nodeManager.connectionManager.LookupZoneByMoref(
			ctx, node.tenantRef, vmRP.Reference(), z.zone, z.region)
		if err == nil {
			zone.FailureDomain = zoneResult[cm.ZoneLabel]
			zone.Region = zoneResult[cm.RegionLabel]
			return zone, nil
		}
	}

	// Look down the folders path
	zoneResult, err = z.nodeManager.connectionManager.LookupZoneByMoref(
		ctx, node.tenantRef, node.vm.Reference(), z.zone, z.region)
	if err != nil {
		klog.Errorf("Failed to get host system properties. err: %+v", err)
		return zone, err
	}

	zone.FailureDomain = zoneResult[cm.ZoneLabel]
	zone.Region = zoneResult[cm.RegionLabel]

	return zone, nil
}
