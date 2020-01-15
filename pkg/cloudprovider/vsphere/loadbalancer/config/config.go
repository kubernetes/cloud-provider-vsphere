/*
 Copyright 2020 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"gopkg.in/gcfg.v1"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

const (
	// DefaultLoadBalancerClass is the default load balancer class
	DefaultLoadBalancerClass = "default"
)

// LoadBalancerSizes contains the valid size names
var LoadBalancerSizes = sets.NewString(
	model.LBService_SIZE_SMALL,
	model.LBService_SIZE_MEDIUM,
	model.LBService_SIZE_LARGE,
	model.LBService_SIZE_XLARGE,
	model.LBService_SIZE_DLB,
)

// LBConfig  is used to read and store information from the cloud configuration file
type LBConfig struct {
	LoadBalancer        LoadBalancerConfig                  `gcfg:"LoadBalancer"`
	LoadBalancerClasses map[string]*LoadBalancerClassConfig `gcfg:"LoadBalancerClass"`
	NSXT                NsxtConfig                          `gcfg:"NSX-T"`
}

// LoadBalancerConfig contains the configuration for the load balancer itself
type LoadBalancerConfig struct {
	LoadBalancerClassConfig
	Size             string `gcfg:"size"`
	LBServiceID      string `gcfg:"lbServiceId"`
	Tier1GatewayPath string `gcfg:"tier1GatewayPath"`
	RawTags          string `gcfg:"tags"`
	AdditionalTags   map[string]string
}

// LoadBalancerClassConfig contains the configuration for a load balancer class
type LoadBalancerClassConfig struct {
	IPPoolName        string `gcfg:"ipPoolName"`
	IPPoolID          string `gcfg:"ipPoolID"`
	TCPAppProfileName string `gcfg:"tcpAppProfileName"`
	TCPAppProfilePath string `gcfg:"tcpAppProfilePath"`
	UDPAppProfileName string `gcfg:"udpAppProfileName"`
	UDPAppProfilePath string `gcfg:"udpAppProfilePath"`
}

// NsxtConfig contains the NSX-T specific configuration
type NsxtConfig struct {
	// NSX-T username.
	User string `gcfg:"user"`
	// NSX-T password in clear text.
	Password string `gcfg:"password"`
	// NSX-T host.
	Host string `gcfg:"host"`
	// InsecureFlag is to be set to true if NSX-T uses self-signed cert.
	InsecureFlag bool `gcfg:"insecure-flag"`

	VMCAccessToken     string `gcfg:"vmcAccessToken"`
	VMCAuthHost        string `gcfg:"vmcAuthHost"`
	ClientAuthCertFile string `gcfg:"client-auth-cert-file"`
	ClientAuthKeyFile  string `gcfg:"client-auth-key-file"`
	CAFile             string `gcfg:"ca-file"`
}

// IsEnabled checks whether the load balancer feature is enabled
// It is enabled if any flavor of the load balancer configuration is given.
func (cfg *LBConfig) IsEnabled() bool {
	return len(cfg.LoadBalancerClasses) > 0 || !cfg.LoadBalancer.IsEmpty()
}

func (cfg *LBConfig) validateConfig() error {
	if cfg.LoadBalancer.LBServiceID == "" && cfg.LoadBalancer.Tier1GatewayPath == "" {
		msg := "either load balancer service id or T1 gateway path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if cfg.LoadBalancer.TCPAppProfileName == "" && cfg.LoadBalancer.TCPAppProfilePath == "" {
		msg := "either load balancer TCP application profile name or path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if cfg.LoadBalancer.UDPAppProfileName == "" && cfg.LoadBalancer.UDPAppProfilePath == "" {
		msg := "either load balancer UDP application profile name or path required"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if !LoadBalancerSizes.Has(cfg.LoadBalancer.Size) {
		msg := fmt.Sprintf("load balancer size is invalid. Valid values are: %s", strings.Join(LoadBalancerSizes.List(), ","))
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if cfg.LoadBalancer.IPPoolID == "" && cfg.LoadBalancer.IPPoolName == "" {
		class, ok := cfg.LoadBalancerClasses[DefaultLoadBalancerClass]
		if !ok {
			msg := "no default load balancer class defined"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		} else if class.IPPoolName == "" && class.IPPoolID == "" {
			msg := "default load balancer class: ipPoolName and ipPoolID is empty"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	} else {
		if cfg.LoadBalancer.IPPoolName != "" && cfg.LoadBalancer.IPPoolID != "" {
			msg := "either load balancer ipPoolName or ipPoolID can be set"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	}
	return cfg.NSXT.validateConfig()
}

// IsEmpty checks whether the load balancer config is empty (no values specified)
func (cfg *LoadBalancerConfig) IsEmpty() bool {
	return cfg.Size == "" && cfg.LBServiceID == "" &&
		cfg.IPPoolID == "" && cfg.IPPoolName == "" &&
		cfg.Tier1GatewayPath == ""
}

func (cfg *NsxtConfig) validateConfig() error {
	if cfg.VMCAccessToken != "" {
		if cfg.VMCAuthHost == "" {
			msg := "vmc auth host must be provided if auth token is provided"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	} else if cfg.User != "" {
		if cfg.Password == "" {
			msg := "password is empty"
			klog.Errorf(msg)
			return fmt.Errorf(msg)
		}
	} else {
		msg := "either user or vmc access token must be set"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	if cfg.Host == "" {
		msg := "host is empty"
		klog.Errorf(msg)
		return fmt.Errorf(msg)
	}
	return nil
}

// FromEnv initializes the provided configuration object with values
// obtained from environment variables. If an environment variable is set
// for a property that's already initialized, the environment variable's value
// takes precedence.
func (cfg *NsxtConfig) FromEnv() error {
	if v := os.Getenv("NSXT_MANAGER_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("NSXT_USERNAME"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("NSXT_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("NSXT_ALLOW_UNVERIFIED_SSL"); v != "" {
		InsecureFlag, err := strconv.ParseBool(v)
		if err != nil {
			klog.Errorf("Failed to parse NSXT_ALLOW_UNVERIFIED_SSL: %s", err)
			return fmt.Errorf("Failed to parse NSXT_ALLOW_UNVERIFIED_SSL: %s", err)
		}
		cfg.InsecureFlag = InsecureFlag
	}
	if v := os.Getenv("NSXT_CLIENT_AUTH_CERT_FILE"); v != "" {
		cfg.ClientAuthCertFile = v
	}
	if v := os.Getenv("NSXT_CLIENT_AUTH_KEY_FILE"); v != "" {
		cfg.ClientAuthKeyFile = v
	}
	if v := os.Getenv("NSXT_CA_FILE"); v != "" {
		cfg.CAFile = v
	}

	return nil
}

// ReadConfig parses vSphere cloud config file and stores it into LBConfig.
// Environment variables are also checked
func ReadConfig(config io.Reader) (*LBConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("no vSphere cloud provider config file given")
	}

	cfg := &LBConfig{}

	if err := gcfg.FatalOnly(gcfg.ReadInto(cfg, config)); err != nil {
		return nil, err
	}

	err := cfg.CompleteAndValidate()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// CompleteAndValidate sets default values, overrides by env and validates the resulting config
func (cfg *LBConfig) CompleteAndValidate() error {
	if !cfg.IsEnabled() {
		return nil
	}

	cfg.LoadBalancer.AdditionalTags = map[string]string{}
	if cfg.LoadBalancer.RawTags != "" {
		err := json.Unmarshal([]byte(cfg.LoadBalancer.RawTags), &cfg.LoadBalancer.AdditionalTags)
		if err != nil {
			return fmt.Errorf("unmarshalling load balancer tags failed: %s", err)
		}
	}
	if cfg.LoadBalancerClasses == nil {
		cfg.LoadBalancerClasses = map[string]*LoadBalancerClassConfig{}
	}
	for _, class := range cfg.LoadBalancerClasses {
		if class.IPPoolName == "" {
			if class.IPPoolID == "" {
				class.IPPoolID = cfg.LoadBalancer.IPPoolID
				class.IPPoolName = cfg.LoadBalancer.IPPoolName
			}
		}
	}

	// Env Vars should override config file entries if present
	if err := cfg.NSXT.FromEnv(); err != nil {
		return err
	}

	return cfg.validateConfig()
}
