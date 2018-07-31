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

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/soap"
)

type ClientOption struct {
	hostURL  string
	insecure bool
}

func (o *ClientOption) NewClient(ctx context.Context) (*govmomi.Client, error) {
	u, err := soap.ParseURL(o.hostURL)
	if err != nil {
		return nil, err
	}
	return govmomi.NewClient(ctx, u, o.insecure)
}
