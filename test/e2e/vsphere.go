/*
Copyright 2021 The Kubernetes Authors.

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

package e2e

import (
	"context"
	"errors"
	"net/url"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/soap"

	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

var ErrFieldNotFound = errors.New("field not found in the e2e config")

type VSphereClient interface {
}

// vSphere specific client for e2e testing
type vSphereTestClient struct {
	Config     *vSphereClientConfig
	Client     *govmomi.Client
	Finder     *find.Finder
	Datacenter *object.Datacenter
}

// configurations for VSphereClient
type vSphereClientConfig struct {
	username   string
	password   string
	server     string
	datacenter string
}

// NewVSphereClientConfigFromE2E extracts a vSphereClientConfig from the cluster-api e2e config
func NewVSphereClientConfigFromE2E(e *clusterctl.E2EConfig) (*vSphereClientConfig, error) {
	server, ok := e.Variables["VSPHERE_SERVER"]
	if !ok {
		return nil, ErrFieldNotFound
	}
	username, ok := e.Variables["VSPHERE_USERNAME"]
	if !ok {
		return nil, ErrFieldNotFound
	}
	password, ok := e.Variables["VSPHERE_PASSWORD"]
	if !ok {
		return nil, ErrFieldNotFound
	}
	datacenter, ok := e.Variables["VSPHERE_DATACENTER"]
	if !ok {
		return nil, ErrFieldNotFound
	}
	return &vSphereClientConfig{
		username:   username,
		password:   password,
		server:     server,
		datacenter: datacenter,
	}, nil
}

// CreateVSphereTestClient creates an vSphereTestClient when config is provided
func CreateVSphereTestClient(ctx context.Context, e2eConfig *clusterctl.E2EConfig) (VSphereClient, error) {
	config, err := NewVSphereClientConfigFromE2E(e2eConfig)
	if err != nil {
		return nil, err
	}
	serverURL, err := soap.ParseURL(config.server)
	if err != nil {
		return nil, err
	}
	serverURL.User = url.UserPassword(config.username, config.password)

	client, err := govmomi.NewClient(ctx, serverURL, true)
	if err != nil {
		return nil, err
	}

	finder := find.NewFinder(client.Client)
	datacenter, err := finder.DatacenterOrDefault(ctx, config.datacenter)
	if err != nil {
		return nil, err
	}
	return vSphereTestClient{Config: config, Client: client, Finder: finder, Datacenter: datacenter}, nil
}
