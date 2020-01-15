/*
 Copyright 2020 The Kubernetes Authors.

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

package loadbalancer

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/types"

	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"
)

// Tags is a map of NSXT-T tags indexed by the tag scope
type Tags map[string]model.Tag

// ByScope is an array of sags sortable by tag scope
type ByScope []model.Tag

func (a ByScope) Len() int           { return len(a) }
func (a ByScope) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByScope) Less(i, j int) bool { return strings.Compare(*a[i].Scope, *a[j].Scope) < 0 }

// Append clones the Tags and optionally adds additional tags
func (m Tags) Append(tags ...model.Tag) Tags {
	result := Tags{}
	for n, t := range m {
		result[n] = t
	}
	for _, t := range tags {
		result[*t.Scope] = t
	}
	return result
}

// Normalize returns a tag array sorted by scopes
func (m Tags) Normalize() []model.Tag {
	result := make(ByScope, len(m))
	cnt := 0
	for _, t := range m {
		result[cnt] = t
		cnt++
	}
	sort.Sort(result)
	return result
}

func newTag(scope, tag string) model.Tag {
	return model.Tag{Scope: &scope, Tag: &tag}
}

func clusterTag(clusterName string) model.Tag {
	return newTag(ScopeCluster, clusterName)
}

func serviceTag(objectName types.NamespacedName) model.Tag {
	return newTag(ScopeService, objectName.String())
}

func portTag(mapping Mapping) model.Tag {
	return newTag(ScopePort, fmt.Sprintf("%s/%d", mapping.Protocol, mapping.SourcePort))
}

func checkTags(tags []model.Tag, required ...model.Tag) bool {
outer:
	for _, req := range required {
		for _, tag := range tags {
			if *tag.Scope == *req.Scope {
				if *tag.Tag != *req.Tag {
					return false
				}
				continue outer
			}
		}
		return false
	}
	return true
}

func getTag(tags []model.Tag, scope string) string {
	for _, tag := range tags {
		if *tag.Scope == scope {
			return *tag.Tag
		}
	}
	return ""
}
