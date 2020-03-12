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

package config

import (
	"fmt"
	"strings"

	ini "gopkg.in/gcfg.v1"
	"k8s.io/klog"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (ci *ConfigINI) CreateConfig() *Config {
	cfg := &Config{
		VirtualCenter: make(map[string]*VirtualCenterConfig),
	}

	cfg.Global.User = ci.Global.User
	cfg.Global.Password = ci.Global.Password
	cfg.Global.VCenterIP = ci.Global.VCenterIP
	cfg.Global.VCenterPort = ci.Global.VCenterPort
	cfg.Global.InsecureFlag = ci.Global.InsecureFlag
	cfg.Global.Datacenters = ci.Global.Datacenters
	cfg.Global.RoundTripperCount = ci.Global.RoundTripperCount
	cfg.Global.CAFile = ci.Global.CAFile
	cfg.Global.Thumbprint = ci.Global.Thumbprint
	cfg.Global.SecretName = ci.Global.SecretName
	cfg.Global.SecretNamespace = ci.Global.SecretNamespace
	cfg.Global.SecretsDirectory = ci.Global.SecretsDirectory
	cfg.Global.APIDisable = ci.Global.APIDisable
	cfg.Global.APIBinding = ci.Global.APIBinding

	for keyVcConfig, valVcConfig := range ci.VirtualCenter {
		cfg.VirtualCenter[keyVcConfig] = &VirtualCenterConfig{
			User:              valVcConfig.User,
			TenantRef:         valVcConfig.TenantRef,
			VCenterIP:         valVcConfig.VCenterIP,
			VCenterPort:       valVcConfig.VCenterPort,
			InsecureFlag:      valVcConfig.InsecureFlag,
			Datacenters:       valVcConfig.Datacenters,
			RoundTripperCount: valVcConfig.RoundTripperCount,
			CAFile:            valVcConfig.CAFile,
			Thumbprint:        valVcConfig.Thumbprint,
			SecretRef:         valVcConfig.SecretRef,
			SecretName:        valVcConfig.SecretName,
			SecretNamespace:   valVcConfig.SecretNamespace,
			IPFamilyPriority:  valVcConfig.IPFamilyPriority,
		}
	}

	cfg.Labels.Region = ci.Labels.Region
	cfg.Labels.Zone = ci.Labels.Zone

	return cfg
}

// validateIPFamily takes the possible values of IPFamily and initializes the
// slice as determined bby priority
func (vcci *VirtualCenterConfigINI) validateIPFamily() error {
	if len(vcci.IPFamily) == 0 {
		vcci.IPFamily = DefaultIPFamily
	}

	ipFamilies := strings.Split(vcci.IPFamily, ",")
	for i, ipFamily := range ipFamilies {
		ipFamily = strings.TrimSpace(ipFamily)
		if len(ipFamily) == 0 {
			copy(ipFamilies[i:], ipFamilies[i+1:])      // Shift a[i+1:] left one index.
			ipFamilies[len(ipFamilies)-1] = ""          // Erase last element (write zero value).
			ipFamilies = ipFamilies[:len(ipFamilies)-1] // Truncate slice.
			continue
		}
		if !strings.EqualFold(ipFamily, IPv4Family) && !strings.EqualFold(ipFamily, IPv6Family) {
			return ErrInvalidIPFamilyType
		}
	}

	vcci.IPFamilyPriority = ipFamilies
	return nil
}

// isSecretInfoProvided returns true if k8s secret is set or using generic CO secret method.
// If both k8s secret and generic CO both are true, we don't know which to use, so return false.
func (ci *ConfigINI) isSecretInfoProvided() bool {
	return (ci.Global.SecretName != "" && ci.Global.SecretNamespace != "" && ci.Global.SecretsDirectory == "") ||
		(ci.Global.SecretName == "" && ci.Global.SecretNamespace == "" && ci.Global.SecretsDirectory != "")
}

// isSecretInfoProvided returns true if the secret per VC has been configured
func (vcci *VirtualCenterConfigINI) isSecretInfoProvided() bool {
	return vcci.SecretName != "" && vcci.SecretNamespace != ""
}

func (ci *ConfigINI) validateConfig() error {
	//Fix default global values
	if ci.Global.RoundTripperCount == 0 {
		ci.Global.RoundTripperCount = DefaultRoundTripperCount
	}
	if ci.Global.VCenterPort == "" {
		ci.Global.VCenterPort = DefaultVCenterPort
	}
	if ci.Global.APIBinding == "" {
		ci.Global.APIBinding = DefaultAPIBinding
	}
	if ci.Global.IPFamily == "" {
		ci.Global.IPFamily = DefaultIPFamily
	}

	// Create a single instance of VSphereInstance for the Global VCenterIP if the
	// VirtualCenter does not already exist in the map
	if ci.Global.VCenterIP != "" && ci.VirtualCenter[ci.Global.VCenterIP] == nil {
		ci.VirtualCenter[ci.Global.VCenterIP] = &VirtualCenterConfigINI{
			User:              ci.Global.User,
			Password:          ci.Global.Password,
			TenantRef:         ci.Global.VCenterIP,
			VCenterIP:         ci.Global.VCenterIP,
			VCenterPort:       ci.Global.VCenterPort,
			InsecureFlag:      ci.Global.InsecureFlag,
			Datacenters:       ci.Global.Datacenters,
			RoundTripperCount: ci.Global.RoundTripperCount,
			CAFile:            ci.Global.CAFile,
			Thumbprint:        ci.Global.Thumbprint,
			SecretRef:         DefaultCredentialManager,
			SecretName:        ci.Global.SecretName,
			SecretNamespace:   ci.Global.SecretNamespace,
			IPFamily:          ci.Global.IPFamily,
		}
	}

	// Must have at least one vCenter defined
	if len(ci.VirtualCenter) == 0 {
		klog.Error(ErrMissingVCenter)
		return ErrMissingVCenter
	}

	// vsphere.conf is no longer supported in the old format.
	for vcServer, vcConfig := range ci.VirtualCenter {
		klog.V(4).Infof("Initializing vc server %s", vcServer)
		if vcServer == "" {
			klog.Error(ErrInvalidVCenterIP)
			return ErrInvalidVCenterIP
		}

		// If vcConfig.VCenterIP is explicitly set, that means the vcServer
		// above is the TenantRef
		if vcConfig.VCenterIP != "" {
			//vcConfig.VCenterIP is already set
			vcConfig.TenantRef = vcServer
		} else {
			vcConfig.VCenterIP = vcServer
			vcConfig.TenantRef = vcServer
		}

		if !ci.isSecretInfoProvided() && !vcConfig.isSecretInfoProvided() {
			if vcConfig.User == "" {
				vcConfig.User = ci.Global.User
				if vcConfig.User == "" {
					klog.Errorf("vcConfig.User is empty for vc %s!", vcServer)
					return ErrUsernameMissing
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = ci.Global.Password
				if vcConfig.Password == "" {
					klog.Errorf("vcConfig.Password is empty for vc %s!", vcServer)
					return ErrPasswordMissing
				}
			}
		} else if ci.isSecretInfoProvided() && !vcConfig.isSecretInfoProvided() {
			vcConfig.SecretRef = DefaultCredentialManager
		} else if vcConfig.isSecretInfoProvided() {
			vcConfig.SecretRef = vcConfig.SecretNamespace + "/" + vcConfig.SecretName
		}

		if vcConfig.VCenterPort == "" {
			vcConfig.VCenterPort = ci.Global.VCenterPort
		}

		if vcConfig.Datacenters == "" {
			if ci.Global.Datacenters != "" {
				vcConfig.Datacenters = ci.Global.Datacenters
			}
		}
		if vcConfig.RoundTripperCount == 0 {
			vcConfig.RoundTripperCount = ci.Global.RoundTripperCount
		}
		if vcConfig.CAFile == "" {
			vcConfig.CAFile = ci.Global.CAFile
		}
		if vcConfig.Thumbprint == "" {
			vcConfig.Thumbprint = ci.Global.Thumbprint
		}

		if vcConfig.IPFamily == "" {
			vcConfig.IPFamily = ci.Global.IPFamily
		}

		err := vcConfig.validateIPFamily()
		if err != nil {
			klog.Errorf("Invalid vcConfig IPFamily: %s, err=%s", vcConfig.IPFamily, err)
			return err
		}

		insecure := vcConfig.InsecureFlag
		if !insecure {
			vcConfig.InsecureFlag = ci.Global.InsecureFlag
		}
	}

	return nil
}

// ReadRawConfigINI parses vSphere cloud config file and stores it into ConfigINI
func ReadRawConfigINI(byConfig []byte) (*ConfigINI, error) {
	if len(byConfig) == 0 {
		return nil, fmt.Errorf("Invalid INI file")
	}

	strConfig := string(byConfig[:len(byConfig)])
	klog.V(6).Infof("INI RAW: %s", strConfig)

	cfg := &ConfigINI{
		VirtualCenter: make(map[string]*VirtualCenterConfigINI),
	}

	if err := ini.FatalOnly(ini.ReadStringInto(cfg, strConfig)); err != nil {
		return nil, err
	}

	err := cfg.validateConfig()
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// ReadConfigINI parses vSphere cloud config file and stores it into Config
func ReadConfigINI(byConfig []byte) (*Config, error) {
	cfg, err := ReadRawConfigINI(byConfig)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
