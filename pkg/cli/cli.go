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

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ssoadmin"
	"github.com/vmware/govmomi/ssoadmin/types"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/mo"
	vimType "github.com/vmware/govmomi/vim25/types"

	"k8s.io/cloud-provider-vsphere/pkg/common/config"
)

func ParseConfig(configFile string) (*config.Config, error) {
	if len(configFile) == 0 {
		return nil, fmt.Errorf("Please specify vsphere cloud config file, e.g. --config vsphere.conf")
	}
	if _, err := os.Stat(configFile); err != nil {
		return nil, fmt.Errorf("Can not find config file %s, %v", configFile, err)
	}
	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("Can not open config file %s, %v", configFile, err)
	}
	cfg, err := config.ReadConfig(f)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// CheckVSphereConfig performs vSphere health check on VMs
// TODO (fanz) : support checking network
func CheckVSphereConfig(ctx context.Context, o *ClientOption) error {
	c, err := o.GetClient()
	if err != nil {
		return err
	}
	vc := view.NewManager(c)
	cv, err := vc.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"VirtualMachine"}, true)
	if err != nil {
		return err
	}
	defer cv.Destroy(ctx)
	var vms []mo.VirtualMachine
	var vm *object.VirtualMachine

	config := []vimType.BaseOptionValue{&vimType.OptionValue{Key: "disk.enableUUID", Value: "1"}}

	err = cv.Retrieve(ctx, []string{"VirtualMachine"}, []string{"summary"}, &vms)
	if err != nil {
		return err
	}
	for _, v := range vms {
		if v.Summary.Config.Uuid == "" {
			name := v.Summary.Config.Name
			// TODO (fanz): filter vm for node in kubernetes cluster
			if !IsClusterNode(name) {
				continue
			}
			vm = object.NewVirtualMachine(c, v.Reference())
			spec := vimType.VirtualMachineConfigSpec{
				ExtraConfig: config,
			}
			task, err := vm.Reconfigure(ctx, spec)
			err = task.Wait(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateRole creates vSphere role
func CreateRole(ctx context.Context, o *ClientOption, role *Role) error {
	p, err := GetRolePermission(ctx, o)
	if err != nil {
		return err
	}
	_, err = p.am.AddRole(ctx, role.RoleName, role.Privileges)
	return err
}

// CreateSolutionUser creates a default solution user (k8s-vcp) for CCM with Administrator role and WSTrust permissions
func CreateSolutionUser(ctx context.Context, o *ClientOption) error {
	u := User{}
	u.role = o.getCredential().role
	u.cert = o.getCredential().cert
	return u.Run(ctx, o, CreateUserFunc(func(c *ssoadmin.Client) error {

		_, err := os.Stat(u.cert)
		if err == nil {
			return fmt.Errorf("cert file already exists (%s). Please delete the cert and key files", u.cert)
		} else if os.IsNotExist(err) {
			err = u.createCert()
			if err != nil {
				return fmt.Errorf("Create solution user certificate (%s) error: %s", u.cert, err)
			}
		} else {
			return fmt.Errorf("Invalid cert file or directory (%s), create solution user error : %s", u.cert, err)
		}

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
		p := types.PrincipalId{Name: u.id, Domain: c.Domain}

		if _, err := c.SetRole(ctx, p, u.role); err != nil {
			return err
		}
		if _, err := c.GrantWSTrustRole(ctx, p, types.RoleActAsUser); err != nil {
			return err
		}
		return nil
	}))
}
