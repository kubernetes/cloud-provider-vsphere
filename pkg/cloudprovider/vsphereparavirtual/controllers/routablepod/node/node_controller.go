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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/nsxipmanager"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
)

const (
	controllerName = "node-controller"
)

// Controller adds or removes node's CIDR allocation request from ippool spec
// whenever a node is added/updated/removed.
// Create a ippool if there isn't one for current cluster.
type Controller struct {
	nsxIPManager nsxipmanager.NSXIPManager

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
	nsxIPManager nsxipmanager.NSXIPManager,
	informerManager *k8s.InformerManager,
	clusterName string,
	clusterNS string,
	ownerRef *metav1.OwnerReference) *Controller {

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events(clusterNS)})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerName})

	c := &Controller{
		nsxIPManager:     nsxIPManager,
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
			// no need to add request if it's already allocated
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
		// node absence in store means watcher caught the deletion, ensure Pod CIDR of this Node is released
		klog.V(4).Infof("Node %s is not found, releasing its Pod CIDR", name)
		err = c.nsxIPManager.ReleasePodCIDR(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}})
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("unable to retrieve node %v from store: %v", name, err))
	default:
		// node exists in store, ensure Pod CIDR of this Node is claimed
		klog.V(4).Infof("Node %s is found, ensuring Pod CIDR claimed", name)
		err = c.nsxIPManager.ClaimPodCIDR(node)
	}

	return err
}
