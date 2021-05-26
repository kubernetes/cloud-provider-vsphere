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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ippoolclientset "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	ippoolscheme "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/scheme"
	ippoolinformers "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/informers/externalversions"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/ippool"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/node"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

const (
	defaultResyncTime time.Duration = time.Minute * 1
)

// StartControllers starts ippool_controller and node_controller
func StartControllers(scCfg *rest.Config, client kubernetes.Interface, informerManager *k8s.InformerManager, clusterName, clusterNS string, ownerRef *metav1.OwnerReference) error {
	if clusterName == "" {
		return fmt.Errorf("cluster name can't be empty")
	}
	if clusterNS == "" {
		return fmt.Errorf("cluster namespace can't be empty")
	}
	ipcs, err := ippoolclientset.NewForConfig(scCfg)
	if err != nil {
		return fmt.Errorf("error building ippool clientset: %w", err)
	}

	s := scheme.Scheme
	if err := ippoolscheme.AddToScheme(s); err != nil {
		return fmt.Errorf("failed to register ippoolSchemes")
	}

	ippoolInformerFactory := ippoolinformers.NewSharedInformerFactoryWithOptions(ipcs, defaultResyncTime, ippoolinformers.WithNamespace(clusterNS))
	ippoolInformer := ippoolInformerFactory.Nsx().V1alpha1().IPPools()

	ippoolController := ippool.NewController(client, ipcs, ippoolInformer)
	go ippoolController.Run(context.Background().Done())

	ippoolInformerFactory.Start(wait.NeverStop)

	nodeController := node.NewController(client, ipcs, informerManager, clusterName, clusterNS, ownerRef)
	go nodeController.Run(context.Background().Done())

	return nil
}
