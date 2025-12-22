/*
Copyright 2025 The Kubernetes Authors.

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
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// loadImagesToCluster deploys a privileged daemonset and uses it to stream-load container images.
// This implementation fixes the runc mountpoint issue by mounting specific paths instead of root.
func loadImagesToCluster(ctx context.Context, sourceFile string, clusterProxy framework.ClusterProxy) {
	daemonSet, daemonSetMutateFn, daemonSetLabels := getPreloadDaemonset()
	ctrlClient := clusterProxy.GetClient()

	// Create the DaemonSet.
	_, err := controllerutil.CreateOrPatch(ctx, ctrlClient, daemonSet, daemonSetMutateFn)
	Expect(err).ToNot(HaveOccurred())

	// Wait for DaemonSet to be available.
	waitForDaemonSetAvailable(ctx, waitForDaemonSetAvailableInput{Getter: ctrlClient, Daemonset: daemonSet}, time.Minute*3, time.Second*10)

	// List all pods and load images via each found pod.
	pods := &corev1.PodList{}
	Expect(ctrlClient.List(
		ctx,
		pods,
		client.InNamespace(daemonSet.Namespace),
		client.MatchingLabels(daemonSetLabels),
	)).To(Succeed())

	errs := []error{}
	for j := range pods.Items {
		pod := pods.Items[j]
		klog.Infof("Loading images to node %s via pod %s/%s", pod.Spec.NodeName, pod.Namespace, pod.Name)
		if err := loadImagesViaPod(ctx, clusterProxy, sourceFile, pod.Namespace, pod.Name, pod.Spec.Containers[0].Name); err != nil {
			errs = append(errs, err)
		}
	}
	Expect(kerrors.NewAggregate(errs)).ToNot(HaveOccurred())

	// Delete the DaemonSet.
	Expect(ctrlClient.Delete(ctx, daemonSet)).To(Succeed())
}

func loadImagesViaPod(ctx context.Context, clusterProxy framework.ClusterProxy, sourceFile, namespace, podName, containerName string) error {
	clientSet := clusterProxy.GetClientSet()
	restConfig := clusterProxy.GetRESTConfig()

	// Open the source file
	file, err := os.Open(sourceFile)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", sourceFile, err)
	}
	defer file.Close()

	// Create the exec request
	// Note: We use nsenter to access the host's containerd namespace
	// This avoids the need to mount the host root filesystem
	req := clientSet.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: containerName,
			Command: []string{
				"nsenter",
				"--target", "1",
				"--mount",
				"--",
				"ctr",
				"--address=/run/containerd/containerd.sock",
				"--namespace=k8s.io",
				"images",
				"import",
				"-",
			},
			Stdin:  true,
			Stdout: true,
			Stderr: true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(restConfig, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:  file,
		Stdout: &stdout,
		Stderr: &stderr,
	})

	if err != nil {
		return fmt.Errorf("failed to stream image to pod %s/%s: %w\nstdout: %s\nstderr: %s",
			namespace, podName, err, stdout.String(), stderr.String())
	}

	return nil
}

// getPreloadDaemonset returns a DaemonSet that can be used to preload images.
// This implementation uses nsenter instead of mounting host root to avoid runc issues.
func getPreloadDaemonset() (*appsv1.DaemonSet, controllerutil.MutateFn, map[string]string) {
	labels := map[string]string{
		"app": "image-preloader",
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceSystem,
			Name:      "image-preloader",
			Labels:    labels,
		},
	}
	mutateFunc := func() error {
		ds.Labels = labels
		ds.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "preloader",
							Image:   "busybox:1.36",
							Command: []string{"/bin/sh", "-c", "sleep infinity"},
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							// Note: We mount /run/containerd to access the containerd socket
							// and use nsenter to access host namespaces.
							// This avoids mounting "/" which triggers runc v1.3.3+ mountpoint errors.
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "containerd-sock",
									MountPath: "/run/containerd",
								},
							},
						},
					},
					HostPID: true,
					HostIPC: true,
					Volumes: []corev1.Volume{
						{
							Name: "containerd-sock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/containerd",
									Type: ptr.To(corev1.HostPathDirectory),
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						// Tolerate any taint.
						{
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		}
		return nil
	}
	return ds, mutateFunc, labels
}

type waitForDaemonSetAvailableInput struct {
	Getter    framework.Getter
	Daemonset *appsv1.DaemonSet
}

func waitForDaemonSetAvailable(ctx context.Context, input waitForDaemonSetAvailableInput, intervals ...interface{}) {
	By(fmt.Sprintf("Waiting for DaemonSet %s to be available", klog.KObj(input.Daemonset)))
	Eventually(func() bool {
		ds := &appsv1.DaemonSet{}
		key := client.ObjectKey{
			Namespace: input.Daemonset.Namespace,
			Name:      input.Daemonset.Name,
		}
		if err := input.Getter.Get(ctx, key, ds); err != nil {
			return false
		}
		return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled &&
			ds.Status.DesiredNumberScheduled > 0
	}, intervals...).Should(BeTrue(), func() string {
		ds := &appsv1.DaemonSet{}
		key := client.ObjectKey{
			Namespace: input.Daemonset.Namespace,
			Name:      input.Daemonset.Name,
		}
		_ = input.Getter.Get(ctx, key, ds)
		return fmt.Sprintf("DaemonSet %s is not available yet: NumberReady=%d, DesiredNumberScheduled=%d",
			klog.KObj(ds), ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	})
}

