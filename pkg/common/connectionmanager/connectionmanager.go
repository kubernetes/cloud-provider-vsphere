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

	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/credentialmanager"
	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// FindVM is the type that represents the types of searches used to
// discover VMs.
type FindVM int

const (
	// FindVMByUUID finds VMs with the provided UUID.
	FindVMByUUID FindVM = iota // 0
	// FindVMByName finds VMs with the provided name.
	FindVMByName // 1
	// FindVMByIP finds VMs with the provided IP adress.
	FindVMByIP // 2

	// PoolSize is the number of goroutines used in parallel to find a VM.
	PoolSize int = 8

	// QueueSize is the size of the channel buffer used to find objects.
	// Only QueueSize objects may be placed into the queue before blocking.
	QueueSize int = PoolSize * 10

	// NumConnectionAttempts is the number of allowed connection attempts
	// before an error is returned.
	NumConnectionAttempts int = 3

	// RetryAttemptDelaySecs is the number of seconds to wait between
	// connection attempts.
	RetryAttemptDelaySecs int = 1
)

// NewConnectionManager returns a new ConnectionManager object.
func NewConnectionManager(config *vcfg.Config, secretLister v1.SecretLister) *ConnectionManager {
	connM := &ConnectionManager{
		VsphereInstanceMap: generateInstanceMap(config),
		credentialManager: &cm.SecretCredentialManager{
			Cache: &cm.SecretCache{
				VirtualCenter: make(map[string]*cm.Credential),
			},
		},
	}

	if secretLister != nil {
		klog.V(2).Info("NewConnectionManager with K8s SecretLister")
		connM.credentialManager.SecretName = config.Global.SecretName
		connM.credentialManager.SecretNamespace = config.Global.SecretNamespace
		connM.credentialManager.SecretLister = secretLister
		return connM
	}

	if config.Global.SecretsDirectory != "" {
		klog.V(2).Info("NewConnectionManager generic CO with secrets")
		connM.credentialManager.SecretsDirectory = config.Global.SecretsDirectory
		connM.credentialManager.SecretsDirectoryParse = false
		return connM
	}

	klog.V(2).Info("NewConnectionManager generic CO")
	return connM
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

// Connect establishes a connection to the supplied vCenter.
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

// Logout closes existing connections to remote vCenter endpoints.
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

// Verify validates the configuration by attempting to connect to the
// configured, remote vCenter endpoints.
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

// VerifyWithContext is the same as Verify but allows a Go Context
// to control the lifecycle of the connection event.
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

// APIVersion returns the version of the vCenter API
func (cm *ConnectionManager) APIVersion(vcenter string) (string, error) {
	if err := cm.Connect(context.Background(), vcenter); err != nil {
		return "", err
	}

	instance := cm.VsphereInstanceMap[vcenter]
	if instance == nil || instance.Conn.Client == nil {
		return "", ErrConnectionNotFound
	}

	return instance.Conn.Client.ServiceContent.About.ApiVersion, nil
}
