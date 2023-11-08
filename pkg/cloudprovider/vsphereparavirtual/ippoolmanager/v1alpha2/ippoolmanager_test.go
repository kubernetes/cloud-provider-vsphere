package v1alpha2

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	vpcnetworkingapis "github.com/vmware-tanzu/nsx-operator/pkg/apis/nsx.vmware.com/v1alpha2"
	fakevpcnetworkingclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	t1networkingapis "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/apis/nsxnetworking/v1alpha1"
	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/ippoolmanager/helper"
)

const (
	testClusterNameSpace = "test-guest-cluster-ns"
	testClustername      = "test-cluster"
	testNodeName1        = "fakeNode1"
	testNodeName2        = "fakeNode2"
)

func initIPPoolTest() (*IPPoolManager, *fakevpcnetworkingclients.Clientset) {
	ippoolclientset := fakevpcnetworkingclients.NewSimpleClientset()
	ippManager, _ := NewIPPoolManagerWithClients(ippoolclientset, testClusterNameSpace)

	ippoolclientset.ClearActions()
	return ippManager, ippoolclientset
}

func TestAddDeleteSubnetToIPPool(t *testing.T) {
	testcases := []struct {
		name                 string
		nodeToAdd            *corev1.Node
		nodeToDelete         *corev1.Node
		expectedSubs         int
		expectedPatches      int
		expectedUpdateIPPool bool
	}{
		{
			name: "Add 1 subnet to IPPool, remove the subnet from IPPool",
			nodeToAdd: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNodeName1,
				},
			},
			nodeToDelete: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNodeName1,
				},
			},
			expectedSubs:    0,
			expectedPatches: 2,
		},
		{
			name: "Add 1 subnet to IPPool, remove another subnet from IPPool",
			nodeToAdd: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNodeName1,
				},
			},
			nodeToDelete: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNodeName2,
				},
			},
			expectedSubs:    1,
			expectedPatches: 2,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			ipm, ippcs := initIPPoolTest()
			ipp, err := ipm.CreateIPPool(testClusterNameSpace, testClustername, &metav1.OwnerReference{})
			assert.Equal(t, nil, err)

			err = ipm.AddSubnetToIPPool(testCase.nodeToAdd, ipp, &metav1.OwnerReference{})
			assert.Equal(t, nil, err)
			ippools, err := ippcs.NsxV1alpha2().IPPools(testClusterNameSpace).List(context.Background(), metav1.ListOptions{})
			assert.Equal(t, 1, len(ippools.Items[0].Spec.Subnets))
			assert.Equal(t, testCase.nodeToAdd.Name, ippools.Items[0].Spec.Subnets[0].Name)
			assert.Equal(t, nil, err)

			err = ipm.DeleteSubnetFromIPPool(testCase.nodeToDelete.Name, &ippools.Items[0])
			assert.Equal(t, nil, err)
			actions := ippcs.Actions()
			numPatches := 0
			for _, a := range actions {
				if a.Matches("update", "ippools") {
					numPatches++
				}
			}
			assert.Equal(t, testCase.expectedPatches, numPatches)
			ippools, err = ippcs.NsxV1alpha2().IPPools(testClusterNameSpace).List(context.Background(), metav1.ListOptions{})
			assert.Equal(t, nil, err)
			assert.Equal(t, 1, len(ippools.Items))
			assert.Equal(t, testCase.expectedSubs, len(ippools.Items[0].Spec.Subnets))
		})
	}
}

func TestGetIPPoolSubnets(t *testing.T) {
	testcases := []struct {
		name         string
		ipp          helper.NSXIPPool
		expectedSubs map[string]string
		expectedErr  error
	}{
		{
			name: "There are two subnets realized in IPPool",
			ipp: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName2,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
						{
							CIDR: "10.10.2.0/24",
							Name: testNodeName2,
						},
					},
				},
			},
			expectedSubs: map[string]string{
				testNodeName1: "10.10.1.0/24",
				testNodeName2: "10.10.2.0/24",
			},
			expectedErr: nil,
		},
		{
			name: "There are no subnets realized in IPPool",
			ipp: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName2,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{},
				},
			},
			expectedSubs: map[string]string{},
			expectedErr:  nil,
		},
		{
			name: "IPPool type is wrong",
			ipp: &t1networkingapis.IPPool{
				Status: t1networkingapis.IPPoolStatus{
					Subnets: []t1networkingapis.SubnetResult{},
				},
			},
			expectedSubs: nil,
			expectedErr:  fmt.Errorf("unknown ippool type"),
		},
	}

	for _, testCase := range testcases {
		ipm, _ := initIPPoolTest()
		t.Run(testCase.name, func(t *testing.T) {
			subs, err := ipm.GetIPPoolSubnets(testCase.ipp)
			assert.Equal(t, testCase.expectedSubs, subs)
			assert.Equal(t, testCase.expectedErr, err)
		})
	}
}

func TestDiffIPPoolSubnets(t *testing.T) {
	testcases := []struct {
		name         string
		old          helper.NSXIPPool
		cur          helper.NSXIPPool
		expectedDiff bool
	}{
		{
			name: "IPPool with new subnet realized",
			old: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
					},
				},
			},
			cur: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName2,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
						{
							CIDR: "10.10.2.0/24",
							Name: testNodeName2,
						},
					},
				},
			},
			expectedDiff: true,
		},
		{
			name: "IPPool with new subnet still not realized",
			old: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
					},
				},
			},
			cur: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName2,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
					},
				},
			},
			expectedDiff: false,
		},
		{
			name: "IPPool with no new subnet created",
			old: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
					},
				},
			},
			cur: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
				Status: vpcnetworkingapis.IPPoolStatus{
					Subnets: []vpcnetworkingapis.SubnetResult{
						{
							CIDR: "10.10.1.0/24",
							Name: testNodeName1,
						},
					},
				},
			},
			expectedDiff: false,
		},
		{
			name: "IPPool with no subnet realized",
			old: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
			},
			cur: &vpcnetworkingapis.IPPool{
				Spec: vpcnetworkingapis.IPPoolSpec{
					Subnets: []vpcnetworkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "ipv4",
							Name:         testNodeName1,
						},
					},
				},
			},
			expectedDiff: false,
		},
		{
			name: "IPPool with wrong type used",
			old: &t1networkingapis.IPPool{
				Spec: t1networkingapis.IPPoolSpec{
					Subnets: []t1networkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "IPv4",
							Name:         testNodeName1,
						},
					},
				},
			},
			cur: &t1networkingapis.IPPool{
				Spec: t1networkingapis.IPPoolSpec{
					Subnets: []t1networkingapis.SubnetRequest{
						{
							PrefixLength: 24,
							IPFamily:     "IPv4",
							Name:         testNodeName1,
						},
					},
				},
			},
			expectedDiff: false,
		},
	}

	for _, testCase := range testcases {
		ipm, _ := initIPPoolTest()
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedDiff, ipm.DiffIPPoolSubnets(testCase.old, testCase.cur))
		})
	}
}
