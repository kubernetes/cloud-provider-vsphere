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
	"time"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/session/keepalive"
	"github.com/vmware/govmomi/view"
	"github.com/vmware/govmomi/vim25/soap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// VSphereTestClient defines a VSphere client for e2e testing
type VSphereTestClient struct {
	Client     *govmomi.Client
	Manager    *view.Manager
	Finder     *find.Finder
	Datacenter *object.Datacenter
}

// initVSphereTestClient creates an VSphereTestClient when config is provided
func initVSphereTestClient(ctx context.Context, e2eConfig *clusterctl.E2EConfig) (*VSphereTestClient, error) {
	server := e2eConfig.GetVariable("VSPHERE_SERVER")
	username := e2eConfig.GetVariable("VSPHERE_USERNAME")
	password := e2eConfig.GetVariable("VSPHERE_PASSWORD")
	datacenter := e2eConfig.GetVariable("VSPHERE_DATACENTER")

	serverURL, err := soap.ParseURL(server)
	if err != nil {
		return nil, err
	}
	serverURL.User = url.UserPassword(username, password)

	client, err := govmomi.NewClient(ctx, serverURL, true)
	if err != nil {
		return nil, err
	}
	// To keep the session from timing out until the test suite finishes
	client.RoundTripper = keepalive.NewHandlerSOAP(client.RoundTripper, 1*time.Minute, nil)

	finder := find.NewFinder(client.Client)
	dc, err := finder.DatacenterOrDefault(ctx, datacenter)
	if err != nil {
		return nil, err
	}
	finder.SetDatacenter(dc)

	manager := view.NewManager(client.Client)
	if manager == nil {
		return nil, errors.New("fail to get the vSphere manager")
	}
	return &VSphereTestClient{Client: client, Finder: finder, Datacenter: dc, Manager: manager}, nil
}
