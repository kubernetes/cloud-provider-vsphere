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

package ippool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ippoolv1alpha1 "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	ippoolclientset "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned"
	ippoolscheme "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/clientset/versioned/scheme"
	ippoolinformers "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/informers/externalversions/nsxnetworking/v1alpha1"
	ippoollisters "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/client/listers/nsxnetworking/v1alpha1"
	klog "k8s.io/klog/v2"
)

const (
	controllerName    = "ippool-controller"
	cidrUpdateRetries = 3
	// Interval of synchronizing ippool status from apiserver
	ippoolSyncPeriod = 30 * time.Second
)

// Controller update node's podCIDR whenever ippool's status get updated CIDR allocation result
type Controller struct {
	kubeclientset      kubernetes.Interface
	ippoolclientset    ippoolclientset.Interface
	ippoolLister       ippoollisters.IPPoolLister
	ippoolListerSynced cache.InformerSynced

	recorder  record.EventRecorder
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a Controller that reconciles ippool
func NewController(
	kubeClient kubernetes.Interface,
	ippoolclientset ippoolclientset.Interface,
	ippoolInformer ippoolinformers.IPPoolInformer) *Controller {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(ippoolscheme.Scheme, corev1.EventSource{Component: controllerName})

	c := &Controller{
		kubeclientset:      kubeClient,
		ippoolclientset:    ippoolclientset,
		ippoolLister:       ippoolInformer.Lister(),
		ippoolListerSynced: ippoolInformer.Informer().HasSynced,

		recorder:  recorder,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "IPPools"),
	}

	// watch ippool change
	ippoolInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueIPPool,
			UpdateFunc: func(old, cur interface{}) {
				oldIPPool, ok := old.(*ippoolv1alpha1.IPPool)
				if !ok {
					return
				}
				curIPPool, ok := cur.(*ippoolv1alpha1.IPPool)
				if !ok {
					return
				}
				if !shouldSyncIPPool(oldIPPool, curIPPool) {
					return
				}
				c.enqueueIPPool(cur)
			},
			// skip delete since network provider operator will clean up subnets
		},
		ippoolSyncPeriod,
	)

	return c
}

// if allocated subnets are updated, then need to update nodes with new subnets
func shouldSyncIPPool(old, cur *ippoolv1alpha1.IPPool) bool {
	return !reflect.DeepEqual(old.Status.Subnets, cur.Status.Subnets)
}

func (c *Controller) enqueueIPPool(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// Run starts the worker to process ippool updates
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.V(4).Info("Waiting cache to be synced.")

	if !cache.WaitForNamedCacheSync("ippool", stopCh, c.ippoolListerSynced) {
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

	// We wrap this block in a func so we can defer c.workqueue.Done.
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
		// IPPool resource to be synced.
		if err := c.syncIPPool(key); err != nil {
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

// syncIPPool will sync the IPPool with the given key if it has had its expectations fulfilled,
// meaning it did not expect to see any more of its pods created or deleted. This function is not meant to be
// invoked concurrently with the same key.
func (c *Controller) syncIPPool(key string) error {
	startTime := time.Now()
	defer func() {
		klog.V(4).Infof("Finished syncing service %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	ippool, err := c.ippoolLister.IPPools(namespace).Get(name)
	switch {
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("unable to retrieve service %v from store: %v", key, err))
	default:
		err = c.processIPPoolCreateOrUpdate(ippool)
	}

	return err
}

func (c *Controller) processIPPoolCreateOrUpdate(ippool *ippoolv1alpha1.IPPool) error {
	ctx := context.Background()
	// make map of allocated subnets
	subs := make(map[string]string)
	for _, sub := range ippool.Status.Subnets {
		// if ippool is exhausted, the cidr value could be empty
		if sub.CIDR != "" {
			subs[sub.Name] = sub.CIDR
		}
	}
	nodes, err := c.kubeclientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	// update node with allocated subnet
	for _, n := range nodes.Items {
		if v, ok := subs[n.Name]; ok {
			if n.Spec.PodCIDR == "" || len(n.Spec.PodCIDRs) == 0 {
				// If we reached here, it means that the node has no CIDR currently assigned. So we set it.
				if err := c.patchNodeCIDRWithRetry(types.NodeName(n.Name), v); err == nil {
					// continue to next node if this one succeeded
					continue
				}
				klog.Errorf("Failed to update node %v PodCIDR to %v after multiple attempts: %v", n.Name, v, err)
				c.recordNodeStatusChange(&n, "CIDRAssignmentFailed")
				klog.Errorf("CIDR assignment for node %v failed: %v. Try again in next reconcile", n.Name, err)

				return err
			}
		}
	}

	return nil
}

type nodeForCIDRMergePatch struct {
	Spec nodeSpecForMergePatch `json:"spec"`
}

type nodeSpecForMergePatch struct {
	PodCIDR  string   `json:"podCIDR"`
	PodCIDRs []string `json:"podCIDRs,omitempty"`
}

// patchNodeCIDRWithRetry patches the specified node's CIDR to the given value with retries
func (c *Controller) patchNodeCIDRWithRetry(node types.NodeName, cidr string) error {
	var err error
	for i := 0; i < cidrUpdateRetries; i++ {
		if err = c.patchNodeCIDR(node, cidr); err == nil {
			klog.V(4).Info("Set node %v PodCIDR to %v", node, cidr)
			return nil
		}
	}
	return err
}

// patchNodeCIDR patches the specified node's CIDR to the given value.
func (c *Controller) patchNodeCIDR(node types.NodeName, cidr string) error {
	patch := nodeForCIDRMergePatch{
		Spec: nodeSpecForMergePatch{
			PodCIDR:  cidr,
			PodCIDRs: []string{cidr},
		},
	}
	patchBytes, err := json.Marshal(&patch)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal CIDR: %v", err)
	}

	if _, err := c.kubeclientset.CoreV1().Nodes().Patch(context.TODO(), string(node), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("failed to patch node CIDR: %v", err)
	}
	return nil
}

// recordNodeStatusChange records a event related to a node status change. (Common to lifecycle and ipam)
func (c *Controller) recordNodeStatusChange(node *corev1.Node, newStatus string) {
	ref := &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
		Namespace:  "",
	}
	klog.V(2).Infof("Recording status change %s event message for node %s", newStatus, node.Name)
	c.recorder.Eventf(ref, corev1.EventTypeNormal, newStatus, "Node %s status is now: %s", node.Name, newStatus)
}
