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

// This file implements the InstancesV2 interface.

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
)

// AdditionalLabels contains additional labels to add to all nodes when generating the metadata information
var AdditionalLabels map[string]string

func newInstancesV2(instances cloudprovider.Instances, zones cloudprovider.Zones) cloudprovider.InstancesV2 {
	return &instancesV2{
		instances: instances,
		zones:     zones,
	}
}

func (c *instancesV2) getProviderID(ctx context.Context, node *v1.Node) (string, error) {
	klog.V(4).Infof("instancesV2.getProviderID() called for node %s", node.Name)
	if node.Spec.ProviderID != "" {
		return node.Spec.ProviderID, nil
	}

	instanceID, err := c.instances.InstanceID(ctx, types.NodeName(node.Name))
	if err != nil {
		return "", err
	}

	return ProviderName + "://" + instanceID, nil
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (c *instancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).Infof("instancesV2.InstanceExists() called for node %s", node.Name)
	providerID, err := c.getProviderID(ctx, node)
	if err != nil {
		return false, err
	}

	return c.instances.InstanceExistsByProviderID(ctx, providerID)
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (c *instancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	klog.V(4).Infof("instancesV2.InstanceShutdown() called for node %s", node.Name)
	providerID, err := c.getProviderID(ctx, node)
	if err != nil {
		return false, err
	}

	return c.instances.InstanceShutdownByProviderID(ctx, providerID)
}

func (c *instancesV2) getAdditionalLabels() (map[string]string, error) {
	return AdditionalLabels, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields and labels in the Node object on registration.
// Implementations should always check node.spec.providerID first when trying to discover the instance
// for a given node. In cases where node.spec.providerID is empty, implementations can use other
// properties of the node like its name, labels and annotations.
func (c *instancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	klog.V(4).Infof("instancesV2.InstanceMetadata() called with node %s", node.Name)

	providerID, err := c.getProviderID(ctx, node)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("instancesV2.InstanceMetadata() got provider ID %s", providerID)

	instanceType, err := c.instances.InstanceTypeByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("instancesV2.InstanceMetadata() got instanceType %s", instanceType)

	zone, err := c.zones.GetZoneByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	klog.V(4).InfoS("instancesV2.InstanceMetadata() got zone info", "zone", zone)

	nodeAddresses, err := c.instances.NodeAddressesByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	klog.V(4).InfoS("instancesV2.InstanceMetadata() got nodeAddresses", "nodeAddresses", nodeAddresses)

	// Generate additionalLabels
	additionalLabels, err := c.getAdditionalLabels()
	if err != nil {
		return nil, err
	}
	klog.V(4).InfoS("instancesV2.InstanceMetadata() got additionalLabels", "additionalLabels", additionalLabels)

	return &cloudprovider.InstanceMetadata{
		ProviderID:       providerID,
		InstanceType:     instanceType,
		NodeAddresses:    nodeAddresses,
		Zone:             zone.FailureDomain,
		Region:           zone.Region,
		AdditionalLabels: additionalLabels,
	}, nil
}
