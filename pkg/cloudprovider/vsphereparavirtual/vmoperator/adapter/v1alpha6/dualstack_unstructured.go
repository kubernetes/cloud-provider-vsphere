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

package v1alpha6

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/vmoperator/types"
)

// readDualStackFromUnstructured populates hub dual-stack fields from the raw object.
// VM Operator v1alpha6 Go types may lag the CRD; these keys match core Service JSON.
func readDualStackFromUnstructured(u map[string]interface{}, spec *types.VirtualMachineServiceSpec) {
	families, found, err := unstructured.NestedStringSlice(u, "spec", "ipFamilies")
	if !found || err != nil || len(families) == 0 {
		spec.IPFamilies = nil
	} else {
		spec.IPFamilies = make([]corev1.IPFamily, len(families))
		for i, f := range families {
			spec.IPFamilies[i] = corev1.IPFamily(f)
		}
	}
	pol, found, err := unstructured.NestedString(u, "spec", "ipFamilyPolicy")
	if !found || err != nil || pol == "" {
		spec.IPFamilyPolicy = nil
	} else {
		p := corev1.IPFamilyPolicyType(pol)
		spec.IPFamilyPolicy = &p
	}
}

// writeDualStackToUnstructured sets or clears spec.ipFamilies and spec.ipFamilyPolicy on the object map.
func writeDualStackToUnstructured(obj map[string]interface{}, spec *types.VirtualMachineServiceSpec) {
	if len(spec.IPFamilies) == 0 && spec.IPFamilyPolicy == nil {
		unstructured.RemoveNestedField(obj, "spec", "ipFamilies")
		unstructured.RemoveNestedField(obj, "spec", "ipFamilyPolicy")
		return
	}
	if len(spec.IPFamilies) > 0 {
		fams := make([]interface{}, len(spec.IPFamilies))
		for i, f := range spec.IPFamilies {
			fams[i] = string(f)
		}
		_ = unstructured.SetNestedSlice(obj, fams, "spec", "ipFamilies")
	} else {
		unstructured.RemoveNestedField(obj, "spec", "ipFamilies")
	}
	if spec.IPFamilyPolicy != nil {
		_ = unstructured.SetNestedField(obj, string(*spec.IPFamilyPolicy), "spec", "ipFamilyPolicy")
	} else {
		unstructured.RemoveNestedField(obj, "spec", "ipFamilyPolicy")
	}
}
