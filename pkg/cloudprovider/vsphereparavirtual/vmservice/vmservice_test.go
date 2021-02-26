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

package vmservice

import (
	"context"
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/api/node/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	rest "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeClient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	vmopv1alpha1 "github.com/vmware-tanzu/vm-operator-api/api/v1alpha1"

	util "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmservice/testutil"
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
	vms                     VMService
	testK8sService          *v1.Service
	testK8sServiceName      string
	testK8sServiceNameSpace string
	fakeLBIP                = "1.1.1.1"

	// FakeClientWrapper allows functions to be replaced for fault injection
	fcw *util.FakeClientWrapper
)

var _ = Describe("NewVMService", func() {
	var (
		err     error
		cfg     *rest.Config
		testEnv *envtest.Environment
		client  client.Client
	)

	BeforeEach(func() {
		testEnv = &envtest.Environment{}
		cfg, err = testEnv.Start()
		Expect(err).ShouldNot(HaveOccurred())
	})
	AfterEach(func() {
		err = testEnv.Stop()
		Expect(err).ShouldNot(HaveOccurred())
	})

	Context("when everything is ok", func() {
		It("should not return error", func() {
			client, err = GetVmopClient(cfg)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(client).ShouldNot(BeNil())
			realVms := NewVMService(client, testClusterNameSpace, &testOwnerReference)
			Expect(realVms).ShouldNot(BeNil())
		})
	})
})

