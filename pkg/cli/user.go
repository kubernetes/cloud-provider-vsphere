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
	"os"

	"github.com/vmware/govmomi/ssoadmin"
	"github.com/vmware/govmomi/ssoadmin/types"
	"github.com/vmware/govmomi/sts"
	"github.com/vmware/govmomi/vim25/soap"
)

type User struct {
	id string
	types.AdminPersonDetails
	password string
	solution types.AdminSolutionDetails
	role     string
	cert     string
}

// CreateUserFunc is function to create person user or solution user
type CreateUserFunc func(c *ssoadmin.Client) error

func (u *User) Run(ctx context.Context, c *ClientOption, fn CreateUserFunc) error {

	vc, err := c.GetClient()
	if err != nil {
		return err
	}

	ssoClient, err := ssoadmin.NewClient(ctx, vc)
	if err != nil {
		return err
	}

	token := os.Getenv("SSO_LOGIN_TOKEN")
	header := soap.Header{
		Security: &sts.Signer{
			Certificate: vc.Certificate(),
			Token:       token,
		},
	}
	if token == "" {
		tokens, cerr := sts.NewClient(ctx, vc)
		if cerr != nil {
			return cerr
		}
		req := sts.TokenRequest{
			Certificate: vc.Certificate(),
			Userinfo:    c.Userinfo(),
		}

		header.Security, cerr = tokens.Issue(ctx, req)
		if cerr != nil {
			return cerr
		}
	}

	if err = ssoClient.Login(ssoClient.WithHeader(ctx, header)); err != nil {
		return err
	}
	defer ssoClient.Logout(ctx)

	return fn(ssoClient)
}
