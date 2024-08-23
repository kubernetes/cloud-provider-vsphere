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

package ipaddressallocation

import (
	"context"
	"fmt"
	"time"

	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	vpcinformerv1 "github.com/vmware-tanzu/nsx-operator/pkg/client/informers/externalversions/vpc/v1alpha1"
	vpclisterv1alpha1 "github.com/vmware-tanzu/nsx-operator/pkg/client/listers/vpc/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listerv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/controllers/routablepod/utils"
)

const (
	// controllerAgentName is the string used by this controller to identify
	controllerAgentName = "ipaddressallocation-controller"
	// syncPeriod Interval of synchronizing IPAddressAllocation from apiserver
	syncPeriod = 30 * time.Second
)

// Controller update node's podCIDR whenever ipaddressallocation's status get updated CIDR allocation result
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	nodesLister                listerv1.NodeLister
	nodesSynced                cache.InformerSynced
	ipAddressAllocationsLister vpclisterv1alpha1.IPAddressAllocationLister
	ipAddressAllocationsSynced cache.InformerSynced

	recorder  record.EventRecorder
	workqueue workqueue.RateLimitingInterface
}

// NewController returns a Controller that reconciles IPAddressAllocation
func NewController(
	ctx context.Context,
	kubeclientset kubernetes.Interface,
	nodesLister listerv1.NodeLister,
	nodesSynced cache.InformerSynced,
	ipAddressAllocationInformer vpcinformerv1.IPAddressAllocationInformer) *Controller {
	logger := klog.FromContext(ctx)

	// Create event broadcaster
	utilruntime.Must(vpcapisv1.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	c := &Controller{
		kubeclientset:              kubeclientset,
		nodesLister:                nodesLister,
		nodesSynced:                nodesSynced,
		ipAddressAllocationsLister: ipAddressAllocationInformer.Lister(),
		ipAddressAllocationsSynced: ipAddressAllocationInformer.Informer().HasSynced,
		recorder:                   recorder,
		workqueue: workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{
			Name: "IPAddressAllocations",
		}),
	}

	logger.Info("Setting up event handlers")
	ipAddressAllocationInformer.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueueIPAddressAllocation,
			UpdateFunc: func(old, cur interface{}) {
				c.enqueueIPAddressAllocation(cur)
			},
		},
		syncPeriod,
	)

	return c
}

// enqueueIPAddressAllocation takes a IPAddressAllocation resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than IPAddressAllocation.
func (c *Controller) enqueueIPAddressAllocation(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting IPAddressAllocation controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.nodesSynced, c.ipAddressAllocationsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch workers to process IPAddressAllocation resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// IPAddressAllocation resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		logger.Info("Successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler will sync the IPAddressAllocation with the given key if it has had its expectations fulfilled.
// This function is not meant to be invoked concurrently with the same key.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	startTime := time.Now()
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "IPAddressAllocation ", key)
	ctx = klog.NewContext(ctx, logger)
	defer func() {
		logger.V(4).Info(fmt.Sprintf("Reconciliation took (%v)", time.Since(startTime)))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the IPAddressAllocation resource with this namespace/name
	ipAddressAllocation, err := c.ipAddressAllocationsLister.IPAddressAllocations(namespace).Get(name)
	if err != nil {
		// The IPAddressAllocation resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("IPAddressAllocation '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	if !ipAddressAllocation.DeletionTimestamp.IsZero() {
		logger.V(4).Info(fmt.Sprintf("IPAddressAllocation %s/%s is being deleted, skip", ipAddressAllocation.Namespace, ipAddressAllocation.Name))
		return nil
	}

	return c.processIPAddressAllocationCreateOrUpdate(ctx, ipAddressAllocation)
}

// processIPAddressAllocationCreateOrUpdate will get CIDR from the IPAddressAllocation status and update it to the
// PodCIDR of the same name Node.
func (c *Controller) processIPAddressAllocationCreateOrUpdate(ctx context.Context, ipAddressAllocation *vpcapisv1.IPAddressAllocation) error {

	var podCIDR string
	for _, condition := range ipAddressAllocation.Status.Conditions {
		if condition.Type == vpcapisv1.Ready {
			if condition.Status != corev1.ConditionTrue {
				return fmt.Errorf("IPAddressAllocation %v is not ready", ipAddressAllocation.Name)
			}
			podCIDR = ipAddressAllocation.Status.CIDR
		}
	}
	if podCIDR == "" {
		return fmt.Errorf("IPAddressAllocation %v does not get CIDR allocated", ipAddressAllocation.Name)
	}
	node, err := c.nodesLister.Get(ipAddressAllocation.Name)
	if err != nil {
		return err
	}

	// update node with allocated podCIDR
	return utils.PatchNodeCIDRWithRetry(ctx, c.kubeclientset, node, podCIDR, c.recorder)
}
