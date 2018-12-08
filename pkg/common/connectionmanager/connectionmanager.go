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
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/client-go/listers/core/v1"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/credentialmanager"
	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

type FindVM int

const (
	FindVMByUUID FindVM = iota // 0
	FindVMByName               // 1

	POOL_SIZE  int = 8
	QUEUE_SIZE int = POOL_SIZE * 10

	// Error Messages
	ConnectionNotFoundErrMsg       = "vCenter not found"
	UnsupportedConfigurationErrMsg = "Unsupported configuration: Supports only a single VC/DC"
)

// Error constants
var (
	ErrConnectionNotFound       = errors.New(ConnectionNotFoundErrMsg)
	ErrUnsupportedConfiguration = errors.New(UnsupportedConfigurationErrMsg)
)

func NewConnectionManager(config *vcfg.Config, secretLister v1.SecretLister) *ConnectionManager {
	if secretLister != nil {
		glog.V(2).Info("NewConnectionManager with SecretLister")
		return &ConnectionManager{
			VsphereInstanceMap: generateInstanceMap(config),
			credentialManager: &cm.SecretCredentialManager{
				SecretName:      config.Global.SecretName,
				SecretNamespace: config.Global.SecretNamespace,
				SecretLister:    secretLister,
				Cache: &cm.SecretCache{
					VirtualCenter: make(map[string]*cm.Credential),
				},
			},
		}
	}
	if config.Global.SecretsDirectory != "" {
		glog.V(2).Info("NewConnectionManager generic CO")
		return &ConnectionManager{
			VsphereInstanceMap: generateInstanceMap(config),
			credentialManager: &cm.SecretCredentialManager{
				SecretsDirectory:      config.Global.SecretsDirectory,
				SecretsDirectoryParse: false,
				Cache: &cm.SecretCache{
					VirtualCenter: make(map[string]*cm.Credential),
				},
			},
		}
	}

	glog.V(2).Info("NewConnectionManager creds from config")
	return &ConnectionManager{
		VsphereInstanceMap: generateInstanceMap(config),
		credentialManager: &cm.SecretCredentialManager{
			Cache: &cm.SecretCache{
				VirtualCenter: make(map[string]*cm.Credential),
			},
		},
	}
}

//GenerateInstanceMap creates a map of vCenter connection objects that can be
//use to create a connection to a vCenter using vclib package
func generateInstanceMap(cfg *vcfg.Config) map[string]*VSphereInstance {
	vsphereInstanceMap := make(map[string]*VSphereInstance)

	for vcServer, vcConfig := range cfg.VirtualCenter {
		vSphereConn := vclib.VSphereConnection{
			Username:          vcConfig.User,
			Password:          vcConfig.Password,
			Hostname:          vcServer,
			Insecure:          vcConfig.InsecureFlag,
			RoundTripperCount: vcConfig.RoundTripperCount,
			Port:              vcConfig.VCenterPort,
			CACert:            vcConfig.CAFile,
			Thumbprint:        vcConfig.Thumbprint,
		}
		vsphereIns := VSphereInstance{
			Conn: &vSphereConn,
			Cfg:  vcConfig,
		}
		vsphereInstanceMap[vcServer] = &vsphereIns
	}

	return vsphereInstanceMap
}

var (
	clientLock sync.Mutex
)

func (cm *ConnectionManager) Connect(ctx context.Context, vcenter string) error {
	clientLock.Lock()
	defer clientLock.Unlock()

	vc := cm.VsphereInstanceMap[vcenter]
	if vc == nil {
		return ErrConnectionNotFound
	}

	return cm.ConnectByInstance(ctx, vc)
}

