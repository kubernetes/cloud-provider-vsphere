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

package vsphere

import (
	"context"
	"strings"
	"testing"

	"github.com/vmware/govmomi/simulator"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientv1 "k8s.io/client-go/listers/core/v1"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	v1helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
)

type MyNodeManager struct {
	NodeManager
}

func newMyNodeManager(cm *cm.ConnectionManager, lister clientv1.NodeLister) *MyNodeManager {
	return &MyNodeManager{*newNodeManager(cm, lister)}
}

// Used to populate the networking info
func (nm *MyNodeManager) RegisterNode(node *v1.Node) {
	nm.NodeManager.RegisterNode(node)

	myNode1 := nm.nodeNameMap[node.Name]
	myNode2 := nm.nodeUUIDMap[ConvertK8sUUIDtoNormal(node.Status.NodeInfo.SystemUUID)]

	addrs := []v1.NodeAddress{}
	v1helper.AddToNodeAddresses(&addrs,
		v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: "127.0.0.1",
		}, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: "127.0.0.1",
		}, v1.NodeAddress{
			Type:    v1.NodeHostName,
			Address: node.Name,
		},
	)

	myNode1.NodeAddresses = addrs
	myNode2.NodeAddresses = addrs
}

func TestInstance(t *testing.T) {
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	//context
	ctx := context.Background()

	/*
	 * Setup
	 */
	connMgr := cm.NewConnectionManager(cfg, nil)
	nm := newMyNodeManager(connMgr, nil)
	instances := newInstances(&nm.NodeManager)

	vm := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine)
	name := vm.Name
	vm.Guest.HostName = name
	UUID := strings.ToUpper(vm.Config.Uuid)
	k8sUUID := ConvertK8sUUIDtoNormal(UUID)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: k8sUUID,
			},
		},
	}

	nm.RegisterNode(node)

	providerID := ProviderPrefix + UUID
	/*
	 * Setup
	 */

	addrs, err := instances.NodeAddresses(ctx, types.NodeName(name))
	if err != nil {
		t.Errorf("NodeAddresses failed err=%v", err)
	}
	if len(addrs) != 3 {
		t.Errorf("NodeAddresses mismatch should be 3 addrs count=%d", len(addrs))
	}

	addrs, err = instances.NodeAddressesByProviderID(ctx, providerID)
	if err != nil {
		t.Errorf("NodeAddressesByProviderID failed err=%v", err)
	}
	if len(addrs) != 3 {
		t.Errorf("NodeAddressesByProviderID mismatch should be 3 addrs count=%d", len(addrs))
	}

	myUUID, err := instances.InstanceID(ctx, types.NodeName(name))
	if err != nil {
		t.Errorf("InstanceID failed err=%v", err)
	}
	if !strings.EqualFold(myUUID, UUID) {
		t.Errorf("InstanceID mismatch %s != %s", myUUID, UUID)
	}

	exists, err := instances.InstanceExistsByProviderID(ctx, providerID)
	if err != nil {
		t.Errorf("InstanceExistsByProviderID failed err=%v", err)
	}
	if !exists {
		t.Error("InstanceExistsByProviderID not found")
	}

	ishut, err := instances.InstanceShutdownByProviderID(ctx, providerID)
	if err != nil {
		t.Errorf("InstanceShutdownByProviderID failed err=%v", err)
	}
	if ishut {
		t.Error("InstanceShutdownByProviderID is shutdown")
	}

}

func TestInvalidInstance(t *testing.T) {
	cfg, ok := configFromEnvOrSim(true)
	defer ok()

	//context
	ctx := context.Background()

	/*
	 * Setup
	 */
	connMgr := cm.NewConnectionManager(cfg, nil)
	nm := newMyNodeManager(connMgr, nil)
	instances := newInstances(&nm.NodeManager)

	name := ""       //junk name
	UUID := ""       //junk UUID
	providerID := "" //junk providerid
	/*
	 * Setup
	 */

	addrs, err := instances.NodeAddresses(ctx, types.NodeName(name))
	if err == nil {
		t.Error("NodeAddresses expected failure but err=nil")
	}
	if len(addrs) != 0 {
		t.Errorf("NodeAddresses mismatch should be 0 addrs count=%d", len(addrs))
	}

	addrs, err = instances.NodeAddressesByProviderID(ctx, providerID)
	if err == nil {
		t.Error("NodeAddressesByProviderID expected failure but err=nil")
	}
	if len(addrs) != 0 {
		t.Errorf("NodeAddressesByProviderID mismatch should be 0 addrs count=%d", len(addrs))
	}

	myUUID, err := instances.InstanceID(ctx, types.NodeName(name))
	if err == nil {
		t.Errorf("InstanceID expected failure but err=nil")
	}
	if !strings.EqualFold(myUUID, UUID) {
		t.Errorf("InstanceID mismatch %s != %s", myUUID, UUID)
	}

	exists, err := instances.InstanceExistsByProviderID(ctx, providerID)
	if err != nil {
		t.Errorf("InstanceExistsByProviderID failed err=%v", err)
	}
	if exists {
		t.Error("InstanceExistsByProviderID excepted not exists")
	}
}
