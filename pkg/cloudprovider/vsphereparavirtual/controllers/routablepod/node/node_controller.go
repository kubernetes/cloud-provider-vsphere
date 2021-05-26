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

package node

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ippoolv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	ippoolclientset "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	ippoolscheme "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/scheme"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/helper"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
	klog "k8s.io/klog/v2"
)

const (
	controllerName = "node-controller"
)

// Controller adds or removes node's CIDR allocation request from ippool spec
// whenever a node is added/updated/removed.
// Create a ippool if there isn't one for current cluster.
type Controller struct {
	ippoolclientset ippoolclientset.Interface

	nodesLister      corelisters.NodeLister
	nodeListerSynced cache.InformerSynced

	recorder  record.EventRecorder
	workqueue workqueue.RateLimitingInterface

	clusterName string
	clusterNS   string

	ownerRef *metav1.OwnerReference
}

// NewController returns controller that reconciles node
func NewController(
	kubeClient kubernetes.Interface,
	ippoolclientset ippoolclientset.Interface,
	informerManager *k8s.InformerManager,
	clusterName string,
	clusterNS string,
	ownerRef *metav1.OwnerReference) *Controller {

	utilruntime.Must(ippoolv1alpha1.AddToScheme(ippoolscheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events(clusterNS)})
	recorder := eventBroadcaster.NewRecorder(ippoolscheme.Scheme, corev1.EventSource{Component: controllerName})

	c := &Controller{
		ippoolclientset:  ippoolclientset,
		nodesLister:      informerManager.GetNodeLister(),
		nodeListerSynced: informerManager.IsNodeInformerSynced(),

		recorder:  recorder,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Nodes"),

		clusterName: clusterName,
		clusterNS:   clusterNS,

		ownerRef: ownerRef,
	}

	// watch node change
	informerManager.AddNodeListener(
		// add
		func(cur interface{}) {
			node := cur.(*corev1.Node).DeepCopy()
			c.enqueueNode(node)
		},
		// remove
		func(old interface{}) {
			c.enqueueNode(old)
		},
		// update
		func(_, cur interface{}) {
			node := cur.(*corev1.Node).DeepCopy()
			// assume there is no subnet change after the allocation
			if len(node.Spec.PodCIDR) == 0 || len(node.Spec.PodCIDRs) == 0 {
				c.enqueueNode(node)
			}
		})
	return c
}

func (c *Controller) enqueueNode(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// Run starts the worker to process node updates
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.V(4).Info("Waiting cache to be synced.")

	if !cache.WaitForNamedCacheSync("node", stopCh, c.nodeListerSynced) {
		return
	}

	klog.V(4).Info("Starting node workers.")
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer nc.workqueue.Done.
	err := func(obj interface{}) error {
		defer c.workqueue.Done(obj)

		var key string
		var ok bool
		if key, ok = obj.(string); !ok {
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		// Run the syncHandler, passing it the key of the
		// Node resource to be synced.
		if err := c.syncNode(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncNode will sync the Node with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its pods created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (c *Controller) syncNode(key string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	node, err := c.nodesLister.Get(name)
	switch {
	case apierrors.IsNotFound(err):
		// node absence in store means watcher caught the deletion, ensure the request in ippool is deleted info is cleaned
		err = c.processNodeDelete(name)
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("unable to retrieve node %v from store: %v", key, err))
	default:
		err = c.processNodeCreateOrUpdate(node)
	}

	return err
}

// remove the node subnet allocation request from ippool's spec
// if ippool is not found, skip the removing
func (c *Controller) processNodeDelete(name string) error {
	ctx := context.Background()

	ippool, err := c.ippoolclientset.NsxV1alpha1().IPPools(c.clusterNS).Get(ctx, helper.IppoolNameFromClusterName(c.clusterName), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(4).Info("ippool is gone, no need to remove the node request")
			return nil
		}
		return err
	}

	newSubnets := []ippoolv1alpha1.SubnetRequest{}
	for _, sub := range ippool.Spec.Subnets {
		if sub.Name == name {
			continue
		}
		newSubnets = append(newSubnets, sub)
	}
	ippool.Spec.Subnets = newSubnets

	_, err = c.ippoolclientset.NsxV1alpha1().IPPools(c.clusterNS).Update(context.TODO(), ippool, metav1.UpdateOptions{})

	return err
}

// when a node is created or updated, check if the node has podCIDR field set.
// if node's podCIDR is empty, add the node CIDR allocation request to ippool spec.
func (c *Controller) processNodeCreateOrUpdate(node *corev1.Node) error {
	ctx := context.Background()
	ippool, err := c.ippoolclientset.NsxV1alpha1().IPPools(c.clusterNS).Get(ctx, helper.IppoolNameFromClusterName(c.clusterName), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// if ippool does not exist, create one
		klog.V(4).Info("creating ippool")
		if ippool, err = c.createIPPool(); err != nil {
			klog.Error("error creating ippool")
			return err
		}
	}

	// skip if the request already added
	for _, sub := range ippool.Spec.Subnets {
		if sub.Name == node.Name {
			klog.V(4).Info("node %s already requested the ip", node.Name)
			return nil
		}
	}

	klog.V(4).Info("updating CIDR request in IPPool %s/%s for node %s", ippool.Namespace, ippool.Name, node.Name)

	newIPPool := ippool.DeepCopy()
	// add node cidr allocation req to the ippool spec only when node doesn't contain pod cidr
	if node.Spec.PodCIDR == "" || len(node.Spec.PodCIDRs) == 0 {
		newIPPool.Spec.Subnets = append(newIPPool.Spec.Subnets, ippoolv1alpha1.SubnetRequest{
			Name:         node.Name,
			IPFamily:     helper.IPFamilyDefault,
			PrefixLength: helper.PrefixLengthDefault,
		})
	}

	if newIPPool.OwnerReferences == nil {
		newIPPool.OwnerReferences = []metav1.OwnerReference{*c.ownerRef}
	}

	_, err = c.ippoolclientset.NsxV1alpha1().IPPools(c.clusterNS).Update(context.Background(), newIPPool, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	klog.V(4).Info("updated CIDR in IPPool %s/%s for node %s", ippool.Namespace, ippool.Name, node.Name)
	return nil
}

func (c *Controller) createIPPool() (*ippoolv1alpha1.IPPool, error) {
	ippool := &ippoolv1alpha1.IPPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helper.IppoolNameFromClusterName(c.clusterName),
			Namespace: c.clusterNS,
			OwnerReferences: []metav1.OwnerReference{
				*c.ownerRef,
			},
		},
		Spec: ippoolv1alpha1.IPPoolSpec{
			Subnets: []ippoolv1alpha1.SubnetRequest{},
		},
	}

	return c.ippoolclientset.NsxV1alpha1().IPPools(c.clusterNS).Create(context.Background(), ippool, metav1.CreateOptions{})
}
