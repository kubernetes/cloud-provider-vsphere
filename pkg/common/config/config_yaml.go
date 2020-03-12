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

	yaml "gopkg.in/yaml.v2"
	"k8s.io/klog"
)

/*
	TODO:
	When the INI based cloud-config is deprecated, this file should be merged into config.go
	and this file should be deleted.
*/

// CreateConfig generates a common Config object based on what other structs and funcs
// are already dependent upon in other packages.
func (cy *ConfigYAML) CreateConfig() *Config {
	cfg := &Config{
		VirtualCenter: make(map[string]*VirtualCenterConfig),
	}

	cfg.Global.User = cy.Global.User
	cfg.Global.Password = cy.Global.Password
	cfg.Global.VCenterIP = cy.Global.VCenterIP
	cfg.Global.VCenterPort = cy.Global.VCenterPort
	cfg.Global.InsecureFlag = cy.Global.InsecureFlag
	cfg.Global.Datacenters = strings.Join(cy.Global.Datacenters, ",")
	cfg.Global.RoundTripperCount = cy.Global.RoundTripperCount
	cfg.Global.CAFile = cy.Global.CAFile
	cfg.Global.Thumbprint = cy.Global.Thumbprint
	cfg.Global.SecretName = cy.Global.SecretName
	cfg.Global.SecretNamespace = cy.Global.SecretNamespace
	cfg.Global.SecretsDirectory = cy.Global.SecretsDirectory
	cfg.Global.APIDisable = cy.Global.APIDisable
	cfg.Global.APIBinding = cy.Global.APIBinding

	for keyVcConfig, valVcConfig := range cy.VirtualCenter {
		cfg.VirtualCenter[keyVcConfig] = &VirtualCenterConfig{
			User:              valVcConfig.User,
			TenantRef:         valVcConfig.TenantRef,
			VCenterIP:         valVcConfig.VCenterIP,
			VCenterPort:       valVcConfig.VCenterPort,
			InsecureFlag:      valVcConfig.InsecureFlag,
			Datacenters:       strings.Join(valVcConfig.Datacenters, ","),
			RoundTripperCount: valVcConfig.RoundTripperCount,
			CAFile:            valVcConfig.CAFile,
			Thumbprint:        valVcConfig.Thumbprint,
			SecretRef:         valVcConfig.SecretRef,
			SecretName:        valVcConfig.SecretName,
			SecretNamespace:   valVcConfig.SecretNamespace,
			IPFamilyPriority:  valVcConfig.IPFamilyPriority,
		}
	}

	cfg.Labels.Region = cy.Labels.Region
	cfg.Labels.Zone = cy.Labels.Zone

	return cfg
}

// isSecretInfoProvided returns true if k8s secret is set or using generic CO secret method.
// If both k8s secret and generic CO both are true, we don't know which to use, so return false.
func (cy *ConfigYAML) isSecretInfoProvided() bool {
	return (cy.Global.SecretName != "" && cy.Global.SecretNamespace != "" && cy.Global.SecretsDirectory == "") ||
		(cy.Global.SecretName == "" && cy.Global.SecretNamespace == "" && cy.Global.SecretsDirectory != "")
}

// isSecretInfoProvided returns true if the secret per VC has been configured
func (vccy *VirtualCenterConfigYAML) isSecretInfoProvided() bool {
	return vccy.SecretName != "" && vccy.SecretNamespace != ""
}

