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
	"fmt"
	"testing"

	"github.com/vmware/govmomi/ssoadmin"
	"github.com/vmware/govmomi/ssoadmin/types"
	"k8s.io/cloud-provider-vsphere/pkg/cli/test"
)

var (
	o = ClientOption{}
	u = User{}
)

func TestGetRolePermission(t *testing.T) {
	ctx := context.Background()
	f, err := buildClient(ctx)
	if err != nil {
		t.Fatalf("Test build ServiceInstance error : %s", err)
	}
	defer f()
	r, err := GetRolePermission(ctx, &o)
	if err != nil {
		t.Fatalf("GetRolePermission error : %s", err)
	}
	if r == nil {
		t.Fatalf("RolePermission error : %v", r)
	}

}

func TestRunUserFunc(t *testing.T) {
	ctx := context.Background()
	f, err := buildClient(ctx)
	if err != nil {
		t.Fatalf("Test build ServiceInstance error : %s", err)
	}
	defer f()

	// vcsim does not support sso-adminserver, so error is returned
	// if applying with a working sso-adminserver, FindUser should return expected administrator user
	expected := types.AdminUser{}
	expected.Id = types.PrincipalId{
		Name:   "Administrator",
		Domain: "vsphere.local",
	}
	fn := func(c *ssoadmin.Client) error {
		u, err := c.FindUser(ctx, "Administrator")
		if u.Id.Name != expected.Id.Name && u.Id.Domain != expected.Id.Domain {
			return fmt.Errorf("find AdminUser return error, expected (%v), but find (%v)", expected, u)
		}
		return err
	}
	err = u.Run(ctx, &o, fn)
	if err == nil {
		t.Fatalf("Run User Func should return error")
	}
}

func buildClient(ctx context.Context) (func(), error) {
	m, s, err := test.NewServiceInstance()
	if err != nil {
		return nil, err
	}
	fn := func() {
		s.Close()
		m.Remove()
	}
	c, err := o.NewClient(ctx, s.URL.String())
	if err != nil || c == nil {
		return fn, fmt.Errorf("create client error : %s", err)
	}
	o.Client = c
	o.url = s.URL
	o.insecure = true
	return fn, err
}
