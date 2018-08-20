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

package cli

import (
	"fmt"
	"os"

	"github.com/vmware/govmomi"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere"
)

func ParseConfig(configFile string) (vsphere.Config, error) {
	var cfg vsphere.Config
	if len(configFile) == 0 {
		return cfg, fmt.Errorf("Please specify vsphere cloud config file, e.g. --config vsphere.conf")
	}
	if _, err := os.Stat(configFile); err != nil {
		return cfg, fmt.Errorf("Can not find config file %s, %v", configFile, err)
	}
	f, err := os.Open(configFile)
	if err != nil {
		return cfg, fmt.Errorf("Can not open config file %s, %v", configFile, err)
	}
	cfg, err = readConfig(f)
	if err != nil {
		return cfg, err
	}
	return cfg, err
}

// TODO (fanz) : Perform vSphere configuration health check on VM:
func CheckVSphereConfig(config vsphere.Config) error {
	return nil
}

// TODO (fanz) : Create vSphere role with minimal set of permissions
func CreateRole() error {
	return nil
}

// TODO (fanz) : Create vSphere solution user (generate keypair), to be used with CCM
func CreateSolutionUser(o ClientOption, c *govmomi.Client) error {
	return nil
}

// TODO (fanz) :  Convert old in-tree vsphere.conf configuration files to new configmap
func ConvertOldConfig(old string) (vsphere.Config, error) {
	return vsphere.Config{}, nil
}
