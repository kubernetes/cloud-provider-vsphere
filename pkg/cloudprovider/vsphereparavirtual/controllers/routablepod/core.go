/*
Copyright 2021 The Kubernetes Authors.
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

package routablepod

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/ippool"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/node"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

// StartControllers starts ippool_controller and node_controller
func StartControllers(scCfg *rest.Config, client kubernetes.Interface,
	informerManager *k8s.InformerManager, clusterName, clusterNS string, ownerRef *metav1.OwnerReference,
	vpcModeEnabled bool, podIPPoolType string) error {

	if clusterName == "" {
		return fmt.Errorf("cluster name can't be empty")
	}
	if clusterNS == "" {
		return fmt.Errorf("cluster namespace can't be empty")
	}

	klog.V(2).Info("Routable pod controllers start with VPC mode enabled: ", vpcModeEnabled)

	ippManager, err := ippoolmanager.GetIPPoolManager(vpcModeEnabled, scCfg, clusterNS, podIPPoolType)
	if err != nil {
		return fmt.Errorf("fail to get ippool manager or start ippool controller: %w", err)
	}

	ippoolController := ippool.NewController(client, ippManager)
	go ippoolController.Run(context.Background().Done())

	ippManager.StartIPPoolInformers()

	nodeController := node.NewController(client, ippManager, informerManager, clusterName, clusterNS, ownerRef)
	go nodeController.Run(context.Background().Done())

	return nil
}
