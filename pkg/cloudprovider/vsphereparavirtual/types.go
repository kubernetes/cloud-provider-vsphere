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

package vsphereparavirtual

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	cpcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

// VSphereParavirtual is an implementation of cloud provider Interface for vsphere paravirtual.
type VSphereParavirtual struct {
	cfg            *cpcfg.Config
	ownerReference *metav1.OwnerReference
	client         clientset.Interface
	informMgr      *k8s.InformerManager
	loadBalancer   cloudprovider.LoadBalancer
	instances      cloudprovider.Instances
	routes         RoutesProvider
}
