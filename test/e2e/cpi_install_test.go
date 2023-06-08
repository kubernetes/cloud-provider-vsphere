/*
Copyright 2022 The Kubernetes Authors.

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
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// findVSphereCPIDaemonsetInList searches a daemonset with name vsphere-cpi in the daemon list
func findVSphereCPIDaemonsetInList(daemonList *appsv1.DaemonSetList) (*appsv1.DaemonSet, error) {
	for _, d := range daemonList.Items {
		if d.Name == daemonsetName {
			return &d, nil
		}
	}
	return nil, errors.New("CPI daemon set with name vsphere-cpi not found")
}

/*
	CPI should be installable from the helm chart. Its daemon set will eventually
	become ready with number equals to the desired pods.
*/
var _ = Describe("Deploy cloud provider vSphere with helm", func() {
	It("should have running CPI daemon set", func() {
		Eventually(func() error {
			By("CPI daemon should exists")
			daemonList, err := workloadClientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			if len(daemonList.Items) == 0 {
				return errors.New("CPI daemon list is empty")
			}
			daemon, err := findVSphereCPIDaemonsetInList(daemonList)

			By("CPI daemon should be running")
			if daemon.Status.NumberReady != daemon.Status.DesiredNumberScheduled {
				return errors.New("CPI number ready not equal to the desired number to schedule")
			}
			return nil
		}, 2*time.Minute, 5*time.Second).Should(BeNil())
	})

	It("should have all CPI pods in the running state", func() {
		Eventually(func() error {
			pods, err := workloadClientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			for _, pod := range pods.Items {
				if strings.HasPrefix(pod.Name, daemonsetName) {
					Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
					for _, containerStatus := range pod.Status.ContainerStatuses {
						Expect(containerStatus.Ready).To(BeTrue())
					}
				}
			}
			return nil
		}).Should(Succeed())
	})
})
