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

package connectionmanager

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"k8s.io/klog"

	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"

	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

//Well-known keys for k/v maps
const (

	// ZoneLabel is the label for zones.
	ZoneLabel = "Zone"

	// RegionLabel is the label for regions.
	RegionLabel = "Region"
)

// WhichVCandDCByZone gets the corresponding VC+DC combo that supports the availability zone
func (cm *ConnectionManager) WhichVCandDCByZone(ctx context.Context,
	zoneLabel string, regionLabel string, zoneLooking string, regionLooking string) (*ZoneDiscoveryInfo, error) {
	klog.V(4).Infof("WhichVCandDCByZone called with zone: %s and region: %s", zoneLooking, regionLooking)

	// Need at least one VC
	numOfVCs := len(cm.VsphereInstanceMap)
	if numOfVCs == 0 {
		err := ErrMustHaveAtLeastOneVCDC
		klog.Errorf("%v", err)
		return nil, err
	}

	if numOfVCs == 1 {
		klog.Info("Single VC Detected")
		return cm.getDIFromSingleVC(ctx, zoneLabel, regionLabel, zoneLooking, regionLooking)
	}

	klog.Info("Multi VC Detected")
	return cm.getDIFromMultiVCorDC(ctx, zoneLabel, regionLabel, zoneLooking, regionLooking)
}

func (cm *ConnectionManager) getDIFromSingleVC(ctx context.Context,
	zoneLabel string, regionLabel string, zoneLooking string, regionLooking string) (*ZoneDiscoveryInfo, error) {
	klog.V(4).Infof("getDIFromSingleVC called with zone: %s and region: %s", zoneLooking, regionLooking)

	if len(cm.VsphereInstanceMap) != 1 {
		err := ErrUnsupportedConfiguration
		klog.Errorf("%v", err)
		return nil, err
	}

	var vc string

	// Get first vSphere Instance
	var tmpVsi *VSphereInstance
	for vc, tmpVsi = range cm.VsphereInstanceMap {
		break //Grab the first one because there is only one
	}

	var err error
	for i := 0; i < NumConnectionAttempts; i++ {
		err = cm.Connect(ctx, vc)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(RetryAttemptDelaySecs) * time.Second)
	}

	numOfDc, err := vclib.GetNumberOfDatacenters(ctx, tmpVsi.Conn)
	if err != nil {
		klog.Errorf("%v", err)
		return nil, err
	}

	// More than 1 DC in this VC
	if numOfDc > 1 {
		klog.Info("Multi Datacenter configuration detected")
		return cm.getDIFromMultiVCorDC(ctx, zoneLabel, regionLabel, zoneLooking, regionLooking)
	}

	// We are sure this is single VC and DC
	klog.Info("Single vCenter/Datacenter configuration detected")

	datacenterObjs, err := vclib.GetAllDatacenter(ctx, tmpVsi.Conn)
	if err != nil {
		klog.Error("GetAllDatacenter failed. Err:", err)
		return nil, err
	}

	discoveryInfo := &ZoneDiscoveryInfo{
		VcServer:   vc,
		DataCenter: datacenterObjs[0],
	}

	return discoveryInfo, nil
}

