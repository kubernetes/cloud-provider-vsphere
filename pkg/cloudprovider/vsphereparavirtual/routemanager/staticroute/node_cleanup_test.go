/*
Copyright The Kubernetes Authors.

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

package staticroute

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	vpcapisv1 "github.com/vmware-tanzu/nsx-operator/pkg/apis/vpc/v1alpha1"
	nsxclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned"
	fakeclients "github.com/vmware-tanzu/nsx-operator/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/routemanager/helper"
)

func TestDeleteRouteCRPreservesDeleteSemantics(t *testing.T) {
	for _, tc := range []struct {
		name    string
		failure error
	}{
		{"success", nil},
		{"NotFound remains visible to caller", apierrors.NewNotFound(schema.GroupResource{Resource: "staticroutes"}, "worker")},
		{"API error remains visible to caller", fmt.Errorf("delete unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeclients.NewSimpleClientset()
			client.PrependReactor("delete", "staticroutes", func(action clienttesting.Action) (bool, runtime.Object, error) {
				deletion := action.(clienttesting.DeleteAction)
				require.Equal(t, "worker", deletion.GetName())
				require.Equal(t, testClusterNameSpace, deletion.GetNamespace())
				require.Equal(t, metav1.DeleteOptions{}, deletion.GetDeleteOptions())
				return true, nil, tc.failure
			})
			manager, err := NewRouteManagerWithClients(client, testClusterNameSpace)
			require.NoError(t, err)
			err = manager.DeleteRouteCR("worker")
			if tc.failure == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tc.failure)
			}
			require.Len(t, client.Actions(), 1)
		})
	}
}

type cleanupRoundTripper func(*http.Request) (*http.Response, error)

func (rt cleanupRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

func TestDeleteNodeRoutesPreservesContext(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "node cleanup")
	requests := 0
	client, err := nsxclients.NewForConfigAndClient(&rest.Config{Host: "https://supervisor.invalid"}, &http.Client{
		Transport: cleanupRoundTripper(func(req *http.Request) (*http.Response, error) {
			requests++
			require.Equal(t, "node cleanup", req.Context().Value(contextKey{}))
			if req.Method == http.MethodGet {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"apiVersion":"crd.nsx.vmware.com/v1alpha1","kind":"StaticRouteList","items":[{"metadata":{"name":"worker","uid":"route-uid"},"spec":{"network":"10.0.0.0/24"}}]}`)),
				}, nil
			}
			require.Equal(t, http.MethodDelete, req.Method)
			return nil, context.Canceled
		}),
	})
	require.NoError(t, err)
	manager, err := NewRouteManagerWithClients(client, testClusterNameSpace)
	require.NoError(t, err)
	require.ErrorIs(t, manager.DeleteNodeRoutes(ctx, testClustername, "worker"), context.Canceled)
	require.Equal(t, 2, requests, "both list and delete must preserve the caller context")
}

func cleanupRoute(name, cidr, cluster, ownerNode string) *vpcapisv1.StaticRoute {
	route := &vpcapisv1.StaticRoute{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testClusterNameSpace, UID: types.UID(name), Labels: map[string]string{helper.LabelKeyClusterName: cluster}},
		Spec:       vpcapisv1.StaticRouteSpec{Network: cidr},
	}
	if ownerNode != "" {
		route.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: ownerNode}}
	}
	return route
}

func TestDeleteNodeRoutes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		ownerNode string
	}{{"with Node owner", "worker"}, {"legacy without Node owner", ""}} {
		t.Run(tc.name, func(t *testing.T) {
			pending := cleanupRoute("worker", "10.0.0.0/24", testClustername, tc.ownerNode)
			failed := cleanupRoute("worker-ipv6", "fd00::/64", testClustername, tc.ownerNode)
			failed.Status.Conditions = []vpcapisv1.StaticRouteCondition{{Type: vpcapisv1.Ready, Status: corev1.ConditionFalse}}
			live := cleanupRoute("live-worker", "10.0.1.0/24", testClustername, "live-worker")
			foreign := cleanupRoute("other-cluster-route", "10.0.2.0/24", "other-cluster", "worker")
			allocation := &vpcapisv1.IPAddressAllocation{ObjectMeta: metav1.ObjectMeta{Name: "live-worker", Namespace: testClusterNameSpace}}
			client := fakeclients.NewSimpleClientset(pending, failed, live, foreign, allocation)
			manager, err := NewRouteManagerWithClients(client, testClusterNameSpace)
			require.NoError(t, err)
			require.NoError(t, manager.DeleteNodeRoutes(context.Background(), testClustername, "worker"))
			routes, err := client.CrdV1alpha1().StaticRoutes(testClusterNameSpace).List(context.Background(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, routes.Items, 2)
			require.ElementsMatch(t, []string{"live-worker", "other-cluster-route"}, []string{routes.Items[0].Name, routes.Items[1].Name})
			_, err = client.CrdV1alpha1().IPAddressAllocations(testClusterNameSpace).Get(context.Background(), "live-worker", metav1.GetOptions{})
			require.NoError(t, err)
			for _, action := range client.Actions() {
				if deletion, ok := action.(clienttesting.DeleteAction); ok {
					require.NotNil(t, deletion.GetDeleteOptions().Preconditions)
					require.Equal(t, types.UID(deletion.GetName()), *deletion.GetDeleteOptions().Preconditions.UID)
				}
			}
			require.NoError(t, manager.DeleteNodeRoutes(context.Background(), testClustername, "worker"))
		})
	}
}

func TestDeleteNodeRoutesIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, routeName, cidr, ownerNode, deletedNode string
		wantDeleted                                   bool
	}{
		{"IPv4 suffix node survives prefix node deletion", "worker-ipv6", "10.0.0.0/24", "", "worker", false},
		{"IPv4 suffix node deletion", "worker-ipv6", "10.0.0.0/24", "", "worker-ipv6", true},
		{"IPv6 suffix node deletion", "worker-ipv6-ipv6", "fd00::/64", "", "worker-ipv6", true},
		{"different explicit owner survives", "worker", "10.0.0.0/24", "other-worker", "worker", false},
		{"explicit owner supports invalid network cleanup", "worker", "invalid", "worker", "worker", true},
		{"invalid legacy network cannot establish identity", "worker", "invalid", "", "worker", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			route := cleanupRoute(tc.routeName, tc.cidr, testClustername, tc.ownerNode)
			client := fakeclients.NewSimpleClientset(route)
			manager, err := NewRouteManagerWithClients(client, testClusterNameSpace)
			require.NoError(t, err)
			require.NoError(t, manager.DeleteNodeRoutes(context.Background(), testClustername, tc.deletedNode))
			_, err = client.CrdV1alpha1().StaticRoutes(testClusterNameSpace).Get(context.Background(), route.Name, metav1.GetOptions{})
			if tc.wantDeleted {
				require.True(t, apierrors.IsNotFound(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDeleteNodeRoutesErrors(t *testing.T) {
	for _, tc := range []struct {
		name, verb string
		failure    error
		wantError  bool
	}{
		{"list error", "list", fmt.Errorf("list unavailable"), true},
		{"delete error", "delete", fmt.Errorf("delete unavailable"), true},
		{"concurrent deletion", "delete", apierrors.NewNotFound(schema.GroupResource{Resource: "staticroutes"}, "worker"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := fakeclients.NewSimpleClientset(cleanupRoute("worker", "10.0.0.0/24", testClustername, "worker"))
			client.PrependReactor(tc.verb, "staticroutes", func(clienttesting.Action) (bool, runtime.Object, error) { return true, nil, tc.failure })
			manager, err := NewRouteManagerWithClients(client, testClusterNameSpace)
			require.NoError(t, err)
			err = manager.DeleteNodeRoutes(context.Background(), testClustername, "worker")
			if tc.wantError {
				require.ErrorIs(t, err, tc.failure)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
