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
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vapi/tags"
	vimtypes "github.com/vmware/govmomi/vim25/types"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"

	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
)

func TestNewInstancesV2(t *testing.T) {
	instances := &mockInstances{}
	zones := &mockZones{}

	result := newInstancesV2(instances, zones)

	if result == nil {
		t.Error("newInstancesV2 returned nil")
	}

	instancesV2, ok := result.(*instancesV2)
	if !ok {
		t.Error("newInstancesV2 did not return *instancesV2")
	}

	if instancesV2.instances != instances {
		t.Error("instancesV2.instances was not set correctly")
	}

	if instancesV2.zones != zones {
		t.Error("instancesV2.zones was not set correctly")
	}
}

func TestGetProviderID(t *testing.T) {
	testCases := []struct {
		name        string
		node        *v1.Node
		mockFunc    func(ctx context.Context, nodeName types.NodeName) (string, error)
		expected    string
		expectError bool
	}{
		{
			name: "node with existing provider ID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://existing-provider-id",
				},
			},
			expected:    "vsphere://existing-provider-id",
			expectError: false,
		},
		{
			name: "node without provider ID, successful instance ID lookup",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			mockFunc: func(ctx context.Context, nodeName types.NodeName) (string, error) {
				return "instance-123", nil
			},
			expected:    "vsphere://instance-123",
			expectError: false,
		},
		{
			name: "node without provider ID, failed instance ID lookup",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			mockFunc: func(ctx context.Context, nodeName types.NodeName) (string, error) {
				return "", errors.New("instance not found")
			},
			expected:    "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instances := &mockInstances{
				instanceIDFunc: tc.mockFunc,
			}
			zones := &mockZones{}

			c := &instancesV2{
				instances: instances,
				zones:     zones,
			}

			ctx := context.Background()
			result, err := c.getProviderID(ctx, tc.node)

			if tc.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if result != tc.expected {
				t.Errorf("expected %s but got %s", tc.expected, result)
			}
		})
	}
}

func TestInstanceExists(t *testing.T) {
	testCases := []struct {
		name                           string
		node                           *v1.Node
		instanceIDFunc                 func(ctx context.Context, nodeName types.NodeName) (string, error)
		instanceExistsByProviderIDFunc func(ctx context.Context, providerID string) (bool, error)
		expected                       bool
		expectError                    bool
	}{
		{
			name: "instance exists",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceExistsByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return true, nil
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "instance does not exist",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceExistsByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return false, nil
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "error checking instance existence",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceExistsByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return false, errors.New("provider error")
			},
			expected:    false,
			expectError: true,
		},
		{
			name: "error getting instance ID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			instanceIDFunc: func(ctx context.Context, nodeName types.NodeName) (string, error) {
				return "", errors.New("instance ID lookup failed")
			},
			expected:    false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instances := &mockInstances{
				instanceIDFunc:                 tc.instanceIDFunc,
				instanceExistsByProviderIDFunc: tc.instanceExistsByProviderIDFunc,
			}
			zones := &mockZones{}

			c := &instancesV2{
				instances: instances,
				zones:     zones,
			}

			ctx := context.Background()
			result, err := c.InstanceExists(ctx, tc.node)

			if tc.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if result != tc.expected {
				t.Errorf("expected %t but got %t", tc.expected, result)
			}
		})
	}
}

func TestInstanceShutdown(t *testing.T) {
	testCases := []struct {
		name                             string
		node                             *v1.Node
		instanceIDFunc                   func(ctx context.Context, nodeName types.NodeName) (string, error)
		instanceShutdownByProviderIDFunc func(ctx context.Context, providerID string) (bool, error)
		expected                         bool
		expectError                      bool
	}{
		{
			name: "instance is shutdown",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceShutdownByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return true, nil
			},
			expected:    true,
			expectError: false,
		},
		{
			name: "instance is not shutdown",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceShutdownByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return false, nil
			},
			expected:    false,
			expectError: false,
		},
		{
			name: "error checking instance shutdown",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceShutdownByProviderIDFunc: func(ctx context.Context, providerID string) (bool, error) {
				return false, errors.New("provider error")
			},
			expected:    false,
			expectError: true,
		},
		{
			name: "error getting instance ID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			instanceIDFunc: func(ctx context.Context, nodeName types.NodeName) (string, error) {
				return "", errors.New("instance ID lookup failed")
			},
			expected:    false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instances := &mockInstances{
				instanceIDFunc:                   tc.instanceIDFunc,
				instanceShutdownByProviderIDFunc: tc.instanceShutdownByProviderIDFunc,
			}
			zones := &mockZones{}

			c := &instancesV2{
				instances: instances,
				zones:     zones,
			}

			ctx := context.Background()
			result, err := c.InstanceShutdown(ctx, tc.node)

			if tc.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if result != tc.expected {
				t.Errorf("expected %t but got %t", tc.expected, result)
			}
		})
	}
}

