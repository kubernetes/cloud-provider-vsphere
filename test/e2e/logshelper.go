/*
Copyright 2020 The Kubernetes Authors.

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

package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	toolscache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retryableOperationInterval = 3 * time.Second
	// retryableOperationTimeout requires a higher value especially for self-hosted upgrades.
	// Short unavailability of the Kube APIServer due to joining etcd members paired with unreachable conversion webhooks due to
	// failed leader election and thus controller restarts lead to longer taking retries.
	// The timeout occurs when listing machines in `GetControlPlaneMachinesByCluster`.
	retryableOperationTimeout = 3 * time.Minute
)

type WatchDaemonSetLogsByLabelSelectorInput struct {
	GetLister framework.GetLister
	Cache     toolscache.Cache
	ClientSet *kubernetes.Clientset
	Labels    map[string]string
	LogPath   string
}

// WatchDaemonSetLogsByLabelSelector streams logs for all containers for all pods belonging to a daemonset on the basis of label. Each container's logs are streamed
// in a separate goroutine so they can all be streamed concurrently. This only causes a test failure if there are errors
// retrieving the daemonset, its pods, or setting up a log file. If there is an error with the log streaming itself,
// that does not cause the test to fail.
func WatchDaemonSetLogsByLabelSelector(ctx context.Context, input WatchDaemonSetLogsByLabelSelectorInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WatchDaemonSetLogsByLabelSelector")
	Expect(input.Cache).NotTo(BeNil(), "input.Cache is required for WatchDaemonSetLogsByLabelSelector")
	Expect(input.ClientSet).NotTo(BeNil(), "input.ClientSet is required for WatchDaemonSetLogsByLabelSelector")
	Expect(input.Labels).NotTo(BeNil(), "input.Selector is required for WatchDaemonSetLogsByLabelSelector")

	daemonSetList := &appsv1.DaemonSetList{}
	Eventually(func() error {
		return input.GetLister.List(ctx, daemonSetList, client.MatchingLabels(input.Labels))
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to get daemonsets for labels")

	for _, daemonSet := range daemonSetList.Items {
		watchPodLogs(ctx, watchPodLogsInput{
			Cache:          input.Cache,
			ClientSet:      input.ClientSet,
			Namespace:      daemonSet.Namespace,
			DeploymentName: daemonSet.Name,
			LabelSelector:  daemonSet.Spec.Selector,
			LogPath:        input.LogPath,
		})
	}
}

// watchPodLogsInput is the input for watchPodLogs.
type watchPodLogsInput struct {
	Cache          toolscache.Cache
	ClientSet      *kubernetes.Clientset
	Namespace      string
	DeploymentName string
	LabelSelector  *metav1.LabelSelector
	LogPath        string
}

// watchPodLogs streams logs for all containers for all pods belonging to a deployment with the given label. Each container's logs are streamed
// in a separate goroutine so they can all be streamed concurrently. This only causes a test failure if there are errors
// retrieving the deployment, its pods, or setting up a log file. If there is an error with the log streaming itself,
// that does not cause the test to fail.
func watchPodLogs(ctx context.Context, input watchPodLogsInput) {
	// Create informer to watch for pods matching input.

	podInformer, err := input.Cache.GetInformer(ctx, &corev1.Pod{})
	Expect(err).ToNot(HaveOccurred(), "Failed to create controller-runtime informer from cache")

	selector, err := metav1.LabelSelectorAsSelector(input.LabelSelector)
	Expect(err).ToNot(HaveOccurred())

	eventHandler := newWatchPodLogsEventHandler(ctx, input, selector)

	handlerRegistration, err := podInformer.AddEventHandler(eventHandler)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		<-ctx.Done()
		Expect(podInformer.RemoveEventHandler(handlerRegistration)).To(Succeed())
	}()
}

type watchPodLogsEventHandler struct {
	//nolint:containedctx
	ctx         context.Context
	input       watchPodLogsInput
	selector    labels.Selector
	startedPods sync.Map
}

func newWatchPodLogsEventHandler(ctx context.Context, input watchPodLogsInput, selector labels.Selector) cache.ResourceEventHandler {
	return &watchPodLogsEventHandler{
		ctx:         ctx,
		input:       input,
		selector:    selector,
		startedPods: sync.Map{},
	}
}

func (eh *watchPodLogsEventHandler) OnAdd(obj interface{}, _ bool) {
	pod := obj.(*corev1.Pod)
	eh.streamPodLogs(pod)
}

func (eh *watchPodLogsEventHandler) OnUpdate(_, newObj interface{}) {
	pod := newObj.(*corev1.Pod)
	eh.streamPodLogs(pod)
}

func (eh *watchPodLogsEventHandler) OnDelete(_ interface{}) {}

func (eh *watchPodLogsEventHandler) streamPodLogs(pod *corev1.Pod) {
	if pod.GetNamespace() != eh.input.Namespace {
		return
	}
	if !eh.selector.Matches(labels.Set(pod.GetLabels())) {
		return
	}
	if pod.Status.Phase != corev1.PodRunning {
		return
	}
	if _, loaded := eh.startedPods.LoadOrStore(pod.GetUID(), struct{}{}); loaded {
		return
	}

	for _, container := range pod.Spec.Containers {
		klog.Infof("Creating log watcher for controller %s, pod %s, container %s", klog.KRef(eh.input.Namespace, eh.input.DeploymentName), pod.Name, container.Name)

		// Create log metadata file.
		logMetadataFile := filepath.Clean(path.Join(eh.input.LogPath, eh.input.DeploymentName, pod.Name, container.Name+"-log-metadata.json"))
		Expect(os.MkdirAll(filepath.Dir(logMetadataFile), 0750)).To(Succeed())

		metadata := logMetadata{
			Job:       eh.input.Namespace + "/" + eh.input.DeploymentName,
			Namespace: eh.input.Namespace,
			App:       eh.input.DeploymentName,
			Pod:       pod.Name,
			Container: container.Name,
			NodeName:  pod.Spec.NodeName,
			Stream:    "stderr",
		}
		metadataBytes, err := json.Marshal(&metadata)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.WriteFile(logMetadataFile, metadataBytes, 0600)).To(Succeed())

		// Watch each container's logs in a goroutine so we can stream them all concurrently.
		go func(pod *corev1.Pod, container corev1.Container) {
			defer GinkgoRecover()

			logFile := filepath.Clean(path.Join(eh.input.LogPath, eh.input.DeploymentName, pod.Name, container.Name+".log"))
			Expect(os.MkdirAll(filepath.Dir(logFile), 0750)).To(Succeed())

			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			Expect(err).ToNot(HaveOccurred())
			defer f.Close()

			opts := &corev1.PodLogOptions{
				Container: container.Name,
				Follow:    true,
			}

			// Retry streaming the logs of the pods unless ctx.Done() or if the pod does not exist anymore.
			err = wait.PollUntilContextCancel(eh.ctx, 2*time.Second, false, func(ctx context.Context) (done bool, err error) {
				// Wait for pod to be in running state
				actual, err := eh.input.ClientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				if err != nil {
					// The pod got deleted if the error IsNotFound. In this case there are also no logs to stream anymore.
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					// Only log the error to not cause the test to fail via GinkgoRecover
					klog.Infof("Error getting pod %s, container %s: %v", klog.KRef(pod.Namespace, pod.Name), container.Name, err)
					return true, nil
				}
				// Retry later if pod is currently not running
				if actual.Status.Phase != corev1.PodRunning {
					return false, nil
				}
				podLogs, err := eh.input.ClientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, opts).Stream(ctx)
				if err != nil {
					// Only log the error to not cause the test to fail via GinkgoRecover
					klog.Infof("Error starting logs stream for pod %s, container %s: %v", klog.KRef(pod.Namespace, pod.Name), container.Name, err)
					return true, nil
				}
				defer podLogs.Close()

				out := bufio.NewWriter(f)
				defer out.Flush()
				_, err = out.ReadFrom(podLogs)
				if err != nil && err != io.ErrUnexpectedEOF {
					// Failing to stream logs should not cause the test to fail
					klog.Infof("Got error while streaming logs for pod %s, container %s: %v", klog.KRef(pod.Namespace, pod.Name), container.Name, err)
				}
				return false, nil
			})
			if err != nil {
				klog.Infof("Stopped streaming logs for pod %s, container %s: %v", klog.KRef(pod.Namespace, pod.Name), container.Name, err)
			}
		}(pod, container)
	}
}

// logMetadata contains metadata about the logs.
// The format is very similar to the one used by promtail.
type logMetadata struct {
	Job       string            `json:"job"`
	Namespace string            `json:"namespace"`
	App       string            `json:"app"`
	Pod       string            `json:"pod"`
	Container string            `json:"container"`
	NodeName  string            `json:"node_name"`
	Stream    string            `json:"stream"`
	Labels    map[string]string `json:"labels,omitempty"`
}