// ConnectByInstance connects to vCenter with existing credentials
// If credentials are invalid:
// 		1. It will fetch credentials from credentialManager
//      2. Update the credentials
//		3. Connects again to vCenter with fetched credentials
func (cm *ConnectionManager) ConnectByInstance(ctx context.Context, vsphereInstance *VSphereInstance) error {
	err := vsphereInstance.Conn.Connect(ctx)
	if err == nil {
		return nil
	}

	if !vclib.IsInvalidCredentialsError(err) || cm.credentialManager == nil {
		glog.Errorf("Cannot connect to vCenter with err: %v", err)
		return err
	}

	glog.V(2).Infof("Invalid credentials. Cannot connect to server %q. "+
		"Fetching credentials from secrets.", vsphereInstance.Conn.Hostname)

	// Get latest credentials from SecretCredentialManager
	credentials, err := cm.credentialManager.GetCredential(vsphereInstance.Conn.Hostname)
	if err != nil {
		glog.Error("Failed to get credentials from Secret Credential Manager with err:", err)
		return err
	}
	vsphereInstance.Conn.UpdateCredentials(credentials.User, credentials.Password)
	return vsphereInstance.Conn.Connect(ctx)
}

func (cm *ConnectionManager) Logout() {
	for _, vsphereIns := range cm.VsphereInstanceMap {
		clientLock.Lock()
		c := vsphereIns.Conn.Client
		clientLock.Unlock()
		if c != nil {
			vsphereIns.Conn.Logout(context.TODO())
		}
	}
}

func (cm *ConnectionManager) Verify() error {
	for vcServer := range cm.VsphereInstanceMap {
		err := cm.Connect(context.Background(), vcServer)
		if err == nil {
			glog.V(3).Infof("vCenter connect %s succeeded.", vcServer)
		} else {
			glog.Errorf("vCenter %s failed. Err: %q", vcServer, err)
			return err
		}
	}
	return nil
}

func (cm *ConnectionManager) VerifyWithContext(ctx context.Context) error {
	for vcServer := range cm.VsphereInstanceMap {
		err := cm.Connect(ctx, vcServer)
		if err == nil {
			glog.V(3).Infof("vCenter connect %s succeeded.", vcServer)
		} else {
			glog.Errorf("vCenter %s failed. Err: %q", vcServer, err)
			return err
		}
	}
	return nil
}

// WhichVCandDCByZone gets the corresponding VC+DC combo that supports the availability zone
// TODO: we currently only support a single VC/DC combinations until we support availability zones
// *** Working Idea ***
// The current thinking is that Datacenters will be tagged with a "zone" value. This function
// will be passed the zone from the k8s cluster then "look up" the tag on datacenter using
// a method similar to what is done in the function WhichVCandDCByNodeId (aka in parallel)
// then return the DiscoveryInfo for that zone. This should handle non-unique
// DatastoreCluster and Datastore names between Datacenters.
// *** Working Idea ***
func (cm *ConnectionManager) WhichVCandDCByZone(ctx context.Context, zone string) (*DiscoveryInfo, error) {
	if len(cm.VsphereInstanceMap) != 1 {
		glog.Error("Only a single vServer is currently supported")
		return nil, ErrUnsupportedConfiguration
	}

	var vc string
	var vsi *VSphereInstance
	for vc, vsi = range cm.VsphereInstanceMap {
		glog.V(2).Infof("vCenter: %s", vc)
	}

	datacenters := strings.Split(vsi.Cfg.Datacenters, ",")
	if len(datacenters) != 1 {
		glog.Error("Only a single Datacenter is currently supported")
		return nil, ErrUnsupportedConfiguration
	}

	var err error
	for i := 0; i < 3; i++ {
		err = cm.Connect(ctx, vc)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		glog.Error("Discovering node error vc:", err)
		return nil, err
	}

	datacenterObj, err := vclib.GetDatacenter(ctx, vsi.Conn, vsi.Cfg.Datacenters)
	if err != nil {
		glog.Error("Discovering node error dc:", err)
		return nil, err
	}

	discoveryInfo := &DiscoveryInfo{
		VcServer:   vc,
		DataCenter: datacenterObj,
	}

	return discoveryInfo, nil
}

