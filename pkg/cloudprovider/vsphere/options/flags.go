/*
Copyright 2025 The Kubernetes Authors.
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

package options

import (
	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere"
)

// AddFlags add the additional flags for the controller
func AddFlags(fs *pflag.FlagSet) {
	fs.StringToStringVar(&vsphere.AdditionalLabels, "node-labels", nil,
		"Additional labels to add to vSphere nodes during registration.  Each key must follow kubernetes label format.\n"+
			"Example: --node-labels=node.foo.bar=vsphere,foo.bar/mapi=")
}
