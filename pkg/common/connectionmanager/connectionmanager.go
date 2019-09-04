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
	"strings"

	clientset "k8s.io/client-go/kubernetes"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/credentialmanager"
	vclib "k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// NewConnectionManager returns a new ConnectionManager object
// This function also initializes the Default/Global lister for secrets. In other words,
// If a single global secret is used for all VCs, the informMgr param will be used to
// obtain those secrets
func NewConnectionManager(cfg *vcfg.Config, secretLister listerv1.SecretLister, client clientset.Interface) *ConnectionManager {
	connMgr := &ConnectionManager{
		client:             client,
		VsphereInstanceMap: generateInstanceMap(cfg),
		credentialManagers: make(map[string]*cm.CredentialManager),
	}

	if secretLister != nil {
		klog.V(2).Info("Initializing with K8s SecretLister")
		credMgr := cm.NewCredentialManager(cfg.Global.SecretName, cfg.Global.SecretNamespace, "", secretLister)
		connMgr.credentialManagers[vcfg.DefaultCredentialManager] = credMgr

		return connMgr
	}

	if cfg.Global.SecretsDirectory != "" {
		klog.V(2).Info("Initializing for generic CO with secrets")
		credMgr := cm.NewCredentialManager("", "", cfg.Global.SecretsDirectory, secretLister)
		connMgr.credentialManagers[vcfg.DefaultCredentialManager] = credMgr

		return connMgr
	}

	klog.V(2).Info("Initializing generic CO")
	credMgr := cm.NewCredentialManager("", "", "", nil)
	connMgr.credentialManagers[vcfg.DefaultCredentialManager] = credMgr

	return connMgr
}

// generateInstanceMap creates a map of vCenter connection objects that can be
// use to create a connection to a vCenter using vclib package
func generateInstanceMap(cfg *vcfg.Config) map[string]*VSphereInstance {
	vsphereInstanceMap := make(map[string]*VSphereInstance)

	for _, vcConfig := range cfg.VirtualCenter {
		vSphereConn := vclib.VSphereConnection{
			Username:          vcConfig.User,
			Password:          vcConfig.Password,
			Hostname:          vcConfig.VCenterIP,
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
		vsphereInstanceMap[vcConfig.TenantRef] = &vsphereIns
	}

	return vsphereInstanceMap
}

// InitializeCredentialManagers initializes the individual secret listers that are NOT
// handled through the Default/Global lister tied to the default service account.
func (connMgr *ConnectionManager) InitializeCredentialManagers(secretLister listerv1.SecretLister) {
	// For each vsi that has a Secret set createManagersPerTenant
	for _, vInstance := range connMgr.VsphereInstanceMap {
		klog.V(3).Infof("Checking vcServer=%s SecretRef=%s", vInstance.Cfg.VCenterIP, vInstance.Cfg.SecretRef)
		if strings.EqualFold(vInstance.Cfg.SecretRef, vcfg.DefaultCredentialManager) {
			klog.V(3).Infof("Skipping. vCenter %s is configured using global service account/secret.", vInstance.Cfg.VCenterIP)
			continue
		}

		klog.V(3).Infof("Adding credMgr/informMgr for vcServer=%s", vInstance.Cfg.VCenterIP)
		credsMgr := cm.NewCredentialManager(vInstance.Cfg.SecretName, vInstance.Cfg.SecretNamespace, "", secretLister)
		connMgr.credentialManagers[vInstance.Cfg.SecretRef] = credsMgr
	}
}

// Connect connects to vCenter with existing credentials
// If credentials are invalid:
// 		1. It will fetch credentials from credentialManager
//      2. Update the credentials
//		3. Connects again to vCenter with fetched credentials
func (connMgr *ConnectionManager) Connect(ctx context.Context, vcInstance *VSphereInstance) error {
	connMgr.Lock()
	defer connMgr.Unlock()

	err := vcInstance.Conn.Connect(ctx)
	if err == nil {
		return nil
	}

	if !vclib.IsInvalidCredentialsError(err) || connMgr.credentialManagers == nil {
		klog.Errorf("Cannot connect to vCenter with err: %v", err)
		return err
	}

	klog.V(2).Infof("Invalid credentials. Fetching credentials from secrets. vcServer=%s credentialHolder=%s",
		vcInstance.Cfg.VCenterIP, vcInstance.Cfg.SecretRef)

	credMgr := connMgr.credentialManagers[vcInstance.Cfg.SecretRef]
	if credMgr == nil {
		klog.Errorf("Unable to find credential manager for vcServer=%s credentialHolder=%s", vcInstance.Cfg.VCenterIP, vcInstance.Cfg.SecretRef)
		return ErrUnableToFindCredentialManager
	}
	credentials, err := credMgr.GetCredential(vcInstance.Cfg.VCenterIP)
	if err != nil {
		klog.Error("Failed to get credentials from Secret Credential Manager with err:", err)
		return err
	}
	vcInstance.Conn.UpdateCredentials(credentials.User, credentials.Password)
	return vcInstance.Conn.Connect(ctx)
}

// Logout closes existing connections to remote vCenter endpoints.
func (connMgr *ConnectionManager) Logout() {
	for _, vsphereIns := range connMgr.VsphereInstanceMap {
		connMgr.Lock()
		c := vsphereIns.Conn.Client
		connMgr.Unlock()
		if c != nil {
			vsphereIns.Conn.Logout(context.TODO())
		}
	}
}

// Verify validates the configuration by attempting to connect to the
// configured, remote vCenter endpoints.
func (connMgr *ConnectionManager) Verify() error {
	for _, vcInstance := range connMgr.VsphereInstanceMap {
		err := connMgr.Connect(context.Background(), vcInstance)
		if err == nil {
			klog.V(3).Infof("vCenter connect %s succeeded.", vcInstance.Cfg.VCenterIP)
		} else {
			klog.Errorf("vCenter %s failed. Err: %q", vcInstance.Cfg.VCenterIP, err)
			return err
		}
	}
	return nil
}

// VerifyWithContext is the same as Verify but allows a Go Context
// to control the lifecycle of the connection event.
func (connMgr *ConnectionManager) VerifyWithContext(ctx context.Context) error {
	for _, vcInstance := range connMgr.VsphereInstanceMap {
		err := connMgr.Connect(ctx, vcInstance)
		if err == nil {
			klog.V(3).Infof("vCenter connect %s succeeded.", vcInstance.Cfg.VCenterIP)
		} else {
			klog.Errorf("vCenter %s failed. Err: %q", vcInstance.Cfg.VCenterIP, err)
			return err
		}
	}
	return nil
}

// APIVersion returns the version of the vCenter API
func (connMgr *ConnectionManager) APIVersion(vcInstance *VSphereInstance) (string, error) {
	if err := connMgr.Connect(context.Background(), vcInstance); err != nil {
		return "", err
	}

	return vcInstance.Conn.Client.ServiceContent.About.ApiVersion, nil
}
