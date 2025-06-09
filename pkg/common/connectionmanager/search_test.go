/*
Copyright 2019 The Kubernetes Authors.

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

package connectionmanager

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
)

func TestWhichVCandDCByNodeIdByUUID(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(&config.Config, nil, nil)
	defer connMgr.Logout()

	// setup
	vm := config.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	vm.Guest.HostName = strings.ToLower(name)
	UUID := vm.Config.Uuid

	// context
	ctx := context.Background()

	info, err := connMgr.WhichVCandDCByNodeID(ctx, UUID, FindVMByUUID)
	if err != nil {
		t.Fatalf("WhichVCandDCByNodeID err=%v", err)
	}
	if info == nil {
		t.Fatalf("WhichVCandDCByNodeID info=nil")
	}

	if !strings.EqualFold(name, info.NodeName) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
	if !strings.EqualFold(UUID, info.UUID) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
}

func TestWhichVCandDCByNodeIdByName(t *testing.T) {
	config, cleanup := configFromEnvOrSim(true)
	defer cleanup()

	connMgr := NewConnectionManager(&config.Config, nil, nil)
	defer connMgr.Logout()

	// setup
	vm := config.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	vm.Guest.HostName = strings.ToLower(name)
	UUID := vm.Config.Uuid

	// context
	ctx := context.Background()

	info, err := connMgr.WhichVCandDCByNodeID(ctx, name, FindVMByName)
	if err != nil {
		t.Fatalf("WhichVCandDCByNodeID err=%v", err)
	}
	if info == nil {
		t.Fatalf("WhichVCandDCByNodeID info=nil")
	}

	if !strings.EqualFold(name, info.NodeName) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
	if !strings.EqualFold(UUID, info.UUID) {
		t.Fatalf("VM name mismatch %s=%s", name, info.NodeName)
	}
}
