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

package config

import (
	"testing"
)

/*
	TODO:
	When the INI based cloud-config is deprecated. This file should be deleted.
*/

func TestReadINIConfig(t *testing.T) {
	testCases := []struct {
		name     string
		contents string
	}{
		{
			name: "valid INI config",
			contents: `
			[Route]
			router-path = /infra/tier-1s/test-router
			`,
		},
		{
			name: "INI config with extra key/value pairs",
			contents: `
			[Route]
			router-path = /infra/tier-1s/test-router
			extra-key = value
			routers-path = /infra/tier-2s/test-router
			`,
		},
	}
	for _, testCase := range testCases {
		config, err := ReadRawConfigINI([]byte(testCase.contents))
		if err != nil {
			t.Error(err)
			return
		}

		assertEquals := func(name, left, right string) {
			if left != right {
				t.Errorf("%s %s != %s", name, left, right)
			}
		}
		assertEquals("Route.routerPath", config.Route.RouterPath, "/infra/tier-1s/test-router")
	}
}
