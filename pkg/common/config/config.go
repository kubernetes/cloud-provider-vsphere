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
	"io"
	"os"
	"strconv"
	"strings"

	"k8s.io/klog"

	"gopkg.in/gcfg.v1"
)

func getEnvKeyValue(match string, partial bool) (string, string, error) {
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		if len(pair) != 2 {
			continue
		}

		key := pair[0]
		value := pair[1]

		if partial && strings.Contains(key, match) {
			return key, value, nil
		}

		if strings.Compare(key, match) == 0 {
			return key, value, nil
		}
	}

	matchType := "match"
	if partial {
		matchType = "partial match"
	}

	return "", "", fmt.Errorf("Failed to find %s with %s", matchType, match)
}

// FromEnv initializes the provided configuration object with values
// obtained from environment variables. If an environment variable is set
// for a property that's already initialized, the environment variable's value
// takes precedence.
func (cfg *Config) FromEnv() error {

	//Init
	if cfg.VirtualCenter == nil {
		cfg.VirtualCenter = make(map[string]*VirtualCenterConfig)
	}

	//Globals
	if v := os.Getenv("VSPHERE_VCENTER"); v != "" {
		cfg.Global.VCenterIP = v
	}
	if v := os.Getenv("VSPHERE_VCENTER_PORT"); v != "" {
		cfg.Global.VCenterPort = v
	}
	if v := os.Getenv("VSPHERE_USER"); v != "" {
		cfg.Global.User = v
	}
	if v := os.Getenv("VSPHERE_PASSWORD"); v != "" {
		cfg.Global.Password = v
	}
	if v := os.Getenv("VSPHERE_DATACENTER"); v != "" {
		cfg.Global.Datacenters = v
	}
	if v := os.Getenv("VSPHERE_SECRET_NAME"); v != "" {
		cfg.Global.SecretName = v
	}
	if v := os.Getenv("VSPHERE_SECRET_NAMESPACE"); v != "" {
		cfg.Global.SecretNamespace = v
	}

	if v := os.Getenv("VSPHERE_ROUNDTRIP_COUNT"); v != "" {
		tmp, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			klog.Errorf("Failed to parse VSPHERE_ROUNDTRIP_COUNT: %s", err)
		} else {
			cfg.Global.RoundTripperCount = uint(tmp)
		}
	}

	if v := os.Getenv("VSPHERE_INSECURE"); v != "" {
		InsecureFlag, err := strconv.ParseBool(v)
		if err != nil {
			klog.Errorf("Failed to parse VSPHERE_INSECURE: %s", err)
		} else {
			cfg.Global.InsecureFlag = InsecureFlag
		}
	}

	if v := os.Getenv("VSPHERE_API_DISABLE"); v != "" {
		APIDisable, err := strconv.ParseBool(v)
		if err != nil {
			klog.Errorf("Failed to parse VSPHERE_API_DISABLE: %s", err)
		} else {
			cfg.Global.APIDisable = APIDisable
		}
	}

	if v := os.Getenv("VSPHERE_API_BINDING"); v != "" {
		cfg.Global.APIBinding = v
	}

	if v := os.Getenv("VSPHERE_SECRETS_DIRECTORY"); v != "" {
		cfg.Global.SecretsDirectory = v
	}
	if cfg.Global.SecretsDirectory == "" {
		cfg.Global.SecretsDirectory = DefaultSecretDirectory
	}
	if _, err := os.Stat(cfg.Global.SecretsDirectory); os.IsNotExist(err) {
		cfg.Global.SecretsDirectory = "" //Dir does not exist, set to empty string
	}

	if v := os.Getenv("VSPHERE_CAFILE"); v != "" {
		cfg.Global.CAFile = v
	}
	if v := os.Getenv("VSPHERE_THUMBPRINT"); v != "" {
		cfg.Global.Thumbprint = v
	}
	if v := os.Getenv("VSPHERE_LABEL_REGION"); v != "" {
		cfg.Labels.Region = v
	}
	if v := os.Getenv("VSPHERE_LABEL_ZONE"); v != "" {
		cfg.Labels.Zone = v
	}

	if v := os.Getenv("VSPHERE_IP_FAMILY"); v != "" {
		cfg.Global.IPFamily = v
	}
	if cfg.Global.IPFamily == "" {
		cfg.Global.IPFamily = DefaultIPFamily
	}

	//Build VirtualCenter from ENVs
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")

		if len(pair) != 2 {
			continue
		}

		key := pair[0]
		value := pair[1]

		if strings.HasPrefix(key, "VSPHERE_VCENTER_") && len(value) > 0 {
			id := strings.TrimPrefix(key, "VSPHERE_VCENTER_")
			vcenter := value

			_, username, errUsername := getEnvKeyValue("VCENTER_"+id+"_USERNAME", false)
			if errUsername != nil {
				username = cfg.Global.User
			}
			_, password, errPassword := getEnvKeyValue("VCENTER_"+id+"_PASSWORD", false)
			if errPassword != nil {
				password = cfg.Global.Password
			}
			_, server, errServer := getEnvKeyValue("VCENTER_"+id+"_SERVER", false)
			if errServer != nil {
				server = ""
			}
			_, port, errPort := getEnvKeyValue("VCENTER_"+id+"_PORT", false)
			if errPort != nil {
				port = cfg.Global.VCenterPort
			}
			insecureFlag := false
			_, insecureTmp, errInsecure := getEnvKeyValue("VCENTER_"+id+"_INSECURE", false)
			if errInsecure != nil {
				insecureFlagTmp, errTmp := strconv.ParseBool(insecureTmp)
				if errTmp == nil {
					insecureFlag = insecureFlagTmp
				}
			}
			_, datacenters, errDatacenters := getEnvKeyValue("VCENTER_"+id+"_DATACENTERS", false)
			if errDatacenters != nil {
				datacenters = cfg.Global.Datacenters
			}
			roundtrip := DefaultRoundTripperCount
			_, roundtripTmp, errRoundtrip := getEnvKeyValue("VCENTER_"+id+"_ROUNDTRIP", false)
			if errRoundtrip != nil {
				roundtripFlagTmp, errTmp := strconv.ParseUint(roundtripTmp, 10, 32)
				if errTmp == nil {
					roundtrip = uint(roundtripFlagTmp)
				}
			}
			_, caFile, errCaFile := getEnvKeyValue("VCENTER_"+id+"_CAFILE", false)
			if errCaFile != nil {
				caFile = cfg.Global.CAFile
			}
			_, thumbprint, errThumbprint := getEnvKeyValue("VCENTER_"+id+"_THUMBPRINT", false)
			if errThumbprint != nil {
				thumbprint = cfg.Global.Thumbprint
			}

			_, secretName, secretNameErr := getEnvKeyValue("VCENTER_"+id+"_SECRET_NAME", false)
			_, secretNamespace, secretNamespaceErr := getEnvKeyValue("VCENTER_"+id+"_SECRET_NAMESPACE", false)

			if secretNameErr != nil || secretNamespaceErr != nil {
				secretName = ""
				secretNamespace = ""
			}
			secretRef := DefaultCredentialManager
			if secretName != "" && secretNamespace != "" {
				secretRef = vcenter
			}

			_, ipFamily, errIPFamily := getEnvKeyValue("VCENTER_"+id+"_IP_FAMILY", false)
			if errIPFamily != nil {
				ipFamily = cfg.Global.IPFamily
			}

			// If server is explicitly set, that means the vcenter value above is the TenantRef
			vcenterIP := vcenter
			tenantRef := vcenter
			if server != "" {
				vcenterIP = server
				tenantRef = vcenter
			}

			cfg.VirtualCenter[tenantRef] = &VirtualCenterConfig{
				User:              username,
				Password:          password,
				TenantRef:         tenantRef,
				VCenterIP:         vcenterIP,
				VCenterPort:       port,
				InsecureFlag:      insecureFlag,
				Datacenters:       datacenters,
				RoundTripperCount: roundtrip,
				CAFile:            caFile,
				Thumbprint:        thumbprint,
				SecretRef:         secretRef,
				SecretName:        secretName,
				SecretNamespace:   secretNamespace,
				IPFamily:          ipFamily,
			}
		}
	}

	if cfg.Global.VCenterIP != "" && cfg.VirtualCenter[cfg.Global.VCenterIP] == nil {
		cfg.VirtualCenter[cfg.Global.VCenterIP] = &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			TenantRef:         cfg.Global.VCenterIP,
			VCenterIP:         cfg.Global.VCenterIP,
			VCenterPort:       cfg.Global.VCenterPort,
			InsecureFlag:      cfg.Global.InsecureFlag,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			CAFile:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
			SecretRef:         DefaultCredentialManager,
			SecretName:        cfg.Global.SecretName,
			SecretNamespace:   cfg.Global.SecretNamespace,
			IPFamily:          cfg.Global.IPFamily,
		}
	}

	err := cfg.validateConfig()
	if err != nil {
		return err
	}

	return nil
}

