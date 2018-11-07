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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"

	"gopkg.in/gcfg.v1"
)

const (
	DefaultRoundTripperCount uint   = 3
	DefaultAPIBinding        string = ":43001"
	DefaultK8sServiceAccount string = "cloud-controller-manager"
	DefaultVCenterPort       string = "443"
	DefaultSecretDirectory   string = "/etc/cloud/secrets"
)

// Error Messages
const (
	MissingUsernameErrMsg  = "Username is missing"
	MissingPasswordErrMsg  = "Password is missing"
	InvalidVCenterIPErrMsg = "vsphere.conf does not have the VirtualCenter IP address specified"
)

// Error constants
var (
	ErrUsernameMissing  = errors.New(MissingUsernameErrMsg)
	ErrPasswordMissing  = errors.New(MissingPasswordErrMsg)
	ErrInvalidVCenterIP = errors.New(InvalidVCenterIPErrMsg)
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

//ConfigFromEnv allows setting configuration via environment variables.
func ConfigFromEnv() (cfg Config, ok bool) {
	var err error

	//Init
	cfg.VirtualCenter = make(map[string]*VirtualCenterConfig)

	//Globals
	cfg.Global.VCenterIP = os.Getenv("VSPHERE_VCENTER")
	cfg.Global.VCenterPort = os.Getenv("VSPHERE_VCENTER_PORT")
	cfg.Global.User = os.Getenv("VSPHERE_USER")
	cfg.Global.Password = os.Getenv("VSPHERE_PASSWORD")
	cfg.Global.Datacenters = os.Getenv("VSPHERE_DATACENTER")

	var RoundTripCount uint
	if os.Getenv("VSPHERE_ROUNDTRIP_COUNT") != "" {
		var tmp uint64
		tmp, err = strconv.ParseUint(os.Getenv("VSPHERE_ROUNDTRIP_COUNT"), 10, 32)
		RoundTripCount = uint(tmp)
	} else {
		RoundTripCount = DefaultRoundTripperCount
	}
	if err != nil {
		glog.Fatalf("Failed to parse VSPHERE_ROUNDTRIP_COUNT: %s", err)
	}
	cfg.Global.RoundTripperCount = RoundTripCount

	var InsecureFlag bool
	if os.Getenv("VSPHERE_INSECURE") != "" {
		InsecureFlag, err = strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))
	} else {
		InsecureFlag = false
	}
	if err != nil {
		glog.Errorf("Failed to parse VSPHERE_INSECURE: %s", err)
		InsecureFlag = false
	}
	cfg.Global.InsecureFlag = InsecureFlag

	var APIDisable bool
	if os.Getenv("VSPHERE_API_DISABLE") != "" {
		APIDisable, err = strconv.ParseBool(os.Getenv("VSPHERE_API_DISABLE"))
	} else {
		APIDisable = true
	}
	if err != nil {
		glog.Errorf("Failed to parse VSPHERE_API_DISABLE: %s", err)
		APIDisable = true
	}
	cfg.Global.APIDisable = APIDisable

	var APIBinding string
	if os.Getenv("VSPHERE_API_BINDING") != "" {
		APIBinding = os.Getenv("VSPHERE_API_BINDING")
	} else {
		APIBinding = DefaultAPIBinding
	}
	cfg.Global.APIBinding = APIBinding

	var SecretsDirectory string
	if os.Getenv("VSPHERE_SECRETS_DIRECTORY") != "" {
		SecretsDirectory = os.Getenv("VSPHERE_SECRETS_DIRECTORY")
	} else {
		SecretsDirectory = DefaultSecretDirectory
	}
	if _, err := os.Stat(SecretsDirectory); os.IsNotExist(err) {
		SecretsDirectory = "" //Dir does not exist, set to empty string
	}
	cfg.Global.SecretsDirectory = SecretsDirectory

	cfg.Global.CAFile = os.Getenv("VSPHERE_CAFILE")
	cfg.Global.Thumbprint = os.Getenv("VSPHERE_THUMBPRINT")

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

			cfg.VirtualCenter[vcenter] = &VirtualCenterConfig{
				User:              username,
				Password:          password,
				VCenterPort:       port,
				InsecureFlag:      insecureFlag,
				Datacenters:       datacenters,
				RoundTripperCount: roundtrip,
				CAFile:            caFile,
				Thumbprint:        thumbprint,
			}
		}
	}

	if cfg.Global.VCenterIP != "" && cfg.VirtualCenter[cfg.Global.VCenterIP] == nil {
		cfg.VirtualCenter[cfg.Global.VCenterIP] = &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			VCenterPort:       cfg.Global.VCenterPort,
			InsecureFlag:      cfg.Global.InsecureFlag,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			CAFile:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
		}
	}

	//Valid config?
	for _, vcConfig := range cfg.VirtualCenter {
		if (vcConfig.User == "" && vcConfig.Password == "") ||
			(vcConfig.CAFile == "" && vcConfig.Thumbprint == "") {
			ok = false
			return
		}
	}
	ok = (cfg.Global.VCenterIP != "" && cfg.Global.User != "" && cfg.Global.Password != "")
	return
}