func (cm *ConnectionManager) WhichVCandDCByNodeId(ctx context.Context, nodeID string, searchBy FindVM) (*VmDiscoveryInfo, error) {
	if nodeID == "" {
		glog.V(3).Info("WhichVCandDCByNodeId called but nodeID is empty")
		return nil, vclib.ErrNoVMFound
	}
	type vmSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *vmSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *vmSearch, QUEUE_SIZE)

	myNodeID := nodeID
	if searchBy == FindVMByUUID {
		glog.V(3).Info("WhichVCandDCByNodeId by UUID")
		myNodeID = strings.ToLower(nodeID)
	} else {
		glog.V(3).Info("WhichVCandDCByNodeId by Name")
	}
	glog.V(2).Info("WhichVCandDCByNodeId nodeID: ", myNodeID)

	vmFound := false
	globalErr = nil

	setGlobalErr := func(err error) {
		globalErrMutex.Lock()
		globalErr = &err
		globalErrMutex.Unlock()
	}

	setVMFound := func(found bool) {
		mutex.Lock()
		vmFound = found
		mutex.Unlock()
	}

	getVMFound := func() bool {
		mutex.Lock()
		found := vmFound
		mutex.Unlock()
		return found
	}

	go func() {
		var datacenterObjs []*vclib.Datacenter
		for vc, vsi := range cm.VsphereInstanceMap {

			found := getVMFound()
			if found == true {
				break
			}

			var err error
			for i := 0; i < 3; i++ {
				err = cm.Connect(ctx, vc)
				if err == nil {
					break
				}
				err = cm.ConnectByInstance(ctx, vsi)
				if err == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if err != nil {
				glog.Error("WhichVCandDCByNodeId error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.Cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.Conn)
				if err != nil {
					glog.Error("WhichVCandDCByNodeId error dc:", err)
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
						glog.Error("WhichVCandDCByNodeId error dc:", err)
						setGlobalErr(err)
						continue
					}
					datacenterObjs = append(datacenterObjs, datacenterObj)
				}
			}

			for _, datacenterObj := range datacenterObjs {
				found := getVMFound()
				if found == true {
					break
				}

				glog.V(4).Infof("Finding node %s in vc=%s and datacenter=%s", myNodeID, vc, datacenterObj.Name())
				queueChannel <- &vmSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	var vmInfo *VmDiscoveryInfo
	for i := 0; i < POOL_SIZE; i++ {
		go func() {
			for res := range queueChannel {
				var vm *vclib.VirtualMachine
				var err error
				if searchBy == FindVMByUUID {
					vm, err = res.datacenter.GetVMByUUID(ctx, myNodeID)
				} else {
					vm, err = res.datacenter.GetVMByDNSName(ctx, myNodeID)
				}

				if err != nil {
					glog.Errorf("Error while looking for vm=%+v in vc=%s and datacenter=%s: %v",
						vm, res.vc, res.datacenter.Name(), err)
					if err != vclib.ErrNoVMFound {
						setGlobalErr(err)
					} else {
						glog.V(2).Infof("Did not find node %s in vc=%s and datacenter=%s",
							myNodeID, res.vc, res.datacenter.Name())
					}
					continue
				}

				var oVM mo.VirtualMachine
				err = vm.Properties(ctx, vm.Reference(), []string{"config", "summary", "summary.config", "guest.net", "guest"}, &oVM)
				if err != nil {
					glog.Errorf("Error collecting properties for vm=%+v in vc=%s and datacenter=%s: %v",
						vm, res.vc, res.datacenter.Name(), err)
					continue
				}

				glog.V(2).Infof("Found node %s as vm=%+v in vc=%s and datacenter=%s",
					nodeID, vm, res.vc, res.datacenter.Name())
				glog.V(2).Info("Hostname: ", oVM.Guest.HostName, " UUID: ", oVM.Summary.Config.Uuid)

				vmInfo = &VmDiscoveryInfo{DataCenter: res.datacenter, VM: vm, VcServer: res.vc,
					UUID: oVM.Summary.Config.Uuid, NodeName: oVM.Guest.HostName}
				setVMFound(true)
				break
			}
			wg.Done()
		}()
		wg.Add(1)
	}
	wg.Wait()
	if vmFound {
		return vmInfo, nil
	}
	if globalErr != nil {
		return nil, *globalErr
	}

	glog.V(4).Infof("WhichVCandDCByNodeId: %q vm not found", myNodeID)
	return nil, vclib.ErrNoVMFound
}

func (cm *ConnectionManager) WhichVCandDCByFCDId(ctx context.Context, fcdID string) (*FcdDiscoveryInfo, error) {
	if fcdID == "" {
		glog.V(3).Info("WhichVCandDCByFCDId called but fcdID is empty")
		return nil, vclib.ErrNoDiskIDFound
	}
	glog.V(2).Info("WhichVCandDCByFCDId fcdID: ", fcdID)

	type fcdSearch struct {
		vc         string
		datacenter *vclib.Datacenter
	}

	var mutex = &sync.Mutex{}
	var globalErrMutex = &sync.Mutex{}
	var queueChannel chan *fcdSearch
	var wg sync.WaitGroup
	var globalErr *error

	queueChannel = make(chan *fcdSearch, QUEUE_SIZE)

	fcdFound := false
	globalErr = nil

	setGlobalErr := func(err error) {
		globalErrMutex.Lock()
		globalErr = &err
		globalErrMutex.Unlock()
	}

	setFCDFound := func(found bool) {
		mutex.Lock()
		fcdFound = found
		mutex.Unlock()
	}

	getFCDFound := func() bool {
		mutex.Lock()
		found := fcdFound
		mutex.Unlock()
		return found
	}

	go func() {
		var datacenterObjs []*vclib.Datacenter
		for vc, vsi := range cm.VsphereInstanceMap {

			found := getFCDFound()
			if found == true {
				break
			}

			var err error
			for i := 0; i < 3; i++ {
				err = cm.Connect(ctx, vc)
				if err == nil {
					break
				}
				err = cm.ConnectByInstance(ctx, vsi)
				if err == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if err != nil {
				glog.Error("WhichVCandDCByFCDId error vc:", err)
				setGlobalErr(err)
				continue
			}

			if vsi.Cfg.Datacenters == "" {
				datacenterObjs, err = vclib.GetAllDatacenter(ctx, vsi.Conn)
				if err != nil {
					glog.Error("WhichVCandDCByFCDId error dc:", err)
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
						glog.Error("WhichVCandDCByFCDId error dc:", err)
						setGlobalErr(err)
						continue
					}
					datacenterObjs = append(datacenterObjs, datacenterObj)
				}
			}

			for _, datacenterObj := range datacenterObjs {
				found := getFCDFound()
				if found == true {
					break
				}

				glog.V(4).Infof("Finding FCD %s in vc=%s and datacenter=%s", fcdID, vc, datacenterObj.Name())
				queueChannel <- &fcdSearch{
					vc:         vc,
					datacenter: datacenterObj,
				}
			}
		}
		close(queueChannel)
	}()

	var fcdInfo *FcdDiscoveryInfo
	for i := 0; i < POOL_SIZE; i++ {
		go func() {
			for res := range queueChannel {

				fcd, err := res.datacenter.DoesFirstClassDiskExist(ctx, fcdID)
				if err != nil {
					glog.Errorf("Error while looking for FCD=%+v in vc=%s and datacenter=%s: %v",
						fcd, res.vc, res.datacenter.Name(), err)
					if err != vclib.ErrNoDiskIDFound {
						setGlobalErr(err)
					} else {
						glog.V(2).Infof("Did not find FCD %s in vc=%s and datacenter=%s",
							fcdID, res.vc, res.datacenter.Name())
					}
					continue
				}

				glog.V(2).Infof("Found FCD %s as vm=%+v in vc=%s and datacenter=%s",
					fcdID, fcd, res.vc, res.datacenter.Name())

				fcdInfo = &FcdDiscoveryInfo{DataCenter: res.datacenter, FCDInfo: fcd, VcServer: res.vc}
				setFCDFound(true)
				break
			}
			wg.Done()
		}()
		wg.Add(1)
	}
	wg.Wait()
	if fcdFound {
		return fcdInfo, nil
	}
	if globalErr != nil {
		return nil, *globalErr
	}

	glog.V(4).Infof("WhichVCandDCByFCDId: %q FCD not found", fcdID)
	return nil, vclib.ErrNoDiskIDFound
}