// IsSecretInfoProvided returns true if k8s secret is set or using generic CO secret method.
// If both k8s secret and generic CO both are true, we don't know which to use, so return false.
func (cfg *Config) IsSecretInfoProvided() bool {
	return (cfg.Global.SecretName != "" && cfg.Global.SecretNamespace != "" && cfg.Global.SecretsDirectory == "") ||
		(cfg.Global.SecretName == "" && cfg.Global.SecretNamespace == "" && cfg.Global.SecretsDirectory != "")
}

func validateIPFamily(value string) ([]string, error) {
	if len(value) == 0 {
		return []string{DefaultIPFamily}, nil
	}

	ipFamilies := strings.Split(value, ",")
	for i, ipFamily := range ipFamilies {
		ipFamily = strings.TrimSpace(ipFamily)
		if len(ipFamily) == 0 {
			copy(ipFamilies[i:], ipFamilies[i+1:])      // Shift a[i+1:] left one index.
			ipFamilies[len(ipFamilies)-1] = ""          // Erase last element (write zero value).
			ipFamilies = ipFamilies[:len(ipFamilies)-1] // Truncate slice.
			continue
		}
		if !strings.EqualFold(ipFamily, IPv4Family) && !strings.EqualFold(ipFamily, IPv6Family) {
			return nil, ErrInvalidIPFamilyType
		}
	}

	return ipFamilies, nil
}

