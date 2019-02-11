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
	"sync"

	"k8s.io/klog"
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

	NUM_OF_CONNECTION_ATTEMPTS     int = 3
	RETRY_ATTEMPT_DELAY_IN_SECONDS int = 1
)

func NewConnectionManager(config *vcfg.Config, secretLister v1.SecretLister) *ConnectionManager {
	if secretLister != nil {
		klog.V(2).Info("NewConnectionManager with SecretLister")
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
		klog.V(2).Info("NewConnectionManager generic CO")
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

	klog.V(2).Info("NewConnectionManager creds from config")
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
		klog.Errorf("Cannot connect to vCenter with err: %v", err)
		return err
	}

	klog.V(2).Infof("Invalid credentials. Cannot connect to server %q. "+
		"Fetching credentials from secrets.", vsphereInstance.Conn.Hostname)

	// Get latest credentials from SecretCredentialManager
	credentials, err := cm.credentialManager.GetCredential(vsphereInstance.Conn.Hostname)
	if err != nil {
		klog.Error("Failed to get credentials from Secret Credential Manager with err:", err)
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
			klog.V(3).Infof("vCenter connect %s succeeded.", vcServer)
		} else {
			klog.Errorf("vCenter %s failed. Err: %q", vcServer, err)
			return err
		}
	}
	return nil
}

func (cm *ConnectionManager) VerifyWithContext(ctx context.Context) error {
	for vcServer := range cm.VsphereInstanceMap {
		err := cm.Connect(ctx, vcServer)
		if err == nil {
			klog.V(3).Infof("vCenter connect %s succeeded.", vcServer)
		} else {
			klog.Errorf("vCenter %s failed. Err: %q", vcServer, err)
			return err
		}
	}
	return nil
}