func TestGetAdditionalLabels(t *testing.T) {
	// Save original value
	originalLabels := AdditionalLabels
	defer func() {
		AdditionalLabels = originalLabels
	}()

	testCases := []struct {
		name             string
		additionalLabels map[string]string
		expected         map[string]string
	}{
		{
			name:             "nil additional labels",
			additionalLabels: nil,
			expected:         nil,
		},
		{
			name:             "empty additional labels",
			additionalLabels: map[string]string{},
			expected:         map[string]string{},
		},
		{
			name: "additional labels with values",
			additionalLabels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
			expected: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set the global variable
			AdditionalLabels = tc.additionalLabels

			instances := &mockInstances{}
			zones := &mockZones{}

			c := &instancesV2{
				instances: instances,
				zones:     zones,
			}

			result, err := c.getAdditionalLabels()

			if err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestInstanceMetadata(t *testing.T) {
	testCases := []struct {
		name                          string
		node                          *v1.Node
		instanceIDFunc                func(ctx context.Context, nodeName types.NodeName) (string, error)
		instanceTypeByProviderIDFunc  func(ctx context.Context, providerID string) (string, error)
		getZoneByProviderIDFunc       func(ctx context.Context, providerID string) (cloudprovider.Zone, error)
		nodeAddressesByProviderIDFunc func(ctx context.Context, providerID string) ([]v1.NodeAddress, error)
		additionalLabels              map[string]string
		expected                      *cloudprovider.InstanceMetadata
		expectError                   bool
	}{
		{
			name: "successful metadata retrieval",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceTypeByProviderIDFunc: func(ctx context.Context, providerID string) (string, error) {
				return "m1.large", nil
			},
			getZoneByProviderIDFunc: func(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
				return cloudprovider.Zone{
					FailureDomain: "zone-a",
					Region:        "region-1",
				}, nil
			},
			nodeAddressesByProviderIDFunc: func(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
				return []v1.NodeAddress{
					{
						Type:    v1.NodeExternalIP,
						Address: "1.2.3.4",
					},
				}, nil
			},
			additionalLabels: map[string]string{
				"custom-label": "custom-value",
			},
			expected: &cloudprovider.InstanceMetadata{
				ProviderID:   "vsphere://instance-123",
				InstanceType: "m1.large",
				NodeAddresses: []v1.NodeAddress{
					{
						Type:    v1.NodeExternalIP,
						Address: "1.2.3.4",
					},
				},
				Zone:   "zone-a",
				Region: "region-1",
				AdditionalLabels: map[string]string{
					"custom-label": "custom-value",
				},
			},
			expectError: false,
		},
		{
			name: "error getting instance ID",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "",
				},
			},
			instanceIDFunc: func(ctx context.Context, nodeName types.NodeName) (string, error) {
				return "", errors.New("instance ID lookup failed")
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "error getting instance type",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceTypeByProviderIDFunc: func(ctx context.Context, providerID string) (string, error) {
				return "", errors.New("instance type lookup failed")
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "error getting zone",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceTypeByProviderIDFunc: func(ctx context.Context, providerID string) (string, error) {
				return "m1.large", nil
			},
			getZoneByProviderIDFunc: func(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
				return cloudprovider.Zone{}, errors.New("zone lookup failed")
			},
			expected:    nil,
			expectError: true,
		},
		{
			name: "error getting node addresses",
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-node",
				},
				Spec: v1.NodeSpec{
					ProviderID: "vsphere://instance-123",
				},
			},
			instanceTypeByProviderIDFunc: func(ctx context.Context, providerID string) (string, error) {
				return "m1.large", nil
			},
			getZoneByProviderIDFunc: func(ctx context.Context, providerID string) (cloudprovider.Zone, error) {
				return cloudprovider.Zone{
					FailureDomain: "zone-a",
					Region:        "region-1",
				}, nil
			},
			nodeAddressesByProviderIDFunc: func(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
				return nil, errors.New("node addresses lookup failed")
			},
			expected:    nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Save original value
			originalLabels := AdditionalLabels
			defer func() {
				AdditionalLabels = originalLabels
			}()

			// Set the global variable
			AdditionalLabels = tc.additionalLabels

			instances := &mockInstances{
				instanceIDFunc:                tc.instanceIDFunc,
				instanceTypeByProviderIDFunc:  tc.instanceTypeByProviderIDFunc,
				nodeAddressesByProviderIDFunc: tc.nodeAddressesByProviderIDFunc,
			}
			zones := &mockZones{
				getZoneByProviderIDFunc: tc.getZoneByProviderIDFunc,
			}

			c := &instancesV2{
				instances: instances,
				zones:     zones,
			}

			ctx := context.Background()
			result, err := c.InstanceMetadata(ctx, tc.node)

			if tc.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tc.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("expected %+v but got %+v", tc.expected, result)
			}
		})
	}
}

