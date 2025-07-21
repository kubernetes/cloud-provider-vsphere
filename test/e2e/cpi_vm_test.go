package e2e

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	machineNamespace      = "default"
	ControlPlaneNodeLabel = "node-role.kubernetes.io/control-plane"
)

// getWorkerNode retrieves the first worker node object for the E2E testing using workload cluster's clientset
// Only control plane Node has ControlPlaneNodeLabel.
func getWorkerNode() (*corev1.Node, error) {
	nodes, err := workloadClientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, node := range nodes.Items {
		if _, ok := node.GetLabels()[ControlPlaneNodeLabel]; !ok {
			// get the first worker node
			return &node, nil
		}

	}
	return nil, errors.New("worker node not found")
}

// getWorkerMachine retrieves the CAPI machine object with name from the boostrap cluster
func getWorkerMachine(name string) (*v1beta2.Machine, error) {
	machineList := &v1beta2.MachineList{}
	err := proxy.GetClient().List(ctx, machineList)
	if err != nil {
		return nil, errors.New("failed to list Machines")
	}

	for _, machine := range machineList.Items {
		if machine.Status.NodeRef.Name == name {
			return &machine, nil
		}
	}

	return nil, errors.New("machine not found")
}

// deleteWorkerMachine deletes the CAPI machine object with name from the boostrap cluster
func deleteWorkerMachine(name string) error {
	machine := &v1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: machineNamespace,
		},
	}
	return proxy.GetClient().Delete(ctx, machine)
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

// getProviderIDFromNode returns the provider ID of node
func getProviderIDFromNode(node *corev1.Node) string {
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

// updateClusterSpecPaused update the Cluster.Spec.Paused field with desired value.
func updateClusterSpecPaused(ctx context.Context, name string, namespace string, desired string) {
	workloadCluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    proxy.GetClient(),
		Namespace: namespace,
		Name:      name,
	})

	patch := ctrlclient.RawPatch(k8stypes.MergePatchType, []byte(fmt.Sprintf("{\"spec\":{\"paused\":%s}}", desired)))
	err = proxy.GetClient().Patch(ctx, workloadCluster, patch)
	Expect(err).ToNot(HaveOccurred())
}

