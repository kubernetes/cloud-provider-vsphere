package utils

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	cidrUpdateRetries = 3
	// CIDRAssignmentFailedStatus is the event when the Pod CIDR is failed to assign to a Node
	CIDRAssignmentFailedStatus = "CIDRAssignmentFailed"
)

type nodeForCIDRMergePatch struct {
	Spec nodeSpecForMergePatch `json:"spec"`
}

type nodeSpecForMergePatch struct {
	PodCIDR  string   `json:"podCIDR"`
	PodCIDRs []string `json:"podCIDRs,omitempty"`
}

// PatchNodeCIDRWithRetry patches the specified node's PodCIDR and PodCIDRs with retries.
// cidrs must be non-empty; cidrs[0] is used as PodCIDR.
func PatchNodeCIDRWithRetry(ctx context.Context, kubeclientset kubernetes.Interface, node *corev1.Node, cidrs []string, recorder record.EventRecorder) error {
	logger := klog.FromContext(ctx)
	var err error
	for i := 0; i < cidrUpdateRetries; i++ {
		if err = PatchNodeCIDR(ctx, kubeclientset, node.Name, cidrs); err == nil {
			logger.V(4).Info(fmt.Sprintf("Set PodCIDRs to %v on Node %s", cidrs, node.Name))
			return nil
		}
	}
	logger.Error(err, fmt.Sprintf("Failed to set PodCIDRs %v on Node %s after multiple attempts", cidrs, node.Name))
	RecordNodeCIDRAssignmentFailed(ctx, node, recorder)
	logger.Error(err, fmt.Sprintf("CIDR assignment for Node %s failed. Try again in next reconcile", node.Name))
	return err
}

// PatchNodeCIDR patches the specified node's PodCIDR and PodCIDRs.
// cidrs must be non-empty; cidrs[0] is used as PodCIDR (primary family).
func PatchNodeCIDR(ctx context.Context, kubeclientset kubernetes.Interface, name string, cidrs []string) error {
	if len(cidrs) == 0 {
		return fmt.Errorf("cidrs must not be empty")
	}
	patch := nodeForCIDRMergePatch{
		Spec: nodeSpecForMergePatch{
			PodCIDR:  cidrs[0],
			PodCIDRs: cidrs,
		},
	}
	patchBytes, err := json.Marshal(&patch)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal CIDR: %v", err)
	}

	if _, err := kubeclientset.CoreV1().Nodes().Patch(ctx, name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("failed to patch CIDR: %v", err)
	}
	return nil
}

// RecordNodeStatusChange records an event related to a node status change. (Common to lifecycle and ipam)
func RecordNodeStatusChange(ctx context.Context, node *corev1.Node, newStatus string, recorder record.EventRecorder) {
	ref := &corev1.ObjectReference{
		APIVersion: "v1",
		Kind:       "Node",
		Name:       node.Name,
		UID:        node.UID,
		Namespace:  "",
	}
	logger := klog.FromContext(ctx)
	logger.V(2).Info(fmt.Sprintf("Recording status change %s event message for Node %s", newStatus, node.Name))
	recorder.Eventf(ref, corev1.EventTypeNormal, newStatus, "Node %s status is now: %s", node.Name, newStatus)
}

// RecordNodeCIDRAssignmentFailed records an event related to a node CIDR assignment failure.
func RecordNodeCIDRAssignmentFailed(ctx context.Context, node *corev1.Node, recorder record.EventRecorder) {
	RecordNodeStatusChange(ctx, node, CIDRAssignmentFailedStatus, recorder)
}