func (cm *ConnectionManager) getDIFromMultiVCorDC(ctx context.Context,
	zoneLabel string, regionLabel string, zoneLooking string, regionLooking string) (*ZoneDiscoveryInfo, error) {
	klog.V(4).Infof("getDIFromMultiVCorDC called with zone: %s and region: %s", zoneLooking, regionLooking)

	if len(zoneLabel) == 0 || len(regionLabel) == 0 || len(zoneLooking) == 0 || len(regionLooking) == 0 {
		err := ErrMultiVCRequiresZones
		klog.Errorf("%v", err)
		return nil, err
	}

	type zoneSearch struct {
		vc         string
		datacenter *vclib.Datacenter
		host       *object.HostSystem
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *zoneSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *zoneSearch, QueueSize)

	zoneFound := false
	globalErr = nil

	setGlobalErr := func(err error) {
		globalErrMutex.Lock()
		globalErr = &err
		globalErrMutex.Unlock()
	}

	setZoneFound := func(found bool) {
		mutex.Lock()
		zoneFound = found
		mutex.Unlock()
	}

	getZoneFound := func() bool {
		mutex.Lock()
		found := zoneFound
		mutex.Unlock()
		return found
	}

	go func() {
		for vc, vsi := range cm.VsphereInstanceMap {
			var datacenterObjs []*vclib.Datacenter

			found := getZoneFound()
			if found == true {
				break
			}

			var err error
			for i := 0; i < NumConnectionAttempts; i++ {
				err = cm.Connect(ctx, vc)
				if err == nil {
					break
				}
				time.Sleep(time.Duration(RetryAttemptDelaySecs) * time.Second)
			}

			if err != nil {
				klog.Error("getDIFromMultiVCorDC error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.Cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.Conn)
				if err != nil {
					klog.Error("getDIFromMultiVCorDC error dc:", err)
					setGlobalErr(err)
					continue
				}
			} else {
				datacenters := strings.Split(vsi.Cfg.Datacenters, ",")
				for _, dc := range datacenters {
					dc = strings.TrimSpace(dc)
					if dc == "" {
						continue
					}
					datacenterObj, err := vclib.GetDatacenter(ctx, vsi.Conn, dc)
					if err != nil {
						klog.Error("getDIFromMultiVCorDC error dc:", err)
						setGlobalErr(err)
						continue
					}
					datacenterObjs = append(datacenterObjs, datacenterObj)
				}
			}

			for _, datacenterObj := range datacenterObjs {
				found := getZoneFound()
				if found == true {
					break
				}

				finder := find.NewFinder(datacenterObj.Client(), false)
				finder.SetDatacenter(datacenterObj.Datacenter)

				hostList, err := finder.HostSystemList(ctx, "*/*")
				if err != nil {
					klog.Errorf("HostSystemList failed: %v", err)
					continue
				}

				for _, host := range hostList {
					klog.V(3).Infof("Finding zone in vc=%s and datacenter=%s for host: %s", vc, datacenterObj.Name(), host.Name())
					queueChannel <- &zoneSearch{
						vc:         vc,
						datacenter: datacenterObj,
						host:       host,
					}
				}
			}
		}
		close(queueChannel)
	}()

	var zoneInfo *ZoneDiscoveryInfo
	for i := 0; i < PoolSize; i++ {
		wg.Add(1)
		go func() {
			for res := range queueChannel {

				klog.V(3).Infof("Checking zones for host: %s", res.host.Name())
				result, err := cm.LookupZoneByMoref(ctx, res.datacenter, res.host.Reference(), zoneLabel, regionLabel)
				if err != nil {
					klog.Errorf("Failed to find zone: %s and region: %s for host %s", zoneLabel, regionLabel, res.host.Name())
					continue
				}

				if !strings.EqualFold(result[ZoneLabel], zoneLooking) ||
					!strings.EqualFold(result[RegionLabel], regionLooking) {
					klog.V(4).Infof("Does not match region: %s and zone: %s", result[RegionLabel], result[ZoneLabel])
					continue
				}

				klog.Infof("Found zone: %s and region: %s for host %s", zoneLooking, regionLooking, res.host.Name())
				zoneInfo = &ZoneDiscoveryInfo{
					VcServer:   res.vc,
					DataCenter: res.datacenter,
				}

				setZoneFound(true)
				break
			}
			wg.Done()
		}()
	}
	wg.Wait()
	if zoneFound {
		return zoneInfo, nil
	}
	if globalErr != nil {
		return nil, *globalErr
	}

	klog.V(4).Infof("getDIFromMultiVCorDC: zone: %s and region: %s not found", zoneLabel, regionLabel)
	return nil, vclib.ErrNoZoneRegionFound
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

func removePortFromHost(host string) string {
	result := host
	index := strings.IndexAny(host, ":")
	if index != -1 {
		result = host[:index]
	}
	return result
}

// LookupZoneByMoref searches for a zone using the provided managed object reference.
func (cm *ConnectionManager) LookupZoneByMoref(ctx context.Context, dataCenter *vclib.Datacenter,
	moRef types.ManagedObjectReference, zoneLabel string, regionLabel string) (map[string]string, error) {

	vcServer := removePortFromHost(dataCenter.Client().URL().Host)
	result := make(map[string]string, 0)

	vsi := cm.VsphereInstanceMap[vcServer]
	if vsi == nil {
		err := ErrConnectionNotFound
		klog.Errorf("Unable to find Connection for %s", vcServer)
		return nil, err
	}

	err := withTagsClient(ctx, vsi.Conn, func(c *rest.Client) error {
		client := tags.NewManager(c)

		pc := dataCenter.Client().ServiceContent.PropertyCollector
		// example result: ["Folder", "Datacenter", "Cluster", "Host"]
		objects, err := mo.Ancestors(ctx, dataCenter.Client(), pc, moRef)
		if err != nil {
			klog.Errorf("Ancestors failed for %s with err %v", moRef, err)
			return err
		}

		// search the hierarchy, example order: ["Host", "Cluster", "Datacenter", "Folder"]
		for i := range objects {
			obj := objects[len(objects)-1-i]
			klog.V(4).Infof("Name: %s, Type: %s", obj.Self.Value, obj.Self.Type)
			tags, err := client.ListAttachedTags(ctx, obj)
			if err != nil {
				klog.Errorf("Cannot list attached tags. Err: %v", err)
				return err
			}
			for _, value := range tags {
				tag, err := client.GetTag(ctx, value)
				if err != nil {
					klog.Errorf("Zones Get tag %s: %s", value, err)
					return err
				}
				category, err := client.GetCategory(ctx, tag.CategoryID)
				if err != nil {
					klog.Errorf("Zones Get category %s error", value)
					return err
				}

				found := func() {
					klog.V(2).Infof("Found %s tag (%s) attached to %s", category.Name, tag.Name, moRef)
				}
				switch {
				case category.Name == zoneLabel:
					result[ZoneLabel] = tag.Name
					found()
				case category.Name == regionLabel:
					result[RegionLabel] = tag.Name
					found()
				}

				if result[ZoneLabel] != "" && result[RegionLabel] != "" {
					return nil
				}
			}
		}

		if result[RegionLabel] == "" {
			if regionLabel != "" {
				return fmt.Errorf("vSphere region category %s does not match any tags for mo: %v", regionLabel, moRef)
			}
		}
		if result[ZoneLabel] == "" {
			if zoneLabel != "" {
				return fmt.Errorf("vSphere zone category %s does not match any tags for mo: %v", zoneLabel, moRef)
			}
		}

		return nil
	})
	if err != nil {
		klog.Errorf("Get zone for mo: %s: %s", moRef, err)
		return nil, err
	}
	return result, nil
}