func (cfg *Config) validateConfig() error {
	//Fix default global values
	if cfg.Global.RoundTripperCount == 0 {
		cfg.Global.RoundTripperCount = DefaultRoundTripperCount
	}
	if cfg.Global.VCenterPort == "" {
		cfg.Global.VCenterPort = DefaultVCenterPort
	}
	if cfg.Global.APIBinding == "" {
		cfg.Global.APIBinding = DefaultAPIBinding
	}
	if cfg.Global.IPFamily == "" {
		cfg.Global.IPFamily = DefaultIPFamily
	}

	ipFamilyPriority, err := validateIPFamily(cfg.Global.IPFamily)
	if err != nil {
		klog.Errorf("Invalid Global IPFamily: %s, err=%s", cfg.Global.IPFamily, err)
		return err
	}

	// Create a single instance of VSphereInstance for the Global VCenterIP if the
	// VirtualCenter does not already exist in the map
	if cfg.Global.VCenterIP != "" && cfg.VirtualCenter[cfg.Global.VCenterIP] == nil {
		vcConfig := &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			TenantRef:         cfg.Global.VCenterIP,
			VCenterIP:         cfg.Global.VCenterIP,
			VCenterPort:       cfg.Global.VCenterPort,
			InsecureFlag:      cfg.Global.InsecureFlag,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			CAFile:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
			SecretRef:         DefaultCredentialManager,
			SecretName:        cfg.Global.SecretName,
			SecretNamespace:   cfg.Global.SecretNamespace,
			IPFamily:          cfg.Global.IPFamily,
			IPFamilyPriority:  ipFamilyPriority,
		}
		cfg.VirtualCenter[cfg.Global.VCenterIP] = vcConfig
	}

	// Must have at least one vCenter defined
	if len(cfg.VirtualCenter) == 0 {
		klog.Error(ErrMissingVCenter)
		return ErrMissingVCenter
	}

	// vsphere.conf is no longer supported in the old format.
	for vcServer, vcConfig := range cfg.VirtualCenter {
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

		if !cfg.IsSecretInfoProvided() && !vcConfig.IsSecretInfoProvided() {
			if vcConfig.User == "" {
				vcConfig.User = cfg.Global.User
				if vcConfig.User == "" {
					klog.Errorf("vcConfig.User is empty for vc %s!", vcServer)
					return ErrUsernameMissing
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cfg.Global.Password
				if vcConfig.Password == "" {
					klog.Errorf("vcConfig.Password is empty for vc %s!", vcServer)
					return ErrPasswordMissing
				}
			}
		} else if cfg.IsSecretInfoProvided() && !vcConfig.IsSecretInfoProvided() {
			vcConfig.SecretRef = DefaultCredentialManager
		} else if vcConfig.IsSecretInfoProvided() {
			vcConfig.SecretRef = vcConfig.SecretNamespace + "/" + vcConfig.SecretName
		}

		if vcConfig.VCenterPort == "" {
			vcConfig.VCenterPort = cfg.Global.VCenterPort
		}

		if vcConfig.Datacenters == "" {
			if cfg.Global.Datacenters != "" {
				vcConfig.Datacenters = cfg.Global.Datacenters
			}
		}
		if vcConfig.RoundTripperCount == 0 {
			vcConfig.RoundTripperCount = cfg.Global.RoundTripperCount
		}
		if vcConfig.CAFile == "" {
			vcConfig.CAFile = cfg.Global.CAFile
		}
		if vcConfig.Thumbprint == "" {
			vcConfig.Thumbprint = cfg.Global.Thumbprint
		}

		if vcConfig.IPFamily == "" {
			vcConfig.IPFamily = cfg.Global.IPFamily
		}

		ipFamilyPriority, err := validateIPFamily(vcConfig.IPFamily)
		if err != nil {
			klog.Errorf("Invalid vcConfig IPFamily: %s, err=%s", vcConfig.IPFamily, err)
			return err
		}
		vcConfig.IPFamilyPriority = ipFamilyPriority

		insecure := vcConfig.InsecureFlag
		if !insecure {
			vcConfig.InsecureFlag = cfg.Global.InsecureFlag
		}
	}

	return nil
}

// ReadConfig parses vSphere cloud config file and stores it into VSphereConfig.
// Environment variables are also checked
func ReadConfig(config io.Reader) (*Config, error) {
	if config == nil {
		return nil, fmt.Errorf("no vSphere cloud provider config file given")
	}

	cfg := &Config{}

	if err := gcfg.FatalOnly(gcfg.ReadInto(cfg, config)); err != nil {
		return nil, err
	}

	// Env Vars should override config file entries if present
	if err := cfg.FromEnv(); err != nil {
		return nil, err
	}

	return cfg, nil
}
