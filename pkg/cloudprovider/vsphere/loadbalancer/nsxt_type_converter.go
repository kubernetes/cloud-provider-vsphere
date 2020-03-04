/*
 Copyright 2020 The Kubernetes Authors.

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

package loadbalancer

import (
	"fmt"

	"github.com/vmware/vsphere-automation-sdk-go/runtime/bindings"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

type nsxtTypeConverter struct {
	bindings.TypeConverter
}

func newNsxtTypeConverter() *nsxtTypeConverter {
	converter := bindings.NewTypeConverter()
	converter.SetMode(bindings.REST)
	return &nsxtTypeConverter{TypeConverter: *converter}
}

func (c *nsxtTypeConverter) createLBSnatAutoMap() (*data.StructValue, error) {
	entry := model.LBSnatAutoMap{
		Type_: model.LBSnatAutoMap__TYPE_IDENTIFIER,
	}

	dataValue, errs := c.ConvertToVapi(entry, model.LBSnatAutoMapBindingType())
	if errs != nil {
		return nil, errs[0]
	}

	return dataValue.(*data.StructValue), nil
}

func (c *nsxtTypeConverter) convertLBTCPMonitorProfileToStructValue(monitor model.LBTcpMonitorProfile) (*data.StructValue, error) {
	dataValue, errs := c.ConvertToVapi(monitor, model.LBTcpMonitorProfileBindingType())
	if errs != nil {
		return nil, errs[0]
	}

	return dataValue.(*data.StructValue), nil
}

func (c *nsxtTypeConverter) convertStructValueToLBTCPMonitorProfile(dataValue *data.StructValue) (model.LBTcpMonitorProfile, error) {
	itf, errs := c.ConvertToGolang(dataValue, model.LBTcpMonitorProfileBindingType())
	if errs != nil {
		return model.LBTcpMonitorProfile{}, errs[0]
	}

	profile, ok := itf.(model.LBTcpMonitorProfile)
	if !ok {
		return model.LBTcpMonitorProfile{}, fmt.Errorf("converting struct value to LBTcpMonitorProfile failed")
	}
	return profile, nil
}
