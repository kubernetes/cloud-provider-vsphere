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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ssoadmin"
	"github.com/vmware/govmomi/ssoadmin/types"
	"github.com/vmware/govmomi/sts"
	"github.com/vmware/govmomi/vim25/soap"
	vimType "github.com/vmware/govmomi/vim25/types"
)

// User contains information about a person added to the system as a user.
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

// Run executes the supplied CreateUserFunc.
func (u *User) Run(ctx context.Context, c *ClientOption, fn CreateUserFunc) error {

	u.id = "k8s-vcp"

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

// RolePermission represents the permissions to assign to a role.
type RolePermission struct {
	Roles       object.AuthorizationRoleList `json:",omitempty"`
	Permissions []vimType.Permission         `json:",omitempty"`
	am          *object.AuthorizationManager
}

// GetRolePermission returns RolePermission by User
func GetRolePermission(ctx context.Context, c *ClientOption) (*RolePermission, error) {
	vc, err := c.GetClient()
	if err != nil {
		return nil, err
	}
	r := RolePermission{}
	r.am = object.NewAuthorizationManager(vc)
	r.Roles, err = r.am.RoleList(ctx)

	return &r, err
}

// Role represents a role and its privileges.
type Role struct {
	RoleName   string
	Privileges []string
}

// createCert creates a key pair for login purpose
func (u *User) createCert() error {
	id := strings.TrimSuffix(u.cert, ".crt")
	var encodedCert bytes.Buffer
	certFile, err := os.Create(id + ".crt")
	if err != nil {
		return err
	}
	defer certFile.Close()

	keyFile, err := os.Create(id + ".key")
	if err != nil {
		return err
	}
	defer keyFile.Close()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(5 * 365 * 24 * time.Hour) // 5 years

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"kubernetes"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	err = pem.Encode(&encodedCert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		return err
	}

	_, err = certFile.Write(encodedCert.Bytes())
	if err != nil {
		return err
	}

	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err != nil {
		return err
	}

	return nil
}
