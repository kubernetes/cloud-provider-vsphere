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
	"crypto/md5" // #nosec
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"slices"
	"strconv"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vmop "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator"
	vmoptypes "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

const (
	// ClusterSelectorKey expects key/value pair {ClusterSelectorKey: <cluster name>} for target nodes: ClusterSelectorKey
	ClusterSelectorKey = "capv.vmware.com/cluster.name"
	// NodeSelectorKey expects key/value pair {NodeSelectorKey: NodeRole} for target nodes: NodeSelectorKey
	NodeSelectorKey = "capv.vmware.com/cluster.role"

	// LegacyClusterSelectorKey expects key/value pair {LegacyClusterSelectorKey: <cluster name>} for target nodes: LegacyClusterSelectorKey
	LegacyClusterSelectorKey = "capw.vmware.com/cluster.name"
	// LegacyNodeSelectorKey expects key/value pair {LegacyNodeSelectorKey: NodeRole} for target nodes: LegacyNodeSelectorKey
	LegacyNodeSelectorKey = "capw.vmware.com/cluster.role"

	// NodeRole is set by capw, we are targeting worker vms
	NodeRole = "node"

	// LabelClusterNameKey label should be added on virtual machine service with its corresponding k8s service
	LabelClusterNameKey = "run.tanzu.vmware.com/cluster.name"
	// LabelServiceNameKey label should be added on virtual machine service with its corresponding k8s service
	LabelServiceNameKey = "run.tanzu.vmware.com/service.name"
	// LabelServiceNameSpaceKey label should be added on virtual machine service with its corresponding k8s service
	LabelServiceNameSpaceKey = "run.tanzu.vmware.com/service.namespace"

	// AnnotationServiceExternalTrafficPolicyKey label is used to piggyback vSphere Paravirtual Service's
	// configuration to the supervisor cluster. AnnotationServiceExternalTrafficPolicyKey and AnnotationServiceHealthCheckNodePortKey are not part of
	// VirtualMachineService spec because they're K8s Service/Pod specific and
	// don't apply in a VirtualMachine context
	AnnotationServiceExternalTrafficPolicyKey = "virtualmachineservice.vmoperator.vmware.com/service.externalTrafficPolicy"
	// AnnotationServiceHealthCheckNodePortKey label is used to piggyback vSphere Paravirtual Service's
	// configuration to the supervisor cluster.
	AnnotationServiceHealthCheckNodePortKey = "virtualmachineservice.vmoperator.vmware.com/service.healthCheckNodePort"

	// AnnotationLastAppliedConfiguration is used by kubectl as a legacy mechanism to track changes.
	// That mechanism has been superseded by Server-side apply.
	AnnotationLastAppliedConfiguration = "kubectl.kubernetes.io/last-applied-configuration"

	// MaxCheckSumLen is the maximum length of vmservice suffix: vsphere paravirtual name length cannot exceed 41 bytes in total, so we need to make sure vmservice suffix is 21 bytes (63 - 41 -1 = 21)
	// https://gitlab.eng.vmware.com/core-build/guest-cluster-controller/blob/master/webhooks/validation/tanzukubernetescluster_validator.go#L56
	MaxCheckSumLen = 21
)

var excludedAnnotations = []string{
	AnnotationLastAppliedConfiguration,
	AnnotationServiceExternalTrafficPolicyKey,
	AnnotationServiceHealthCheckNodePortKey,
}

// A list of possible error messages
var (
	ErrCreateVMService     = errors.New("failed to create VirtualMachineService")
	ErrUpdateVMService     = errors.New("failed to update VirtualMachineService")
	ErrGetVMService        = errors.New("failed to get VirtualMachineService")
	ErrDeleteVMService     = errors.New("failed to delete VirtualMachineService")
	ErrVMServiceIPNotFound = errors.New("VirtualMachineService IP not found")
	ErrNodePortNotFound    = errors.New("NodePort not found")
)

var (
	// IsLegacy indicates whether legacy paravirtual mode is enabled
	// Default to false
	IsLegacy bool
)

// NewVMService creates a vmService object
func NewVMService(vmClient vmop.Interface, ns string, ownerRef *metav1.OwnerReference, serviceAnnotationPropagationEnabled bool) VMService {
	return &vmService{
		vmClient:                            vmClient,
		namespace:                           ns,
		ownerReference:                      ownerRef,
		serviceAnnotationPropagationEnabled: serviceAnnotationPropagationEnabled,
	}
}

