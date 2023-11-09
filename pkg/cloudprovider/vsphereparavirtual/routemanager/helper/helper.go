package helper

import (
	"strings"
	"time"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// A list of possible RouteSet or StaticRoute operation error messages
var (
	ErrGetRouteCR    = errors.New("failed to get Route CR")
	ErrCreateRouteCR = errors.New("failed to create Route CR")
	ErrListRouteCR   = errors.New("failed to list Route CR")
	ErrDeleteRouteCR = errors.New("failed to delete Route CR")
)

const (
	// LabelKeyClusterName is the label key to specify GC name for RouteSet/StaticRoute CR
	LabelKeyClusterName = "clusterName"
	// RealizedStateTimeout is the timeout duration for realized state check
	RealizedStateTimeout = 10 * time.Second
	// RealizedStateSleepTime is the interval between realized state check
	RealizedStateSleepTime = 1 * time.Second
)

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