func TestInstanceMetadataAfterSessionExpired(t *testing.T) {
	ctx := context.Background()

	cfg, close := configFromEnvOrSim(false)
	defer close()

	connMgr := cm.NewConnectionManager(&cfg.Config, nil, nil)
	defer connMgr.Logout()
	vsi := connMgr.VsphereInstanceMap[cfg.Global.VCenterIP]
	if err := connMgr.Connect(ctx, vsi); err != nil {
		t.Fatalf("Failed to connect to vSphere: %s", err)
	}

	nm := newNodeManager(nil, connMgr)
	c := newInstancesV2(newInstances(nm), newZones(nm, cfg.Labels.Zone, cfg.Labels.Region))

	myvm := cfg.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	myvm.Guest.HostName = myvm.Name
	myvm.Guest.Net = []vimtypes.GuestNicInfo{{Network: "foo-bar", IpAddress: []string{"10.0.0.1"}}}
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: myvm.Name},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{SystemUUID: ConvertK8sUUIDtoNormal(myvm.Config.Uuid)},
		},
	}
	nm.RegisterNode(node)
	node.Spec.ProviderID = ProviderPrefix + myvm.Config.Uuid

	// Tag the VM's host with a region and a zone
	rc := rest.NewClient(vsi.Conn.Client)
	if err := rc.Login(ctx, url.UserPassword(vsi.Conn.Username, vsi.Conn.Password)); err != nil {
		t.Fatal(err)
	}
	m := tags.NewManager(rc)
	host := cfg.Map.Get(*myvm.Runtime.Host)
	for category, tag := range map[string]string{cfg.Labels.Region: "k8s-region-US", cfg.Labels.Zone: "k8s-zone-US-CA1"} {
		categoryID, err := m.CreateCategory(ctx, &tags.Category{Name: category})
		if err != nil {
			t.Fatal(err)
		}
		tagID, err := m.CreateTag(ctx, &tags.Tag{CategoryID: categoryID, Name: tag})
		if err != nil {
			t.Fatal(err)
		}
		if err := m.AttachTag(ctx, tagID, host.Reference()); err != nil {
			t.Fatal(err)
		}
	}

	// Expire the session that the cached node VM was discovered with
	vsi.Conn.Logout(ctx)

	metadata, err := c.InstanceMetadata(ctx, node)
	if err != nil {
		t.Fatalf("InstanceMetadata() failed after the vCenter session expired: %v", err)
	}
	if metadata.Region != "k8s-region-US" || metadata.Zone != "k8s-zone-US-CA1" {
		t.Errorf("unexpected region/zone: %q/%q", metadata.Region, metadata.Zone)
	}
	if got := nm.nodeUUIDMap[GetUUIDFromProviderID(node.Spec.ProviderID)].vm.Client(); got != vsi.Conn.Client {
		t.Errorf("cached VM is not bound to the reconnected client")
	}
}