func (s *vmService) hashString(str string) string {
	// #nosec
	hash := md5.New()
	if _, err := hash.Write([]byte(str)); err != nil {
		log.Error(err, "create hash string failed")
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// GetVMServiceName returns VirtualMachineService name for a lb type of service
func (s *vmService) GetVMServiceName(service *v1.Service, clusterName string) string {
	suffix := s.hashString(service.Name + "." + service.Namespace)
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(6).Info(fmt.Sprintf("Hash string for VirtualMachinService Name is %s", suffix))

	if len(suffix) > MaxCheckSumLen {
		suffix = suffix[:MaxCheckSumLen]
		logger.V(6).Info(fmt.Sprintf("Hash string for VirtualMachinService Name is truncated to %s", suffix))
	}
	return clusterName + "-" + suffix
}

// Get returns the corresponding virtual machine service if it exists
func (s *vmService) Get(ctx context.Context, service *v1.Service, clusterName string) (*vmoptypes.VirtualMachineServiceInfo, error) {
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(2).Info("Attempting to get VirtualMachineService")

	vms, err := s.vmClient.VirtualMachineServices().Get(ctx, s.namespace, s.GetVMServiceName(service, clusterName))
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		logger.Error(ErrGetVMService, fmt.Sprintf("%v", err))
		return nil, err
	}

	return vms, nil
}

// Create creates a vmservice to map to the given lb type of service, it should be called if vmservice not found
func (s *vmService) Create(ctx context.Context, service *v1.Service, clusterName string) (*vmoptypes.VirtualMachineServiceInfo, error) {
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(2).Info("Attempting to create VirtualMachineService")

	info, err := s.lbServiceToVMServiceInfo(service, clusterName)
	if err != nil {
		logger.Error(ErrCreateVMService, fmt.Sprintf("%v", err))
		return nil, err
	}

	created, err := s.vmClient.VirtualMachineServices().Create(ctx, info)
	if err != nil {
		logger.Error(ErrCreateVMService, fmt.Sprintf("%v", err))
		return nil, err
	}

	logger.V(2).Info("Successfully created VirtualMachineService")

	return created, nil
}

// CreateOrUpdate creates a vmservice to map to the given lb type of service
func (s *vmService) CreateOrUpdate(ctx context.Context, service *v1.Service, clusterName string) (*vmoptypes.VirtualMachineServiceInfo, error) {
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(2).Info("Attempting to create or update a VirtualMachineService")

	if clusterName == "" {
		logger.Error(ErrCreateVMService, "cluster name is required to create or update a vm service")
		return nil, errors.Wrapf(ErrCreateVMService, "cluster name cannot be empty")
	}

	vms, err := s.Get(ctx, service, clusterName)
	if err != nil {
		return nil, err
	}

	if vms == nil {
		// Create a new VirtualMachineService if not found
		vms, err = s.Create(ctx, service, clusterName)
		if err != nil {
			logger.Error(ErrCreateVMService, fmt.Sprintf("%v", err))
			return nil, err
		}
	} else {
		// Update the existing VirtualMachineService
		vms, err = s.Update(ctx, service, clusterName, vms)
		if err != nil {
			logger.Error(ErrUpdateVMService, fmt.Sprintf("%v", err))
			return nil, err
		}
	}

	if !loadBalancerIngressesSatisfied(service, vms) {
		return vms, ErrVMServiceIPNotFound
	}

	logger.V(2).Info("VirtualMachineService load balancer ingress is ready")

	return vms, err
}

// Update updates a vmservice
func (s *vmService) Update(ctx context.Context, service *v1.Service, clusterName string, existing *vmoptypes.VirtualMachineServiceInfo) (*vmoptypes.VirtualMachineServiceInfo, error) {
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(2).Info("Attempting to update VirtualMachineService")

	ports, err := findPorts(service)
	if err != nil {
		logger.Error(ErrUpdateVMService, fmt.Sprintf("%v", err))
		return nil, err
	}

	annotations := getVMServiceAnnotations(service, s.serviceAnnotationPropagationEnabled)

	// reflect.DeepEqual is used here intentionally: the compared types
	// ([]VirtualMachineServicePort, []string, map[string]string) contain only
	// plain value fields (no interfaces, no pointers, no unexported fields),
	// so DeepEqual is both correct and safe. The call frequency is low (one
	// per LoadBalancer reconcile), so the performance cost is acceptable.
	//
	// Normalize nil/empty slices and maps to nil before comparison to avoid
	// spurious updates. For example, the API server may return an empty slice
	// where the desired state has nil — both represent "not set". Normalising
	// to nil (rather than to an empty value) is consistent with the Kubernetes
	// convention that nil and empty are semantically equivalent for these fields.
	existingRanges := existing.Spec.LoadBalancerSourceRanges
	if len(existingRanges) == 0 {
		existingRanges = nil
	}
	serviceRanges := service.Spec.LoadBalancerSourceRanges
	if len(serviceRanges) == 0 {
		serviceRanges = nil
	}
	existingAnnotations := existing.Annotations
	if len(existingAnnotations) == 0 {
		existingAnnotations = nil
	}
	desiredAnnotations := annotations
	if len(desiredAnnotations) == 0 {
		desiredAnnotations = nil
	}
	existingIPFams := existing.Spec.IPFamilies
	if len(existingIPFams) == 0 {
		existingIPFams = nil
	}
	desiredIPFams := cloneIPFamilies(service.Spec.IPFamilies)
	servicePolicy := cloneIPFamilyPolicy(service.Spec.IPFamilyPolicy)

	var needsUpdate bool
	if !reflect.DeepEqual(existing.Spec.Ports, ports) {
		needsUpdate = true
	}
	if existing.Spec.LoadBalancerIP != service.Spec.LoadBalancerIP {
		needsUpdate = true
	}
	if !reflect.DeepEqual(existingRanges, serviceRanges) {
		needsUpdate = true
	}
	if !reflect.DeepEqual(existingAnnotations, desiredAnnotations) {
		needsUpdate = true
	}
	if !reflect.DeepEqual(existingIPFams, desiredIPFams) {
		needsUpdate = true
	}
	if !ipFamilyPolicyEqual(existing.Spec.IPFamilyPolicy, service.Spec.IPFamilyPolicy) {
		needsUpdate = true
	}

	if needsUpdate {
		update := &vmoptypes.VirtualMachineServiceInfo{
			Annotations: annotations,
			Spec: vmoptypes.VirtualMachineServiceSpec{
				Ports:                    ports,
				LoadBalancerIP:           service.Spec.LoadBalancerIP,
				LoadBalancerSourceRanges: serviceRanges,
				IPFamilies:               desiredIPFams,
				IPFamilyPolicy:           servicePolicy,
			},
		}
		result, err := s.vmClient.VirtualMachineServices().Update(ctx, s.namespace, existing.Name, update)
		if err != nil {
			logger.Error(ErrUpdateVMService, fmt.Sprintf("%v", err))
			return nil, err
		}

		logger.V(2).Info("Successfully updated VirtualMachineService")
		return result, nil
	}

	return existing, nil
}

// Delete deletes the vmservice mapped to the given lb type of service
func (s *vmService) Delete(ctx context.Context, service *v1.Service, clusterName string) error {
	logger := log.WithValues("name", service.Name, "namespace", service.Namespace)
	logger.V(2).Info("Attempting to delete VirtualMachineService")

	err := s.vmClient.VirtualMachineServices().Delete(ctx, s.namespace, s.GetVMServiceName(service, clusterName))
	if err != nil {
		logger.Error(ErrDeleteVMService, fmt.Sprintf("%v", err))
		return err
	}

	logger.V(2).Info("Successfully deleted VirtualMachineService")
	return nil
}

func findPorts(service *v1.Service) ([]vmoptypes.VirtualMachineServicePort, error) {
	ports := make([]vmoptypes.VirtualMachineServicePort, 0, len(service.Spec.Ports))
	for _, port := range service.Spec.Ports {
		if port.NodePort == 0 {
			return nil, errors.Wrapf(ErrNodePortNotFound, "port %s", port.Name)
		}
		ports = append(ports, vmoptypes.VirtualMachineServicePort{
			Name:       port.Name,
			Port:       port.Port,
			TargetPort: port.NodePort,
			Protocol:   string(port.Protocol),
		})
	}
	return ports, nil
}

func (s *vmService) lbServiceToVMServiceInfo(service *v1.Service, clusterName string) (*vmoptypes.VirtualMachineServiceInfo, error) {
	ports, err := findPorts(service)
	if err != nil {
		return nil, err
	}

	selector := map[string]string{
		ClusterSelectorKey: clusterName,
		NodeSelectorKey:    NodeRole,
	}
	if IsLegacy {
		selector = map[string]string{
			LegacyClusterSelectorKey: clusterName,
			LegacyNodeSelectorKey:    NodeRole,
		}
	}

	info := &vmoptypes.VirtualMachineServiceInfo{
		Name:      s.GetVMServiceName(service, clusterName),
		Namespace: s.namespace,
		Labels: map[string]string{
			LabelClusterNameKey:      clusterName,
			LabelServiceNameKey:      service.Name,
			LabelServiceNameSpaceKey: service.Namespace,
		},
		OwnerReferences: []metav1.OwnerReference{
			*s.ownerReference,
		},
		Spec: vmoptypes.VirtualMachineServiceSpec{
			Type:                     vmoptypes.VirtualMachineServiceTypeLoadBalancer,
			Ports:                    ports,
			Selector:                 selector,
			LoadBalancerIP:           service.Spec.LoadBalancerIP,
			LoadBalancerSourceRanges: service.Spec.LoadBalancerSourceRanges,
			IPFamilies:               cloneIPFamilies(service.Spec.IPFamilies),
			IPFamilyPolicy:           cloneIPFamilyPolicy(service.Spec.IPFamilyPolicy),
		},
	}

	if annotations := getVMServiceAnnotations(service, s.serviceAnnotationPropagationEnabled); len(annotations) != 0 {
		info.Annotations = annotations
	}

	return info, nil
}

func getVMServiceAnnotations(service *v1.Service, serviceAnnotationPropagationEnabled bool) map[string]string {
	var annotations map[string]string
	// When ExternalTrafficPolicy is set to Local in the Service, add its
	// value and the healthCheckNodePort to VirtualMachineService annotations.
	// When ExternalTrafficPolicy is set to Cluster, do nothing as that's
	// the default value, also there will be no HealthCheckNodePort
	// allocated in that case.
	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		annotations = make(map[string]string)
		annotations[AnnotationServiceExternalTrafficPolicyKey] = string(service.Spec.ExternalTrafficPolicy)
		annotations[AnnotationServiceHealthCheckNodePortKey] = strconv.Itoa(int(service.Spec.HealthCheckNodePort))
	}

	// Annotation propagation logic
	if serviceAnnotationPropagationEnabled {
		if annotations == nil {
			annotations = make(map[string]string)
		}
		for k, v := range service.Annotations {
			if !slices.Contains(excludedAnnotations, k) {
				annotations[k] = v
			}
		}
	}

	return annotations
}

