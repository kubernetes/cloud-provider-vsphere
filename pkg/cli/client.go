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
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/soap"
)

type ClientOption struct {
	insecure   bool
	credential Credential
	Client     *govmomi.Client
	url        *url.URL
	// config Config
	Config Config
}

func (o *ClientOption) NewClient(ctx context.Context, hostURL string) (*govmomi.Client, error) {
	if o.Client != nil {
		return o.Client, nil
	}

	u, err := soap.ParseURL(hostURL)
	if err != nil {
		return nil, err
	}
	o.processCredential(u)
	o.url = u
	client, err := govmomi.NewClient(ctx, o.url, o.insecure)
	if err != nil {
		return client, err
	}
	o.Client = client
	return o.Client, nil
}

func (o *ClientOption) processCredential(u *url.URL) error {
	// consider secret, and env for credentials

	envUsername := o.getCredential().username
	envPassword := o.getCredential().password
	// Override username if provided
	if envUsername != "" {
		var password string
		var ok bool

		if u.User != nil {
			password, ok = u.User.Password()
		}

		if ok {
			u.User = url.UserPassword(envUsername, password)
		} else {
			u.User = url.User(envUsername)
		}
	}

	// Override password if provided
	if envPassword != "" {
		var username string

		if u.User != nil {
			username = u.User.Username()
		}

		u.User = url.UserPassword(username, envPassword)
	}
	return nil
}

func (o *ClientOption) GetClient() (*vim25.Client, error) {
	if o.Client.Client != nil {
		return o.Client.Client, nil
	}
	return nil, nil
}

func (o *ClientOption) Userinfo() *url.Userinfo {
	return o.url.User
}

func (o *ClientOption) getCredential() Credential {
	return o.credential
}

type Credential struct {
	username string
	password string
	cert     string
	role     string
	Secret   VCCMSecret
}

type VCCMSecret struct {
	Name string
	Data map[string]string
}

func (o *ClientOption) LoadCredential(username, password, cert, role string) {
	c := Credential{}
	c.username = username
	c.password = password
	c.cert = cert
	c.role = role
	// TODO: (fanz) Secret of Credential
	o.credential = c
}
