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

package vsphere

import (
	"context"
	"errors"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudprovider"

	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
)

// Error constants
var (
	// ErrNotFound is returned by NodeAddresses, NodeAddressesByProviderID,
	// and InstanceID when a node cannot be found.
	ErrNodeNotFound = errors.New("Node not found")
)

func newInstances(nodeManager *NodeManager) cloudprovider.Instances {
	return &instances{nodeManager}
}

// NodeAddresses returns all the valid addresses of the instance identified by
// nodeName. Only the public/private IPv4 addresses are considered for now.
//
// When nodeName identifies more than one instance, only the first will be
// considered.
func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddresses() called with ", string(nodeName))

	// Check if node has been discovered already
	if node, ok := i.nodeManager.nodeNameMap[string(nodeName)]; ok {
		klog.V(2).Info("instances.NodeAddresses() CACHED with ", string(nodeName))
		return node.NodeAddresses, nil
	}

	if err := i.nodeManager.DiscoverNode(string(nodeName), cm.FindVMByName); err == nil {
		if i.nodeManager.nodeNameMap[string(nodeName)] == nil {
			klog.Errorf("DiscoverNode succeeded, but CACHE missed for node=%s. If this is a Linux VM, hostnames are case sensitive. Make sure they match.", string(nodeName))
			return []v1.NodeAddress{}, ErrNodeNotFound
		}
		klog.V(2).Info("instances.NodeAddresses() FOUND with ", string(nodeName))
		return i.nodeManager.nodeNameMap[string(nodeName)].NodeAddresses, nil
	}

	klog.V(4).Info("instances.NodeAddresses() NOT FOUND with ", string(nodeName))
	return []v1.NodeAddress{}, ErrNodeNotFound
}

// NodeAddressesByProviderID returns all the valid addresses of the instance
// identified by providerID. Only the public/private IPv4 addresses will be
// considered for now.
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	klog.V(4).Info("instances.NodeAddressesByProviderID() called with ", providerID)

	// Check if node has been discovered already
	uid := GetUUIDFromProviderID(providerID)
	if node, ok := i.nodeManager.nodeUUIDMap[uid]; ok {
		klog.V(2).Info("instances.NodeAddressesByProviderID() CACHED with ", uid)
		return node.NodeAddresses, nil
	}

	if err := i.nodeManager.DiscoverNode(uid, cm.FindVMByUUID); err == nil {
		klog.V(2).Info("instances.NodeAddressesByProviderID() FOUND with ", uid)
		return i.nodeManager.nodeUUIDMap[uid].NodeAddresses, nil
	}

	klog.V(4).Info("instances.NodeAddressesByProviderID() NOT FOUND with ", uid)
	return []v1.NodeAddress{}, ErrNodeNotFound
}

// ExternalID returns the cloud provider ID of the instance identified by
// nodeName. If the instance does not exist or is no longer running, the
// returned error will be cloudprovider.InstanceNotFound.
//
// When nodeName identifies more than one instance, only the first will be
// considered.
func (i *instances) ExternalID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.V(4).Info("instances.ExternalID() called with ", nodeName)
	return i.InstanceID(ctx, nodeName)
}

// InstanceID returns the cloud provider ID of the instance identified by nodeName.
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	klog.V(4).Info("instances.InstanceID() called with ", nodeName)

	// Check if node has been discovered already
	if node, ok := i.nodeManager.nodeNameMap[string(nodeName)]; ok {
		klog.V(2).Info("instances.InstanceID() CACHED with ", string(nodeName))
		return node.UUID, nil
	}

	if err := i.nodeManager.DiscoverNode(string(nodeName), cm.FindVMByName); err == nil {
		if i.nodeManager.nodeNameMap[string(nodeName)] == nil {
			klog.Errorf("DiscoverNode succeeded, but CACHE missed for node=%s. If this is a Linux VM, hostnames are case sensitive. Make sure they match.", string(nodeName))
			return "", ErrNodeNotFound
		}
		klog.V(2).Infof("instances.InstanceID() FOUND with %s", string(nodeName))
		return i.nodeManager.nodeNameMap[string(nodeName)].UUID, nil
	}

	klog.V(4).Info("instances.InstanceID() NOT FOUND with ", string(nodeName))
	return "", ErrNodeNotFound
}

// InstanceType returns the type of the instance identified by name.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	klog.V(4).Info("instances.InstanceType() called")
	return i.nodeManager.nodeNameMap[string(name)].NodeType, nil
}

// InstanceTypeByProviderID returns the type of the instance identified by providerID.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	klog.V(4).Info("instances.InstanceTypeByProviderID() called")
	uid := GetUUIDFromProviderID(providerID)
	return i.nodeManager.nodeUUIDMap[uid].NodeType, nil
}

// AddSSHKeyToAllInstances is not implemented; it always returns an error.
func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	klog.V(4).Info("instances.AddSSHKeyToAllInstances() called")
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns hostname as a NodeName value.
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	klog.V(4).Info("instances.CurrentNodeName() called")
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance identified by
// providerID is running.
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceExistsByProviderID() called with ", providerID)

	// Check if node has been discovered already
	uid := GetUUIDFromProviderID(providerID)
	if _, ok := i.nodeManager.nodeUUIDMap[uid]; ok {
		klog.V(2).Info("instances.InstanceExistsByProviderID() CACHED with ", uid)
		return true, nil
	}

	if err := i.nodeManager.DiscoverNode(uid, cm.FindVMByUUID); err == nil {
		klog.V(2).Info("instances.InstanceExistsByProviderID() EXISTS with ", uid)
		return true, err
	}

	klog.V(4).Info("instances.InstanceExistsByProviderID() NOT FOUND with ", uid)
	return false, nil
}

// InstanceShutdownByProviderID returns true if the instance is in safe state to detach volumes
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	klog.V(4).Info("instances.InstanceShutdownByProviderID() called")
	return false, cloudprovider.NotImplemented
}
