/*
Copyright 2018 The Kubernetes Authors.

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

package vsphere

import (
	"sync"

	v1 "k8s.io/api/core/v1"
	clientv1 "k8s.io/client-go/listers/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	k8s "k8s.io/cloud-provider-vsphere/pkg/common/kubernetes"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// GRPCServer describes an object that can start a gRPC server.
type GRPCServer interface {
	Start()
}

// VSphere is an implementation of cloud provider Interface for VSphere.
type VSphere struct {
	cfg               *vcfg.Config
	connectionManager *cm.ConnectionManager
	nodeManager       *NodeManager
	informMgr         *k8s.InformerManager
	instances         cloudprovider.Instances
	zones             cloudprovider.Zones
	server            GRPCServer
}

// NodeInfo is information about a Kubernetes node.
type NodeInfo struct {
	dataCenter    *vclib.Datacenter
	vm            *vclib.VirtualMachine
	vcServer      string
	UUID          string
	NodeName      string
	NodeType      string
	NodeAddresses []v1.NodeAddress
}

// DatacenterInfo is information about a vCenter datascenter.
type DatacenterInfo struct {
	name   string
	vmList map[string]*NodeInfo
}

// VCenterInfo is information about a vCenter.
type VCenterInfo struct {
	address string
	dcList  map[string]*DatacenterInfo
}

// NodeManager is used to manage Kubernetes nodes.
type NodeManager struct {
	// Maps node name to node info
	nodeNameMap map[string]*NodeInfo
	// Maps UUID to node info.
	nodeUUIDMap map[string]*NodeInfo
	// Maps VC -> DC -> VM
	vcList map[string]*VCenterInfo
	// Maps UUID to node info.
	nodeRegUUIDMap map[string]*v1.Node
	// ConnectionManager
	connectionManager *cm.ConnectionManager
	// NodeLister to track Node properties
	nodeLister clientv1.NodeLister

	// Mutexes
	nodeInfoLock    sync.RWMutex
	nodeRegInfoLock sync.RWMutex
}

type instances struct {
	nodeManager *NodeManager
}

type zones struct {
	nodeManager *NodeManager
	zone        string
	region      string
}

// GuestOSLookup is a table for quick lookup between guestOsIdentifier and a shorthand name
var GuestOSLookup = map[string]string{
	"asianux3_64Guest":        "asianux3",
	"asianux3Guest":           "asianux3",
	"asianux4_64Guest":        "asianux4",
	"asianux4Guest":           "asianux4",
	"asianux5_64Guest":        "asianux5",
	"asianux7_64Guest":        "asianux7",
	"centos6_64Guest":         "centos6",
	"centos64Guest":           "centos64",
	"centos6Guest":            "centos6",
	"centos7_64Guest":         "centos7",
	"centos7Guest":            "centos7",
	"centosGuest":             "centos",
	"coreos64Guest":           "coreos",
	"darwin10_64Guest":        "darwin",
	"darwin10Guest":           "darwin",
	"darwin11_64Guest":        "darwin",
	"darwin11Guest":           "darwin",
	"darwin12_64Guest":        "darwin",
	"darwin13_64Guest":        "darwin",
	"darwin14_64Guest":        "darwin",
	"darwin15_64Guest":        "darwin",
	"darwin16_64Guest":        "darwin",
	"darwin64Guest":           "darwin",
	"darwinGuest":             "darwin",
	"debian10_64Guest":        "debian10",
	"debian10Guest":           "debian10",
	"debian4_64Guest":         "debian4",
	"debian4Guest":            "debian4",
	"debian5_64Guest":         "debian5",
	"debian5Guest":            "debian5",
	"debian6_64Guest":         "debian6",
	"debian6Guest":            "debian6",
	"debian7_64Guest":         "debian7",
	"debian7Guest":            "debian7",
	"debian8_64Guest":         "debian8",
	"debian8Guest":            "debian8",
	"debian9_64Guest":         "debian9",
	"debian9Guest":            "debian9",
	"dosGuest":                "dos",
	"eComStation2Guest":       "eComStation2",
	"eComStationGuest":        "eComStation",
	"fedora64Guest":           "fedora",
	"fedoraGuest":             "fedora",
	"freebsd64Guest":          "freebsd",
	"freebsdGuest":            "freebsd",
	"genericLinuxGuest":       "linux",
	"mandrakeGuest":           "mandrake",
	"mandriva64Guest":         "mandriva",
	"mandrivaGuest":           "mandriva",
	"netware4Guest":           "netware4",
	"netware5Guest":           "netware5",
	"netware6Guest":           "netware6",
	"nld9Guest":               "nld9",
	"oesGuest":                "oes",
	"openServer5Guest":        "openServer5",
	"openServer6Guest":        "openServer6",
	"opensuse64Guest":         "opensuse",
	"opensuseGuest":           "opensuse",
	"oracleLinux6_64Guest":    "oracleLinux6",
	"oracleLinux64Guest":      "oracleLinux",
	"oracleLinux6Guest":       "oracleLinux6",
	"oracleLinux7_64Guest":    "oracleLinux7",
	"oracleLinux7Guest":       "oracleLinux7",
	"oracleLinuxGuest":        "oracleLinux",
	"os2Guest":                "os2",
	"other24xLinux64Guest":    "linux",
	"other24xLinuxGuest":      "linux",
	"other26xLinux64Guest":    "linux",
	"other26xLinuxGuest":      "linux",
	"other3xLinux64Guest":     "linux",
	"other3xLinuxGuest":       "linux",
	"otherGuest":              "other",
	"otherGuest64":            "other",
	"otherLinux64Guest":       "linux",
	"otherLinuxGuest":         "linux",
	"redhatGuest":             "rhel",
	"rhel2Guest":              "rhel2",
	"rhel3_64Guest":           "rhel3",
	"rhel3Guest":              "rhel3",
	"rhel4_64Guest":           "rhel4",
	"rhel4Guest":              "rhel4",
	"rhel5_64Guest":           "rhel5",
	"rhel5Guest":              "rhel5",
	"rhel6_64Guest":           "rhel6",
	"rhel6Guest":              "rhel6",
	"rhel7_64Guest":           "rhel7",
	"rhel7Guest":              "rhel7",
	"sjdsGuest":               "sjds",
	"sles10_64Guest":          "sles10",
	"sles10Guest":             "sles10",
	"sles11_64Guest":          "sles11",
	"sles11Guest":             "sles11",
	"sles12_64Guest":          "sles12",
	"sles12Guest":             "sles12",
	"sles64Guest":             "sles64",
	"slesGuest":               "sles",
	"solaris10_64Guest":       "solaris10",
	"solaris10Guest":          "solaris10",
	"solaris11_64Guest":       "solaris11",
	"solaris6Guest":           "solaris6",
	"solaris7Guest":           "solaris7",
	"solaris8Guest":           "solaris8",
	"solaris9Guest":           "solaris9",
	"suse64Guest":             "suse",
	"suseGuest":               "suse",
	"turboLinux64Guest":       "turbolinux",
	"turboLinuxGuest":         "turbolinux",
	"ubuntu64Guest":           "ubuntu",
	"ubuntuGuest":             "ubuntu",
	"unixWare7Guest":          "unixware7",
	"vmkernel5Guest":          "vmkernel5",
	"vmkernel65Guest":         "vmkernel65",
	"vmkernel6Guest":          "vmkernel6",
	"vmkernelGuest":           "vmkernel",
	"vmwarePhoton64Guest":     "photon",
	"win2000AdvServGuest":     "win2000advserv",
	"win2000ProGuest":         "win2000pro",
	"win2000ServGuest":        "win2000serv",
	"win31Guest":              "win31",
	"win95Guest":              "win95",
	"win98Guest":              "win98",
	"windows7_64Guest":        "win7",
	"windows7Guest":           "win7",
	"windows7Server64Guest":   "win7server",
	"windows8_64Guest":        "win8",
	"windows8Guest":           "win8",
	"windows8Server64Guest":   "win8server",
	"windows9_64Guest":        "win10",
	"windows9Guest":           "win10",
	"windows9Server64Guest":   "win10server",
	"windowsHyperVGuest":      "windowshyperv",
	"winLonghorn64Guest":      "winlonghorn",
	"winLonghornGuest":        "winlonghorn",
	"winMeGuest":              "winme",
	"winNetBusinessGuest":     "winnetbusiness",
	"winNetDatacenter64Guest": "winnetdatacenter",
	"winNetDatacenterGuest":   "winnetdatacenter",
	"winNetEnterprise64Guest": "winnetenterprise",
	"winNetEnterpriseGuest":   "winnetenterprise",
	"winNetStandard64Guest":   "winnetstandard",
	"winNetStandardGuest":     "winnetstandard",
	"winNetWebGuest":          "winnetweb",
	"winNTGuest":              "winnt",
	"winVista64Guest":         "winvista",
	"winVistaGuest":           "winvista",
	"winXPHomeGuest":          "winxphome",
	"winXPPro64Guest":         "winxppro",
	"winXPProGuest":           "winxppro",
}
