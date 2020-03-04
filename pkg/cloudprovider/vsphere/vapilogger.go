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

package vsphere

import (
	"github.com/vmware/vsphere-automation-sdk-go/runtime/log"
	"k8s.io/klog"
)

// klogBridge is a connector for the vapi logger to klog
// the github.com/vmware/vsphere-automation-sdk-go SDK used for the NSX-T
// load balancer support logs a lot of stuff on its own logger defaulted
// to standard output. This bridge redirects the SDK log to the
// logging environment used by the controller manager (klog).
type klogBridge struct{}

// NewKlogBridge provides a vapi logger with klog backend
func NewKlogBridge() log.Logger {
	return klogBridge{}
}

func (d klogBridge) Error(args ...interface{}) {
	klog.Error(args...)
}

func (d klogBridge) Errorf(a string, args ...interface{}) {
	klog.Errorf(a, args...)
}

func (d klogBridge) Info(args ...interface{}) {
	klog.Info(args...)
}

func (d klogBridge) Infof(a string, args ...interface{}) {
	klog.Infof(a, args...)
}

func (d klogBridge) Debug(args ...interface{}) {
	klog.V(4).Info(args...)
}

func (d klogBridge) Debugf(a string, args ...interface{}) {
	klog.V(4).Infof(a, args...)
}
