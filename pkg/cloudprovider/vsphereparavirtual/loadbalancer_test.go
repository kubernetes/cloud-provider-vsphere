/*
Copyright 2021 The Kubernetes Authors.

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

package vsphereparavirtual

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/klog/v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	cloudprovider "k8s.io/cloud-provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice"
	util "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice/testutil"

	"github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"
)

var (
	testClusterNameSpace = "test-guest-cluster-ns"
	testClustername      = "test-cluster"
	testOwnerReference   = metav1.OwnerReference{
		APIVersion: "v1alpha1",
		Kind:       "TanzuKubernetesCluster",
		Name:       testClustername,
		UID:        "1bbf49a7-fbce-4502-bb4c-4c3544cacc9e",
	}
)

func TestLoadBalancer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "LoadBalancer Suite")
}

var _ = Describe("LoadBalancer Support", func() {
	var (
		lb                      cloudprovider.LoadBalancer
		testK8sService          *v1.Service
		testK8sServiceName      string
		testK8sServiceNameSpace string

		// FakeClientWrapper allows functions to be replaced for fault injection
		fcw *util.FakeClientWrapper
	)

	newTestLoadBalancer := func() (cloudprovider.LoadBalancer, error) {
		scheme := runtime.NewScheme()
		_ = vmopv1alpha1.AddToScheme(scheme)
		fc := fakeClient.NewFakeClientWithScheme(scheme)
		fcw = util.NewFakeClientWrapper(fc)
		vms := vmservice.NewVMService(fcw, testClusterNameSpace, &testOwnerReference)
		return &loadBalancer{
			vmService: vms,
		}, nil
	}

	BeforeEach(func() {
		lb, _ = newTestLoadBalancer()
		testK8sServiceName = "test-lb-service"
		testK8sServiceNameSpace = "test-service-ns"
	})

	Describe("NewLoadBalancer", func() {
		Context("when everything is ok", func() {
			var (
				err     error
				cfg     *rest.Config
				testEnv *envtest.Environment
			)

			It("should not return error", func() {
				testEnv = &envtest.Environment{}
				cfg, err = testEnv.Start()
				Expect(err).ShouldNot(HaveOccurred())

				_, err = NewLoadBalancer(testClusterNameSpace, cfg, &testOwnerReference)
				if err != nil {
					klog.Infof("nicole, error is ", err)
				} else {
					klog.Infof("nicole, lb error is nothing")
				}
				Expect(err).ShouldNot(HaveOccurred())

				err = testEnv.Stop()
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("GetLoadBalancer", func() {
		var (
			err    error
			exists bool
		)

		JustBeforeEach(func() {
			_, exists, err = lb.GetLoadBalancer(context.Background(), testClustername, testK8sService)
		})

		Context("when VMService is created", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
				_, err := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
				Expect(err).Should(Equal(vmservice.ErrVMServiceIPNotFound))
			})
			AfterEach(func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should not return error", func() {
				Expect(exists).Should(Equal(true))
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when VMService is not found", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})

			It("should return error", func() {
				Expect(exists).Should(Equal(false))
				Expect(err).Should(HaveOccurred())
			})
		})
	})

	Describe("UpdateLoadBalancer", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = lb.UpdateLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
		})

		Context("when GetVMService failed", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			It("should return error", func() {
				// Error should be NotFound during the Get() call
				Expect(err).Should(HaveOccurred())
				Expect(apierrors.IsNotFound(err))
			})
		})

		Context("when VMService update failed", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}

				// Add the service with no ports
				_, err := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
				Expect(err).Should(Equal(vmservice.ErrVMServiceIPNotFound))

				// Update the service definition to add ports
				testK8sService.Spec = v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test-port",
							Port:     80,
							NodePort: 30900,
							Protocol: "TCP",
						},
					},
				}

				// Ensure that the client Update call returns an error on update
				fcw.UpdateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
					return fmt.Errorf("Some undefined update error")
				}
			})
			It("should return error", func() {
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when VMService is updated", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}

				// Add the service with no ports
				_, err := lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
				Expect(err).Should(Equal(vmservice.ErrVMServiceIPNotFound))

				// Update the service definition to add ports
				testK8sService.Spec = v1.ServiceSpec{
					Ports: []v1.ServicePort{
						{
							Name:     "test-port",
							Port:     80,
							NodePort: 30900,
							Protocol: "TCP",
						},
					},
				}
			})
			It("should not return error", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
		})
	})

	Describe("EnsureLoadBalancer", func() {
		var (
			status    *v1.LoadBalancerStatus
			ensureErr error
		)

		JustBeforeEach(func() {
			status, ensureErr = lb.EnsureLoadBalancer(context.Background(), testClustername, testK8sService, []*v1.Node{})
		})

		Context("when service configured ExternalTrafficPolicy to be local", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
					Spec: v1.ServiceSpec{
						ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
					},
				}
				fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
					vms := &vmopv1alpha1.VirtualMachineService{
						Status: vmopv1alpha1.VirtualMachineServiceStatus{
							LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
								Ingress: []v1alpha1.LoadBalancerIngress{
									{
										IP: "10.10.10.10",
									},
								},
							},
						},
					}
					vms.DeepCopyInto(obj.(*vmopv1alpha1.VirtualMachineService))
					return nil
				}

			})
			AfterEach(func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should not return error", func() {
				Expect(ensureErr).ShouldNot(HaveOccurred())
			})
		})

		Context("when VMService is created but IP not found", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			AfterEach(func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should return IP not found error", func() {
				Expect(ensureErr).Should(Equal(vmservice.ErrVMServiceIPNotFound))
			})
		})

		Context("when VMService creation failed", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
				// Ensure that the client Create call will fail. The preceding Get call should return NotFound
				fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
					return fmt.Errorf("failed to create VirtualMachineService")
				}
			})
			AfterEach(func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should return error", func() {
				Expect(ensureErr).Should(HaveOccurred())
			})
		})

		Context("when VMService created and IP found", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
				// Ensure that the client Create call returns a VMService with a valid IP
				fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
					vms := &vmopv1alpha1.VirtualMachineService{
						Status: vmopv1alpha1.VirtualMachineServiceStatus{
							LoadBalancer: vmopv1alpha1.LoadBalancerStatus{
								Ingress: []v1alpha1.LoadBalancerIngress{
									{
										IP: "10.10.10.10",
									},
								},
							},
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "test-vm-service-name",
							OwnerReferences: []metav1.OwnerReference{
								testOwnerReference,
							},
						},
						Spec: vmopv1alpha1.VirtualMachineServiceSpec{
							Type: vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
							Ports: []v1alpha1.VirtualMachineServicePort{
								{
									Name:       "test-port",
									Port:       80,
									TargetPort: 30800,
									Protocol:   "TCP",
								},
							},
							Selector: map[string]string{
								vmservice.ClusterSelectorKey: testClustername,
								vmservice.NodeSelectorKey:    vmservice.NodeRole,
							},
						},
					}
					vms.DeepCopyInto(obj.(*vmopv1alpha1.VirtualMachineService))
					return nil
				}
			})
			AfterEach(func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should not return error", func() {
				Expect(ensureErr).ShouldNot(HaveOccurred())
				Expect(status.Ingress[0].IP).Should(Equal("10.10.10.10"))
			})
		})

		Context("when delete loadbalancer", func() {
			BeforeEach(func() {
				testK8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			It("should pass without error", func() {
				err := lb.EnsureLoadBalancerDeleted(context.Background(), testClustername, testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should ignore not found error", func() {
				fcw.DeleteFunc = func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
					return apierrors.NewNotFound(v1alpha1.Resource("virtualmachineservice"), testClustername)
				}
				err := lb.EnsureLoadBalancerDeleted(context.Background(), "test", testK8sService)
				Expect(err).ShouldNot(HaveOccurred())
			})
			It("should return error", func() {
				fcw.DeleteFunc = func(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
					return fmt.Errorf("an error occurred while deleting load balancer")
				}
				err := lb.EnsureLoadBalancerDeleted(context.Background(), "test", testK8sService)
				Expect(err).To(MatchError("an error occurred while deleting load balancer"))
			})
		})
	})
})
