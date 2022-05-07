package e2e

import (
	"errors"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// getWorkerNode retrieves the worker node object for the E2E testing using workload cluster's clientset
func getWorkerNode() (*corev1.Node, error) {
	nodes, err := workloadClientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return getFirstWorkerNodeFromList(nodes)
}

// getFirstWorkerNodeFromList searches the first worker node that forms the cluster.
// We assume all the name of worker node contains substring `-md-`
func getFirstWorkerNodeFromList(nodes *corev1.NodeList) (*corev1.Node, error) {
	for _, node := range nodes.Items {
		if strings.Contains(node.Name, "-md-") {
			return &node, nil
		}
	}
	return nil, errors.New("worker node not found")
}

// getExternalIPFromNode returns the external IP from Node.status.addresses, given a node object
func getExternalIPFromNode(node *corev1.Node) (string, error) {
	addresses := node.Status.Addresses
	for _, address := range addresses {
		if address.Type == corev1.NodeExternalIP {
			return address.String(), nil
		}
	}
	return "", errors.New("external IP not found")
}

// getInternalIPFromNode returns the internal IP from Node.status.addresses, given a node object
func getInternalIPFromNode(node *corev1.Node) (string, error) {
	addresses := node.Status.Addresses
	for _, address := range addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.String(), nil
		}
	}
	return "", errors.New("internal IP not found")
}

// getProviderIPFromNode returns the provider ID of node
func getProviderIPFromNode(node *corev1.Node) string {
	return node.Spec.ProviderID
}

// DoesNodeHasReadiness returns whether the not is the given node ready
func DoesNodeHasReadiness(node *corev1.Node, readiness corev1.ConditionStatus) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == readiness
		}
	}
	return false
}

// getWorkerVM retrieves the worker virtual machine for the E2E testing with govmomi
func getWorkerVM(name string) (*object.VirtualMachine, error) {
	workerVMs, err := vsphere.Finder.VirtualMachineList(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(workerVMs) != 1 {
		return nil, errors.New("expect only one virtual machine with name " + name)
	}
	return workerVMs[0], nil
}

// WaitForWorkerNodeReadiness returns a function for Eventually that
// retrieves the latest node and asserts its readiness
func WaitForWorkerNodeReadiness(readiness corev1.ConditionStatus) func() error {
	return func() error {
		node, err := getWorkerNode()
		if err != nil {
			return err
		}
		if DoesNodeHasReadiness(node, readiness) {
			return errors.New("worker node ready status is not " + string(readiness))
		}
		return nil
	}
}

// WaitForVMPowerState returns a function for Eventually that
// retrieves the latest virtual machine and asserts its power state
func WaitForVMPowerState(name string, targetState types.VirtualMachinePowerState) func() error {
	return func() error {
		vm, err := getWorkerVM(name)
		if err != nil {
			return err
		}
		state, err := vm.PowerState(ctx)
		if err != nil {
			return err
		}
		if state != targetState {
			return errors.New("worker vm hasn't become " + string(targetState))
		}
		return nil
	}
}

var _ = Describe("Recreating VMs", func() {
	It("should result in new nodes", func() {
		// TODO(fhan)
	})
})

/*
	Restart a worker node, then assert that the external, internal IP and
	the provider ID for the node should not change.
*/
var _ = Describe("Restarting VMs", func() {

	var workerNode *corev1.Node
	var workerVM *object.VirtualMachine

	BeforeEach(func() {
		By("Get the name of worker node", func() {
			workerNode, err = getWorkerNode()
			Expect(err).ToNot(HaveOccurred())

			klog.Infof("The worker node for testing is %s\n", workerNode.Name)
		})

		By("Get corresponding VM object for node", func() {
			workerVM, err = getWorkerVM(workerNode.Name)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	It("should pertain the original node when VM restarts", func() {

		By("Read the externalIP, internalIP and providerID of VM")
		externalIP, err := getExternalIPFromNode(workerNode)
		Expect(err).ToNot(HaveOccurred())

		internalIP, err := getInternalIPFromNode(workerNode)
		Expect(err).ToNot(HaveOccurred())

		providerID := getProviderIPFromNode(workerNode)
		Expect(DoesNodeHasReadiness(workerNode, corev1.ConditionTrue)).To(BeTrue())

		By("Shutdown VM "+workerVM.Name(), func() {
			task, err := workerVM.PowerOff(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot power off vm")

			err = task.Wait(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot wait for vm to power off")
		})

		By("Wait for VM " + workerVM.Name() + " to go down")
		Eventually(WaitForVMPowerState(workerVM.Name(), types.VirtualMachinePowerStatePoweredOff))

		By("Wait for node " + workerNode.Name + " to become not ready")
		Eventually(WaitForWorkerNodeReadiness(corev1.ConditionFalse), 5*time.Minute, 2*time.Second).Should(BeNil())

		By("Power on VM "+workerVM.Name(), func() {
			task, err := workerVM.PowerOn(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot power on vm")

			err = task.Wait(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot wait for vm to power on")
		})

		By("Wait for VM " + workerVM.Name() + " to go up again")
		Eventually(WaitForVMPowerState(workerVM.Name(), types.VirtualMachinePowerStatePoweredOn))

		By("Wait for node " + workerNode.Name + " to become ready")
		Eventually(WaitForWorkerNodeReadiness(corev1.ConditionTrue), 5*time.Minute, 5*time.Second).Should(BeNil())

		By("Assert that externalIP, internalIP and providerID are preserved after VM restarts", func() {
			Eventually(func() error {
				workerNode, err = getWorkerNode()
				Expect(err).ToNot(HaveOccurred())

				newExternalIP, err := getExternalIPFromNode(workerNode)
				Expect(err).ToNot(HaveOccurred())

				newInternalIP, err := getInternalIPFromNode(workerNode)
				Expect(err).ToNot(HaveOccurred())

				Expect(newExternalIP).To(Equal(externalIP))
				Expect(newInternalIP).To(Equal(internalIP))
				Expect(getProviderIPFromNode(workerNode)).To(Equal(providerID))

				return nil
			}).Should(Succeed())
		})
	})
})
