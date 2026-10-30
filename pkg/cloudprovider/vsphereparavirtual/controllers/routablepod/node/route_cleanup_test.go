/*
Copyright The Kubernetes Authors.

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
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

func TestEnqueueDeletedNodeTombstone(t *testing.T) {
	controller, _ := newController()
	defer controller.workqueue.ShutDown()
	controller.enqueueNode(cache.DeletedFinalStateUnknown{Key: n1Name, Obj: &n1})
	if controller.workqueue.Len() != 1 {
		t.Fatal("deleted Node tombstone was not queued for cleanup")
	}
	key, _ := controller.workqueue.Get()
	defer controller.workqueue.Done(key)
	if key != n1Name {
		t.Fatalf("queued %v, want %q", key, n1Name)
	}
}

type cleanupRecorder struct {
	calls      []string
	cleanupErr error
	releaseErr error
}

func (r *cleanupRecorder) DeleteNodeRoutes(_ context.Context, clusterName, nodeName string) error {
	r.calls = append(r.calls, "routes:"+clusterName+"/"+nodeName)
	return r.cleanupErr
}

func (r *cleanupRecorder) ReleasePodCIDR(node *corev1.Node) error {
	r.calls = append(r.calls, "release:"+node.Name)
	return r.releaseErr
}

func (r *cleanupRecorder) ClaimPodCIDR(node *corev1.Node) error {
	r.calls = append(r.calls, "claim:"+node.Name)
	return nil
}

func TestDeletedNodeCleansRoutesBeforeReleasingCIDR(t *testing.T) {
	controller, _ := newController()
	defer controller.workqueue.ShutDown()
	operations := &cleanupRecorder{}
	controller.routeCleaner, controller.nsxIPManager = operations, operations
	require.NoError(t, controller.syncNode(n1Name))
	require.Equal(t, []string{"routes:" + testClusterName + "/" + n1Name, "release:" + n1Name}, operations.calls)
}

func TestDeletedNodeRetriesCleanupFailure(t *testing.T) {
	for _, stage := range []string{"routes", "allocation"} {
		t.Run(stage, func(t *testing.T) {
			controller, _ := newController()
			defer controller.workqueue.ShutDown()
			operations := &cleanupRecorder{}
			if stage == "routes" {
				operations.cleanupErr = fmt.Errorf("route deletion unavailable")
			} else {
				operations.releaseErr = fmt.Errorf("allocation deletion unavailable")
			}
			controller.routeCleaner, controller.nsxIPManager = operations, operations
			controller.workqueue.Add(n1Name)
			require.True(t, controller.processNextWorkItem())
			require.Equal(t, 1, controller.workqueue.NumRequeues(n1Name))
			if stage == "routes" {
				require.Equal(t, []string{"routes:" + testClusterName + "/" + n1Name}, operations.calls)
			}
			operations.cleanupErr, operations.releaseErr = nil, nil
			operations.calls = nil
			require.NoError(t, controller.syncNode(n1Name))
			require.Equal(t, []string{"routes:" + testClusterName + "/" + n1Name, "release:" + n1Name}, operations.calls)
		})
	}
}

func TestExistingNodeDoesNotCleanRoutes(t *testing.T) {
	controller, _ := newController()
	defer controller.workqueue.ShutDown()
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
	require.NoError(t, indexer.Add(n1.DeepCopy()))
	controller.nodesLister = corelisters.NewNodeLister(indexer)
	operations := &cleanupRecorder{}
	controller.routeCleaner, controller.nsxIPManager = operations, operations
	require.NoError(t, controller.syncNode(n1Name))
	require.Equal(t, []string{"claim:" + n1Name}, operations.calls)
}

func TestDeletedNodeWithoutRouteCleanerRetainsAllocationCleanup(t *testing.T) {
	controller, _ := newController()
	defer controller.workqueue.ShutDown()
	operations := &cleanupRecorder{}
	controller.nsxIPManager = operations
	require.NoError(t, controller.syncNode(n1Name))
	require.Equal(t, []string{"release:" + n1Name}, operations.calls)
}
