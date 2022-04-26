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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
			daemon := daemonList.Items[0]
			if daemon.Name != daemonsetName {
				return errors.New("CPI daemon set name is not vsphere-cpi, instead " + daemon.Name)
			}

			By("CPI daemon should be running")
			if daemon.Status.NumberReady != daemon.Status.DesiredNumberScheduled {
				return errors.New("CPI number ready not equal to the desired number to schedule")
			}
			return nil
		}, 20*time.Second).Should(BeNil())
	})
})