// WaitForWorkerNodeReadiness returns a function for Eventually that
// retrieves the latest node and asserts its readiness
func WaitForWorkerNodeReadiness(readiness corev1.ConditionStatus) func() error {
	return func() error {
		node, err := getWorkerNode()
		if err != nil {
			return err
		}
		if !DoesNodeHasReadiness(node, readiness) {
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

/*
Restart a worker node, then assert that the external, internal IP and
the provider ID for the node should not change.

Delete the worker machine object in the boostrap cluster, after a while CAPV should create a new machine
associated with a new VM. The new node should have correct info.

Delete the VM from VC API, the node should be gone as well
*/
var _ = Describe("Restarting, recreating and deleting VMs", func() {

	var originalWorkerNodeName string
	var workerNode *corev1.Node
	var workerMachine *v1beta2.Machine
	var workerVM *object.VirtualMachine

	BeforeEach(func() {
		By("Get the name of worker node", func() {
			workerNode, err = getWorkerNode()
			Expect(err).ToNot(HaveOccurred())

			klog.Infof("The worker node for testing is %s\n", workerNode.Name)
			originalWorkerNodeName = workerNode.Name
		})

		By("Get the machine object in bootstrap cluster", func() {
			workerMachine, err = getWorkerMachine(workerNode.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(workerMachine).ToNot(BeNil())
		})

		By("Get corresponding VM object for node", func() {
			workerVM, err = getWorkerVM(workerNode.Name)
			Expect(err).ToNot(HaveOccurred())
			Expect(workerVM).ToNot(BeNil())
		})
	})

	It("should pertain the original node when VM restarts", func() {

		Eventually(func() bool {
			workerNode, err = getWorkerNode()
			if err != nil {
				return false
			}
			return DoesNodeHasReadiness(workerNode, corev1.ConditionTrue)
		}, 10*time.Minute).Should(BeTrue())

		By("Read the externalIP, internalIP and providerID of VM")
		externalIP, err := getExternalIPFromNode(workerNode)
		Expect(err).ToNot(HaveOccurred())

		internalIP, err := getInternalIPFromNode(workerNode)
		Expect(err).ToNot(HaveOccurred())

		providerID := getProviderIDFromNode(workerNode)

		By("Pause reconcile for workload cluster", func() {
			updateClusterSpecPaused(ctx, workloadResult.Cluster.Name, workloadResult.Cluster.Namespace, "true")
			Eventually(func() bool {
				wldCluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
					Getter:    proxy.GetClient(),
					Namespace: workloadResult.Cluster.Namespace,
					Name:      workloadResult.Cluster.Name,
				})

				return wldCluster.Spec.Paused
			}, 180*time.Second, 10*time.Second).Should(BeTrue(), "Failed to pause the Workload Cluster")
		})

		By("Shutdown VM "+workerVM.Name(), func() {
			task, err := workerVM.PowerOff(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot power off vm")

			err = task.Wait(ctx)
			Expect(err).ToNot(HaveOccurred(), "cannot wait for vm to power off")
		})

		By("Wait for VM " + workerVM.Name() + " to go down")
		Eventually(WaitForVMPowerState(workerVM.Name(), types.VirtualMachinePowerStatePoweredOff))

		By("Wait for node " + workerNode.Name + " to become not ready")
		Eventually(WaitForWorkerNodeReadiness(corev1.ConditionUnknown), 5*time.Minute, 2*time.Second).Should(BeNil())

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

		By("Unpause reconcile for workload cluster", func() {
			updateClusterSpecPaused(ctx, workloadResult.Cluster.Name, workloadResult.Cluster.Namespace, "false")
			Eventually(func() bool {
				wldCluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
					Getter:    proxy.GetClient(),
					Namespace: workloadResult.Cluster.Namespace,
					Name:      workloadResult.Cluster.Name,
				})

				return wldCluster.Spec.Paused
			}, 180*time.Second, 10*time.Second).Should(BeFalse(), "Failed to unpause the Workload Cluster")
		})

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
				Expect(getProviderIDFromNode(workerNode)).To(Equal(providerID))

				return nil
			}).Should(Succeed())
		})
	})

	It("should result in new node when recreating VM", func() {

		Eventually(func() bool {
			workerNode, err = getWorkerNode()
			if err != nil {
				return false
			}
			return DoesNodeHasReadiness(workerNode, corev1.ConditionTrue)
		}, 10*time.Minute).Should(BeTrue())

		By("Read the providerID of VM")
		providerID := getProviderIDFromNode(workerNode)

		By("Delete machine object", func() {
			err := deleteWorkerMachine(workerMachine.Name)
			Expect(err).To(BeNil(), "cannot delete machine object")
		})

		By("Eventually original node will be gone")
		Eventually(func() bool {
			_, err := workloadClientset.CoreV1().Nodes().Get(ctx, workerNode.Name, metav1.GetOptions{})
			return err != nil && apierrors.IsNotFound(err)
		}, 5*time.Minute, 5*time.Second).Should(BeTrue())

		By("Eventually new node will be created")
		var newExternalIP, newInternalIP string
		Eventually(func() error {
			if workerNode, err = getWorkerNode(); err != nil {
				return err
			}
			if newExternalIP, err = getExternalIPFromNode(workerNode); err != nil {
				return err
			}
			if newInternalIP, err = getInternalIPFromNode(workerNode); err != nil {
				return err
			}
			return nil
		}, 10*time.Minute, 5*time.Second).Should(Succeed())

		By("New node will be created with correct info, different from old one")
		Expect(newExternalIP).ToNot(BeEmpty())
		Expect(newInternalIP).ToNot(BeEmpty())
		Expect(getProviderIDFromNode(workerNode)).ToNot(BeEmpty())

		Expect(workerNode.Name).ToNot(Equal(originalWorkerNodeName), "name still the same")
		Expect(getProviderIDFromNode(workerNode)).ToNot(Equal(providerID), "providerID still the same")
	})

	It("should result in new node when deleting VM from VC", func() {

		Eventually(func() bool {
			workerNode, err = getWorkerNode()
			if err != nil {
				return false
			}
			return DoesNodeHasReadiness(workerNode, corev1.ConditionTrue)
		}, 10*time.Minute).Should(BeTrue())

		By("Powering off machine object")
		task, err := workerVM.PowerOff(ctx)
		Expect(err).ToNot(HaveOccurred(), "cannot power off vm")

		err = task.Wait(ctx)
		Expect(err).ToNot(HaveOccurred(), "cannot wait for vm to power off")

		By("Delete VM from VC")
		task, err = workerVM.Destroy(ctx)
		Expect(err).ToNot(HaveOccurred(), "cannot destroy vm")

		err = task.Wait(ctx)
		Expect(err).ToNot(HaveOccurred(), "cannot wait for vm to destroy")

		By("Eventually original node will be gone")
		Eventually(func() bool {
			_, err := workloadClientset.CoreV1().Nodes().Get(ctx, workerNode.Name, metav1.GetOptions{})
			return err != nil && apierrors.IsNotFound(err)
		}, 5*time.Minute, 5*time.Second).Should(BeTrue())
	})
})