func fixUpConfigFromFile(cfg *Config) error {
	//Fix default global values
	if cfg.Global.RoundTripperCount == 0 {
		cfg.Global.RoundTripperCount = DefaultRoundTripperCount
	}
	if cfg.Global.ServiceAccount == "" {
		cfg.Global.ServiceAccount = DefaultK8sServiceAccount
	}
	if cfg.Global.VCenterPort == "" {
		cfg.Global.VCenterPort = DefaultVCenterPort
	}

	if len(cfg.Network.PublicNetwork) > 0 {
		glog.Warningf("Network section is deprecated, please remove it from your configuration.")
	}

	isSecretInfoProvided := true
	if (cfg.Global.SecretName == "" || cfg.Global.SecretNamespace == "") && cfg.Global.SecretsDirectory == "" {
		isSecretInfoProvided = false
	}

	// vsphere.conf is no longer supported in the old format.
	for vcServer, vcConfig := range cfg.VirtualCenter {
		glog.V(4).Infof("Initializing vc server %s", vcServer)
		if vcServer == "" {
			glog.Error(InvalidVCenterIPErrMsg)
			return ErrInvalidVCenterIP
		}

		if !isSecretInfoProvided {
			if vcConfig.User == "" {
				vcConfig.User = cfg.Global.User
				if vcConfig.User == "" {
					glog.Errorf("vcConfig.User is empty for vc %s!", vcServer)
					return ErrUsernameMissing
				}
			}
			if vcConfig.Password == "" {
				vcConfig.Password = cfg.Global.Password
				if vcConfig.Password == "" {
					glog.Errorf("vcConfig.Password is empty for vc %s!", vcServer)
					return ErrPasswordMissing
				}
			}
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

		insecure := vcConfig.InsecureFlag
		if !insecure {
			insecure = cfg.Global.InsecureFlag
			vcConfig.InsecureFlag = cfg.Global.InsecureFlag
		}
	}

	// Create a single instance of VSphereInstance for the Global VCenterIP if the
	// VirtualCenter does not already exist in the map
	if !isSecretInfoProvided && cfg.Global.VCenterIP != "" && cfg.VirtualCenter[cfg.Global.VCenterIP] == nil {
		vcConfig := &VirtualCenterConfig{
			User:              cfg.Global.User,
			Password:          cfg.Global.Password,
			VCenterPort:       cfg.Global.VCenterPort,
			InsecureFlag:      cfg.Global.InsecureFlag,
			Datacenters:       cfg.Global.Datacenters,
			RoundTripperCount: cfg.Global.RoundTripperCount,
			CAFile:            cfg.Global.CAFile,
			Thumbprint:        cfg.Global.Thumbprint,
		}
		cfg.VirtualCenter[cfg.Global.VCenterIP] = vcConfig
	}

	return nil
}

//ReadConfig parses vSphere cloud config file and stores it into VSphereConfig.
func ReadConfig(config io.Reader) (Config, error) {
	if config == nil {
		return Config{}, fmt.Errorf("no vSphere cloud provider config file given")
	}

	cfg, _ := ConfigFromEnv()

	err := gcfg.ReadInto(&cfg, config)
	if err != nil {
		return cfg, err
	}

	err = fixUpConfigFromFile(&cfg)

	return cfg, err
}

//GenerateInstanceMap creates a map of vCenter connection objects that can be
//use to create a connection to a vCenter using vclib package
func GenerateInstanceMap(cfg Config) map[string]*VSphereInstance {
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
