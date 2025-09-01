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

package vsphere

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

// Mock implementations for testing
type mockInstances struct {
	instanceIDFunc                   func(ctx context.Context, nodeName types.NodeName) (string, error)
	instanceExistsByProviderIDFunc   func(ctx context.Context, providerID string) (bool, error)
	instanceShutdownByProviderIDFunc func(ctx context.Context, providerID string) (bool, error)
	instanceTypeByProviderIDFunc     func(ctx context.Context, providerID string) (string, error)
	nodeAddressesByProviderIDFunc    func(ctx context.Context, providerID string) ([]v1.NodeAddress, error)
}

func (m *mockInstances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	return nil, nil
}

func (m *mockInstances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	if m.nodeAddressesByProviderIDFunc != nil {
		return m.nodeAddressesByProviderIDFunc(ctx, providerID)
	}
	return nil, nil
}

func (m *mockInstances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	if m.instanceIDFunc != nil {
		return m.instanceIDFunc(ctx, nodeName)
	}
	return "", nil
}

func (m *mockInstances) InstanceType(ctx context.Context, nodeName types.NodeName) (string, error) {
	return "", nil
}

func (m *mockInstances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	if m.instanceTypeByProviderIDFunc != nil {
		return m.instanceTypeByProviderIDFunc(ctx, providerID)
	}
	return "", nil
}

func (m *mockInstances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	if m.instanceExistsByProviderIDFunc != nil {
		return m.instanceExistsByProviderIDFunc(ctx, providerID)
	}
	return false, nil
}

func (m *mockInstances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	if m.instanceShutdownByProviderIDFunc != nil {
		return m.instanceShutdownByProviderIDFunc(ctx, providerID)
	}
	return false, nil
}

func (m *mockInstances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return nil
}

func (m *mockInstances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return "", nil
}

func (m *mockInstances) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	return "", nil
}

type mockZones struct {
	getZoneByProviderIDFunc func(ctx context.Context, providerID string) (cloudprovider.Zone, error)
}

func (m *mockZones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, nil
}

func (m *mockZones) GetZoneByProviderID(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
	if m.getZoneByProviderIDFunc != nil {
		return m.getZoneByProviderIDFunc(ctx, providerID)
	}
	return cloudprovider.Zone{}, nil
}

func (m *mockZones) ListZones(ctx context.Context) ([]cloudprovider.Zone, error) {
	return nil, nil
}

func (m *mockZones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	return cloudprovider.Zone{}, nil
}