func cloneIPFamilies(f []v1.IPFamily) []v1.IPFamily {
	if len(f) == 0 {
		return nil
	}
	out := make([]v1.IPFamily, len(f))
	copy(out, f)
	return out
}

func cloneIPFamilyPolicy(p *v1.IPFamilyPolicyType) *v1.IPFamilyPolicyType {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func ipFamilyPolicyEqual(a, b *v1.IPFamilyPolicyType) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// loadBalancerIngressesSatisfied returns true when status has at least one usable
// ingress IP, and when the Service declares specific IP families, each family
// has a matching ingress address.
func loadBalancerIngressesSatisfied(service *v1.Service, vms *vmoptypes.VirtualMachineServiceInfo) bool {
	wantV4, wantV6 := false, false
	for _, f := range service.Spec.IPFamilies {
		switch f {
		case v1.IPv4Protocol:
			wantV4 = true
		case v1.IPv6Protocol:
			wantV6 = true
		}
	}
	var hasV4, hasV6 bool
	for _, ing := range vms.Status.LoadBalancerIngress {
		if ing.IP == "" {
			continue
		}
		ip := net.ParseIP(ing.IP)
		if ip == nil {
			continue
		}
		if ip.To4() != nil {
			hasV4 = true
		} else {
			hasV6 = true
		}
	}
	if !wantV4 && !wantV6 {
		return hasV4 || hasV6
	}
	if wantV4 && !hasV4 {
		return false
	}
	if wantV6 && !hasV6 {
		return false
	}
	return true
}
