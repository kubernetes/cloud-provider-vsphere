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
	"errors"
	"net/http"
	"testing"

	"github.com/vmware/govmomi/vim25/mo"
	"k8s.io/cloud-provider-vsphere/pkg/cli/test"
)

func TestNewClient(t *testing.T) {
	o := ClientOption{}
	m, s, err := test.NewServiceInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		s.Close()
		m.Remove()
	}()
	c, err := o.NewClient(context.Background(), s.URL.String())
	if err != nil {
		t.Fatal(err)
	}

	f := func() error {
		var x mo.Folder
		err = mo.RetrieveProperties(context.Background(), c, c.ServiceContent.PropertyCollector, c.ServiceContent.RootFolder, &x)
		if err != nil {
			return err
		}
		if len(x.Name) == 0 {
			return errors.New("empty response")
		}
		return nil
	}

	// check cookie is valid with an sdk request
	if err := f(); err != nil {
		t.Fatal(err)
	}

	// check cookie is valid with a non-sdk request
	o.url.User = nil // turn off Basic auth
	o.url.Path = "/folder"
	r, err := c.Client.Get(o.url.String())
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != http.StatusOK {
		t.Fatal(r)
	}

	// sdk request should fail w/o a valid cookie
	c.Client.Jar = nil
	if err := f(); err == nil {
		t.Fatal("should fail")
	}

}
