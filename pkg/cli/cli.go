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
	"context"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/vmware/govmomi/ssoadmin"
	"github.com/vmware/govmomi/ssoadmin/types"
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

// CreateSolutionUser creates a default solution user (k8s-vcp) for CCM with Administrator role and WSTrust permissions
func CreateSolutionUser(ctx context.Context, o *ClientOption) error {
	u := User{}
	u.role = o.getCredential().role
	u.cert = o.getCredential().cert
	return u.Run(ctx, o, CreateUserFunc(func(c *ssoadmin.Client) error {
		// TODO (fanz): By far, cert data is provided from crt file (by --cert flag)
		//  Will add an option to generate key-pairs in separate PR.
		if cert, err := ReadContent(u.cert); err == nil {
			block, _ := pem.Decode([]byte(cert))
			if block != nil {
				u.solution.Certificate = base64.StdEncoding.EncodeToString(block.Bytes)
			}
			u.solution.Certificate = cert
		}
		if u.solution.Certificate == "" {
			return fmt.Errorf("Need solution user certificate (--cert) to create solution user")
		}
		u.solution.Description = u.AdminPersonDetails.Description
		if err := c.CreateSolutionUser(ctx, u.id, u.solution); err != nil {
			return err
		}
		p := types.PrincipalId{Name: "k8s-vcp", Domain: c.Domain}

		// TODO (fanz): Create a role with the minimum set of privileges required by VCP before SetRole
		if _, err := c.SetRole(ctx, p, u.role); err != nil {
			return err
		}
		if _, err := c.GrantWSTrustRole(ctx, p, types.RoleActAsUser); err != nil {
			return err
		}
		return nil
	}))
}

// TODO (fanz) :  Convert old in-tree vsphere.conf configuration files to new configmap
func ConvertOldConfig(old string) (vsphere.Config, error) {
	return vsphere.Config{}, nil
}