var _ = Describe("VMService Operations", func() {
	var (
		k8sService *v1.Service
	)

	BeforeSuite(func() {
		testK8sServiceName = "test-lb-service"
		testK8sServiceNameSpace = "test-service-ns"
	})

	BeforeEach(func() {
		testK8sService = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testK8sServiceName,
				Namespace: testK8sServiceNameSpace,
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:     "http",
						Protocol: "tcp",
						Port:     80,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: 80,
						},
						NodePort: 30800,
					},
				},
			},
		}
		scheme := runtime.NewScheme()
		_ = vmopv1alpha1.AddToScheme(scheme)
		fc := fakeClient.NewFakeClientWithScheme(scheme)
		fcw = util.NewFakeClientWrapper(fc)
		vms = NewVMService(fcw, testClusterNameSpace, &testOwnerReference)
	})

	Describe("Get VMService name", func() {
		var (
			expectedName string
			name         string
		)

		JustBeforeEach(func() {
			name = vms.GetVMServiceName(k8sService, testClustername)
			hashStr := vms.(*vmService).hashString(testK8sServiceName + "." + testK8sServiceNameSpace)
			expectedName = testClustername + "-" + hashStr[:MaxCheckSumLen]
		})

		Context("when everything is good", func() {
			BeforeEach(func() {
				k8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			It("should return expected name", func() {
				Expect(name).Should(Equal(expectedName))
			})
		})
	})

	Describe("Get VMService", func() {
		var (
			vmService        *vmopv1alpha1.VirtualMachineService
			createdVMService *vmopv1alpha1.VirtualMachineService
			err              error
		)

		JustBeforeEach(func() {
			vmService, err = vms.Get(context.Background(), k8sService, testClustername)
		})

		Context("when failed to get VMService", func() {
			BeforeEach(func() {
				k8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			It("should return nil", func() {
				Expect(vmService).Should(BeNil())
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when successfully get VMService", func() {
			BeforeEach(func() {
				k8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
				// create a fake VMService
				createdVMService, _ = vms.Create(context.Background(), k8sService, testClustername)
			})
			AfterEach(func() {
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})
			It("should return VMService", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect((*vmService).Spec).Should(Equal((*createdVMService).Spec))
			})
		})
	})

	Describe("Create VMService", func() {
		var (
			vmServiceObj *vmopv1alpha1.VirtualMachineService
			expectedSpec vmopv1alpha1.VirtualMachineServiceSpec
			err          error
		)

		Context("when nodeport is zero", func() {
			BeforeEach(func() {
				k8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{
								Name:     "http",
								Protocol: "tcp",
								Port:     80,
								TargetPort: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 80,
								},
							},
						},
					},
				}
				vmServiceObj, err = vms.Create(context.Background(), k8sService, testClustername)
			})
			It("should return error", func() {
				Expect(vmServiceObj).Should(BeNil())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when VMService creation fails", func() {
			BeforeEach(func() {
				vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)
			})
			It("should return error", func() {
				// Try to create the same object twice
				vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)
				Expect(vmServiceObj).Should(BeNil())
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when successfully created VMService", func() {
			BeforeEach(func() {
				ports, _ := findPorts(testK8sService)
				expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
					Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
					Ports: ports,
					Selector: map[string]string{
						ClusterSelectorKey: testClustername,
						NodeSelectorKey:    NodeRole,
					},
				}

				vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)

			})
			AfterEach(func() {
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})
			It("should return VMService", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
			})
		})

		Context("when Service has loadBalancerIP defined", func() {
			BeforeEach(func() {
				testK8sService.Spec.LoadBalancerIP = fakeLBIP
				ports, _ := findPorts(testK8sService)
				expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
					Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
					Ports: ports,
					Selector: map[string]string{
						ClusterSelectorKey: testClustername,
						NodeSelectorKey:    NodeRole,
					},
					LoadBalancerIP: fakeLBIP,
				}

				vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)

			})
			AfterEach(func() {
				testK8sService.Spec.LoadBalancerIP = ""
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})
			It("should return VMService with LoadBalancerIP specified", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
			})
		})

		Context("when Service has LoadBalancerSourceRanges defined", func() {
			BeforeEach(func() {
				testK8sService.Spec.LoadBalancerSourceRanges = []string{"1.1.1.0/24", "10.0.0.0/19"}
				ports, _ := findPorts(testK8sService)
				expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
					Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
					Ports: ports,
					Selector: map[string]string{
						ClusterSelectorKey: testClustername,
						NodeSelectorKey:    NodeRole,
					},
					LoadBalancerSourceRanges: []string{"1.1.1.0/24", "10.0.0.0/19"},
				}

				vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)

			})
			AfterEach(func() {
				testK8sService.Spec.LoadBalancerSourceRanges = []string{}
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})
			It("should return VMService with LoadBalancerSourceRanges specified", func() {
				Expect(err).ShouldNot(HaveOccurred())
				Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
			})
		})

		Context("when Service has externalTrafficPolicy defined", func() {
			Context("when externalTrafficPolicy is set to Local", func() {
				BeforeEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
					testK8sService.Spec.HealthCheckNodePort = 30012
					vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)

				})
				AfterEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = ""
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should return VMService with ExternalTrafficPolicy and HealthCheckNodePort specified in annotations", func() {
					Expect(err).ShouldNot(HaveOccurred())
					v, ok := vmServiceObj.Annotations[AnnotationServiceExternalTrafficPolicyKey]
					Expect(ok).To(BeTrue())
					Expect(v).To(Equal(string(v1.ServiceExternalTrafficPolicyTypeLocal)))
					hcPort, ok := vmServiceObj.Annotations[AnnotationServiceHealthCheckNodePortKey]
					Expect(ok).To(BeTrue())
					Expect(hcPort).To(Equal(strconv.Itoa(30012)))
				})
			})
			Context("when externalTrafficPolicy is set to Cluster", func() {
				BeforeEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
					vmServiceObj, err = vms.Create(context.Background(), testK8sService, testClustername)

				})
				AfterEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = ""
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should return VMService without ExternalTrafficPolicy and HealthCheckNodePort specified in annotations", func() {
					Expect(err).ShouldNot(HaveOccurred())
					_, ok := vmServiceObj.Annotations[AnnotationServiceExternalTrafficPolicyKey]
					Expect(ok).NotTo(BeTrue())
					_, ok = vmServiceObj.Annotations[AnnotationServiceHealthCheckNodePortKey]
					Expect(ok).NotTo(BeTrue())
				})
			})
		})
	})

	Describe("CreateOrUpdate VMService", func() {
		var (
			vmServiceObj *vmopv1alpha1.VirtualMachineService
			expectedSpec vmopv1alpha1.VirtualMachineServiceSpec
			err          error
		)

		Context("when VMService does not exist", func() {
			BeforeEach(func() {
				k8sService = &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
				}
			})
			It("should create VMService", func() {
				vmServiceObj, err = vms.CreateOrUpdate(context.Background(), k8sService, testClustername)

				Expect(err).Should(HaveOccurred())
				Expect(err).Should(Equal(ErrVMServiceIPNotFound))
			})
		})

		Context("when clusterName is empty", func() {
			It("should update VMService and return IP not found ", func() {
				vmServiceObj, err = vms.CreateOrUpdate(context.Background(), testK8sService, "")
				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when get VMServicefails", func() {
			It("should return error", func() {
				// Redefine Get in the client to return an error
				fcw.GetFunc = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return fmt.Errorf("failed to get VirtualMachineService")
				}

				vmServiceObj, err = vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when VMService creation fails", func() {
			It("should return error", func() {
				// Redefine Get in the client to return an error
				fcw.GetFunc = func(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
					return apierrors.NewNotFound(v1alpha1.Resource("virtualmachineservice"), testClustername)
				}
				fcw.CreateFunc = func(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
					return fmt.Errorf("failed to create VirtualMachineService")
				}
				_, err = vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)

				Expect(err).Should(HaveOccurred())
			})
		})

		Context("when VMService already exists", func() {
			BeforeEach(func() {
				oldK8sService := &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testK8sServiceName,
						Namespace: testK8sServiceNameSpace,
					},
					Spec: v1.ServiceSpec{
						Ports: []v1.ServicePort{
							{
								Name:     "http",
								Protocol: "tcp",
								Port:     80,
								TargetPort: intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: 80,
								},
								NodePort: 30500,
							},
						},
					},
				}

				ports, _ := findPorts(testK8sService)
				expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
					Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
					Ports: ports,
					Selector: map[string]string{
						ClusterSelectorKey: testClustername,
						NodeSelectorKey:    NodeRole,
					},
				}
				// create an old VMService
				_, _ = vms.Create(context.Background(), oldK8sService, testClustername)
			})
			AfterEach(func() {
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})
			It("should update VMService and return IP not found ", func() {
				vmServiceObj, err = vms.CreateOrUpdate(context.Background(), testK8sService, testClustername)

				Expect(err).Should(Equal(ErrVMServiceIPNotFound))
				Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
			})
		})
	})

	Describe("Update VMService", func() {
		var (
			vmServiceObj     *vmopv1alpha1.VirtualMachineService
			createdVMService *vmopv1alpha1.VirtualMachineService
			expectedSpec     vmopv1alpha1.VirtualMachineServiceSpec
			err              error
			oldK8sService    *v1.Service
		)

		BeforeEach(func() {
			oldK8sService = testK8sService.DeepCopy()
		})

		JustBeforeEach(func() {
			vmServiceObj, err = vms.Update(context.Background(), testK8sService, testClustername, createdVMService)
		})

		Context("when k8s service does not change", func() {
			BeforeEach(func() {
				createdVMService, _ = vms.Create(context.Background(), testK8sService, testClustername)
			})
			AfterEach(func() {
				_ = vms.Delete(context.Background(), testK8sService, testClustername)
			})

			It("should not update VMService", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when k8s service changes", func() {
			Context("when node port changes", func() {
				BeforeEach(func() {
					oldK8sService.Spec.Ports[0].NodePort = 30500
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
					}
					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
				})
			})
			Context("when load balancer IP is added", func() {
				BeforeEach(func() {
					testK8sService.Spec.LoadBalancerIP = fakeLBIP
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
						LoadBalancerIP: fakeLBIP,
					}
					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
				})
			})
			Context("when load balancer IP changes", func() {
				BeforeEach(func() {
					testK8sService.Spec.LoadBalancerIP = fakeLBIP
					oldK8sService.Spec.LoadBalancerIP = "2.2.2.2"
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
						LoadBalancerIP: fakeLBIP,
					}
					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
				})
			})
			Context("when load balancer source ranges is added", func() {
				BeforeEach(func() {
					testK8sService.Spec.LoadBalancerSourceRanges = []string{"1.1.1.0/24"}
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
						LoadBalancerSourceRanges: []string{"1.1.1.0/24"},
					}
					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
				})
			})
			Context("when load balancer source ranges changes", func() {
				BeforeEach(func() {
					testK8sService.Spec.LoadBalancerSourceRanges = []string{"1.1.1.0/24"}
					oldK8sService.Spec.LoadBalancerSourceRanges = []string{"2.2.2.0/24"}
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
						LoadBalancerSourceRanges: []string{"1.1.1.0/24"},
					}
					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
				})
			})
			Context("when external traffic policy is set to local after creation", func() {
				var (
					expectedAnnotations map[string]string
				)
				BeforeEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
					testK8sService.Spec.HealthCheckNodePort = 31234
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
					}
					expectedAnnotations = map[string]string{
						AnnotationServiceExternalTrafficPolicyKey: string(v1.ServiceExternalTrafficPolicyTypeLocal),
						AnnotationServiceHealthCheckNodePortKey:   "31234",
					}

					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
					Expect((*vmServiceObj).Annotations).Should(Equal(expectedAnnotations))
				})
			})
			Context("when external traffic policy is set to cluster from local", func() {
				var (
					expectedAnnotations map[string]string
				)
				BeforeEach(func() {
					testK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeCluster
					testK8sService.Spec.HealthCheckNodePort = 31234
					oldK8sService.Spec.ExternalTrafficPolicy = v1.ServiceExternalTrafficPolicyTypeLocal
					ports, _ := findPorts(testK8sService)
					expectedSpec = vmopv1alpha1.VirtualMachineServiceSpec{
						Type:  vmopv1alpha1.VirtualMachineServiceTypeLoadBalancer,
						Ports: ports,
						Selector: map[string]string{
							ClusterSelectorKey: testClustername,
							NodeSelectorKey:    NodeRole,
						},
					}

					// create an old VMService
					createdVMService, _ = vms.Create(context.Background(), oldK8sService, testClustername)
				})
				AfterEach(func() {
					_ = vms.Delete(context.Background(), testK8sService, testClustername)
				})
				It("should update VMService", func() {
					Expect((*vmServiceObj).Spec).Should(Equal(expectedSpec))
					Expect((*vmServiceObj).Annotations).Should(Equal(expectedAnnotations))
				})
			})
		})
	})

	Describe("Delete VMService", func() {
		var (
			err error
		)

		JustBeforeEach(func() {
			err = vms.Delete(context.Background(), testK8sService, testClustername)
		})

		Context("when VMService deletion is successful", func() {
			BeforeEach(func() {
				_, _ = vms.Create(context.Background(), testK8sService, testClustername)
			})

			It("should return no error", func() {
				Expect(err).ShouldNot(HaveOccurred())
			})
		})

	})
})
