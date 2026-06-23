/*
Copyright 2026 The Kubernetes Authors.

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

package helper

import (
	"net"
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Sentinel errors for RouteSet and StaticRoute CRUD operations.
// Callers wrap these with fmt.Errorf or errors.Wrap to attach the underlying cause.
var (
	ErrGetRouteCR    = errors.New("failed to get Route CR")
	ErrCreateRouteCR = errors.New("failed to create Route CR")
	ErrListRouteCR   = errors.New("failed to list Route CR")
	ErrDeleteRouteCR = errors.New("failed to delete Route CR")
)

const (
	// LabelKeyClusterName is the label key for the guest-cluster name on RouteSet/StaticRoute CRs.
	LabelKeyClusterName = "clusterName"
	// LabelKeyNodeName is the label key for the Kubernetes node name that owns the route.
	// Using a label avoids reconstructing the node name from CR name suffixes.
	LabelKeyNodeName = "nodeName"
	// LabelKeyIPFamily is the label key that identifies the IP address family of a route CR.
	// Valid values are LabelValueIPFamilyIPv4 and LabelValueIPFamilyIPv6.
	LabelKeyIPFamily = "ipfamily"

	// LabelValueIPFamilyIPv4 is the ipfamily label value for IPv4 routes.
	LabelValueIPFamilyIPv4 = "ipv4"
	// LabelValueIPFamilyIPv6 is the ipfamily label value for IPv6 routes.
	LabelValueIPFamilyIPv6 = "ipv6"

	// RealizedStateTimeout is the timeout duration for realized state check
	RealizedStateTimeout = 10 * time.Second
	// RealizedStateSleepTime is the interval between realized state check
	RealizedStateSleepTime = 1 * time.Second
	// SuffixIPv6 is appended to CR names that carry IPv6 routes or address allocations.
	// IPv4 CRs use the bare node name; only IPv6 CRs carry this suffix.
	SuffixIPv6 = "-ipv6"
)

// IPFamilyLabel returns the ipfamily label value for the given CIDR string,
// or "" if the input is not a parseable CIDR. Callers should treat a return
// value of "" as a programmer error and avoid writing the empty label onto
// a CR; an unset ipfamily label is preferable to silently mis-tagging.
func IPFamilyLabel(cidr string) string {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return ""
	}
	if ip.To4() != nil {
		return LabelValueIPFamilyIPv4
	}
	return LabelValueIPFamilyIPv6
}

// StripFamilySuffix removes the SuffixIPv6 suffix from a CR name to recover
// the bare node name. IPv4 CRs use the bare node name and are returned as-is.
func StripFamilySuffix(name string) string {
	return strings.TrimSuffix(name, SuffixIPv6)
}

// RouteCR defines an interface that is used to represent different kinds of nsx.vmware.com route CR
type RouteCR interface{}

// RouteCRList defines an interface that is used to represent different kinds of nsx.vmware.com route CR List
type RouteCRList interface{}

// RouteInfo collects all the information to build a RouteCR
type RouteInfo struct {
	Namespace string
	Labels    map[string]string
	Owner     []metav1.OwnerReference
	Name      string // route cr name / node name
	Cidr      string // destination network
	NodeIP    string // next hop / target ip
	RouteName string
}

// GetRouteName returns RouteInfo name as <nodeName>-<cidr>-<clusterName>
// e.g. nodeName-100.96.0.0-24-clusterName
func GetRouteName(nodeName string, cidr string, clusterName string) string {
	return strings.Replace(nodeName+"-"+cidr+"-"+clusterName, "/", "-", -1)
}
