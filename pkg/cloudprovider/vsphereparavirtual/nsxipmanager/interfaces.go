package nsxipmanager

import (
	corev1 "k8s.io/api/core/v1"
)

// NSXIPManager defines an interface that can interact with NSX to claim/release pod cidr.
type NSXIPManager interface {
	// ClaimPodCIDR claims a pod cidr for a node.
	ClaimPodCIDR(node *corev1.Node) error
	// ReleasePodCIDR releases a pod cidr for a node.
	ReleasePodCIDR(node *corev1.Node) error
}
