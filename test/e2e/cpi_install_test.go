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
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/mo"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
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
			Expect(err).ShouldNot(HaveOccurred())

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

	It("should apply nodes config and select correct internal IP for worker node", func() {
		var workerNode *corev1.Node
		var workerVM *object.VirtualMachine
		var originalInternalIP string
		var networkName string

		By("Get the current active worker node from the cluster", func() {
			var err error
			workerNode, err = getWorkerNode()
			Expect(err).NotTo(HaveOccurred())
		})

		By("Fetch the Node's Internal IP", func() {
			var err error
			originalInternalIP, err = getInternalIPFromNode(workerNode)
			Expect(err).NotTo(HaveOccurred())
			Expect(originalInternalIP).NotTo(BeEmpty(), "worker node has no internal IP")
			klog.Infof("Worker node %s internal IP: %s", workerNode.Name, originalInternalIP)
		})

		By("Compute the /24 subnet dynamically from the node's internal IP", func() {
			ip := net.ParseIP(originalInternalIP)
			Expect(ip).NotTo(BeNil(), fmt.Sprintf("invalid IP address: %s", originalInternalIP))

			_, subnet, err := net.ParseCIDR(originalInternalIP + "/24")
			Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("failed to parse /24 CIDR for IP %s", originalInternalIP))
			klog.Infof("Computed /24 subnet: %s", subnet.String())
		})

		By("Locate the vSphere Virtual Machine matching the Node name", func() {
			var err error
			workerVM, err = getWorkerVM(workerNode.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(workerVM).NotTo(BeNil())
			klog.Infof("Found VM %s for node %s", workerVM.Name(), workerNode.Name)
		})

		By("Query all networks attached to the VM's vNICs", func() {
			var vm mo.VirtualMachine
			err := workerVM.Properties(
				ctx,
				workerVM.Reference(),
				[]string{"network"},
				&vm,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(vm.Network).NotTo(
				BeEmpty(),
				"The worker VM has no attached network interfaces",
			)

			for _, netRef := range vm.Network {
				netObj := object.NewNetwork(vsphere.Client.Client, netRef)

				name, err := netObj.ObjectName(ctx)
				Expect(err).NotTo(HaveOccurred())

				if name != "" {
					networkName = name
					break
				}
			}

			Expect(networkName).NotTo(
				BeEmpty(),
				"could not determine network name from VM",
			)
			klog.Infof(
				"VM %s is attached to network: %s",
				workerVM.Name(),
				networkName,
			)
		})

		By("Upgrade CPI using helm with the nodes configuration", func() {
			cmdName := "helm"
			cmdArgs := []string{
				"upgrade", daemonsetName, "vsphere-cpi/vsphere-cpi",
				"--namespace", namespace,
				"--reuse-values",
				"--set", fmt.Sprintf("config.nodes.internalNetworkSubnetCidr=%s", originalInternalIP+"/24"),
				"--set", fmt.Sprintf("config.nodes.internalVmNetworkName=%s", networkName),
				"--set", "config.nodes.excludeInternalNetworkSubnetCidr=255.255.255.0/24",
			}

			cmd := exec.Command(cmdName, cmdArgs...)
			cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", workloadKubeconfig))

			output, err := cmd.CombinedOutput()
			klog.Infof("Helm upgrade output: %s\n", string(output))
			Expect(err).NotTo(HaveOccurred(), string(output))
		})

		By("Wait for CPI daemonset to become available after upgrade", func() {
			Eventually(func() error {
				daemonList, err := workloadClientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{})
				if err != nil {
					return err
				}
				daemon, err := findVSphereCPIDaemonsetInList(daemonList)
				if err != nil {
					return err
				}
				if daemon.Status.NumberReady != daemon.Status.DesiredNumberScheduled {
					return errors.New("CPI daemonset not fully ready after upgrade")
				}
				return nil
			}, 5*time.Minute, 5*time.Second).Should(Succeed())
		})

		By("Verify that the worker node's internal IP remains within the configured subnet", func() {
			_, expectedSubnet, err := net.ParseCIDR(originalInternalIP + "/24")
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				node, err := getWorkerNode()
				if err != nil {
					return err
				}
				internalIP, err := getInternalIPFromNode(node)
				if err != nil {
					return err
				}
				ip := net.ParseIP(internalIP)
				if ip == nil {
					return errors.New("invalid internal IP")
				}
				if !expectedSubnet.Contains(ip) {
					return fmt.Errorf("internal IP %s is not within expected subnet %s", internalIP, expectedSubnet.String())
				}
				return nil
			}, 2*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})