func (cy *ConfigYAML) validateConfig() error {
	//Fix default global values
	if cy.Global.RoundTripperCount == 0 {
		cy.Global.RoundTripperCount = DefaultRoundTripperCount
	}
	if cy.Global.VCenterPort == "" {
		cy.Global.VCenterPort = DefaultVCenterPort
	}
	if cy.Global.APIBinding == "" {
		cy.Global.APIBinding = DefaultAPIBinding
	}
	if len(cy.Global.IPFamilyPriority) == 0 {
		cy.Global.IPFamilyPriority = []string{DefaultIPFamily}
	}

	// Create a single instance of VSphereInstance for the Global VCenterIP if the
	// VirtualCenter does not already exist in the map
	if cy.Global.VCenterIP != "" && cy.VirtualCenter[cy.Global.VCenterIP] == nil {
		cy.VirtualCenter[cy.Global.VCenterIP] = &VirtualCenterConfigYAML{
			User:              cy.Global.User,
			Password:          cy.Global.Password,
			TenantRef:         cy.Global.VCenterIP,
			VCenterIP:         cy.Global.VCenterIP,
			VCenterPort:       cy.Global.VCenterPort,
			InsecureFlag:      cy.Global.InsecureFlag,
			Datacenters:       cy.Global.Datacenters,
			RoundTripperCount: cy.Global.RoundTripperCount,
			CAFile:            cy.Global.CAFile,
			Thumbprint:        cy.Global.Thumbprint,
			SecretRef:         DefaultCredentialManager,
			SecretName:        cy.Global.SecretName,
			SecretNamespace:   cy.Global.SecretNamespace,
			IPFamilyPriority:  cy.Global.IPFamilyPriority,
		}
	}

	// Must have at least one vCenter defined
	if len(cy.VirtualCenter) == 0 {
		klog.Error(ErrMissingVCenter)
		return ErrMissingVCenter
	}

	// vsphere.conf is no longer supported in the old format.
	for tenantRef, vcConfig := range cy.VirtualCenter {
		klog.V(4).Infof("Initializing vc server %s", tenantRef)
		if vcConfig.VCenterIP == "" {
			klog.Error(ErrInvalidVCenterIP)
			return ErrInvalidVCenterIP
		}

		// in the YAML-based config, the tenant ref is required in the config
		vcConfig.TenantRef = tenantRef

		if !cy.isSecretInfoProvided() && !vcConfig.isSecretInfoProvided() {
			if vcConfig.User == "" {
				vcConfig.User = cy.Global.User
				if vcConfig.User == "" {
					klog.Errorf("vcConfig.User is empty for vc %s!", tenantRef)
					return ErrUsernameMissing
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cy.Global.Password
				if vcConfig.Password == "" {
					klog.Errorf("vcConfig.Password is empty for vc %s!", tenantRef)
					return ErrPasswordMissing
				}
			}
		} else if cy.isSecretInfoProvided() && !vcConfig.isSecretInfoProvided() {
			vcConfig.SecretRef = DefaultCredentialManager
		} else if vcConfig.isSecretInfoProvided() {
			vcConfig.SecretRef = vcConfig.SecretNamespace + "/" + vcConfig.SecretName
		}

		if vcConfig.VCenterPort == "" {
			vcConfig.VCenterPort = cy.Global.VCenterPort
		}

		if len(vcConfig.Datacenters) == 0 {
			if len(cy.Global.Datacenters) != 0 {
				vcConfig.Datacenters = cy.Global.Datacenters
			}
		}
		if vcConfig.RoundTripperCount == 0 {
			vcConfig.RoundTripperCount = cy.Global.RoundTripperCount
		}
		if vcConfig.CAFile == "" {
			vcConfig.CAFile = cy.Global.CAFile
		}
		if vcConfig.Thumbprint == "" {
			vcConfig.Thumbprint = cy.Global.Thumbprint
		}

		if len(vcConfig.IPFamilyPriority) == 0 {
			vcConfig.IPFamilyPriority = cy.Global.IPFamilyPriority
		}

		insecure := vcConfig.InsecureFlag
		if !insecure {
			vcConfig.InsecureFlag = cy.Global.InsecureFlag
		}
	}

	return nil
}

// ReadRawConfigYAML parses vSphere cloud config file and stores it into ConfigYAML
func ReadRawConfigYAML(byConfig []byte) (*ConfigYAML, error) {
	if len(byConfig) == 0 {
		klog.Errorf("Invalid YAML file")
		return nil, fmt.Errorf("Invalid YAML file")
	}

	cfg := ConfigYAML{}

	if err := yaml.Unmarshal(byConfig, &cfg); err != nil {
		klog.Errorf("Unmarshal failed: %s", err)
		return nil, err
	}

	err := cfg.validateConfig()
	if err != nil {
		klog.Errorf("validateConfig failed: %s", err)
		return nil, err
	}

	return &cfg, nil
}

// ReadConfigYAML parses vSphere cloud config file and stores it into Config
func ReadConfigYAML(byConfig []byte) (*Config, error) {
	cfg, err := ReadRawConfigYAML(byConfig)
	if err != nil {
		return nil, err
	}

	return cfg.CreateConfig(), nil
}
