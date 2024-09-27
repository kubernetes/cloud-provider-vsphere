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
	"errors"
	"testing"
)

const invalidFormat = `
This is just a string that is neither a yaml or ini file
`

func TestReadConfig(t *testing.T) {
	mockConfigTestCases := []struct {
		name          string
		configData    string
		expectedError error
	}{
		{
			name:       "Valid YAML",
			configData: basicConfigYAML,
		},
		{
			name:          "Invalid YAML",
			configData:    badConfigYAML,
			expectedError: errors.New("ReadConfig failed.  YAML=[yaml: line 5: mapping values are not allowed in this context], INI=[2:1: expected section header]"),
		},
		{
			name:       "Valid INI",
			configData: basicConfigINI,
		},
		{
			name:          "Invalid INI",
			configData:    badConfigINI,
			expectedError: errors.New("ReadConfig failed.  YAML=[yaml: unmarshal errors:\n  line 2: cannot unmarshal !!seq into config.CommonConfigYAML], INI=[2:9: expected EOL, EOF, or comment]"),
		},
		{
			name:          "Invalid format",
			configData:    invalidFormat,
			expectedError: errors.New("ReadConfig failed.  YAML=[yaml: unmarshal errors:\n  line 2: cannot unmarshal !!str `This is...` into config.CommonConfigYAML], INI=[2:1: expected section header]"),
		},
		{
			name:          "Missing vCenter IP YAML",
			configData:    missingServerConfigYAML,
			expectedError: errors.New("No Virtual Center hosts defined"),
		},
		{
			name:          "Missing vCenter IP INI",
			configData:    missingServerConfigINI,
			expectedError: errors.New("No Virtual Center hosts defined"),
		},
	}

	for _, tc := range mockConfigTestCases {
		t.Run(tc.name, func(t *testing.T) {

			cfg, err := ReadConfig([]byte(tc.configData))

			if tc.expectedError != nil {
				if err == nil {
					t.Fatal("ReadConfig was expected to return error")
				}
				if err.Error() != tc.expectedError.Error() {
					t.Fatalf("Expected: %v, got %v", tc.expectedError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("ReadConfig was not expected to return error: %v", err)
				}

				if cfg == nil {
					t.Fatal("ReadConfig did not return a config object")
				}
			}
		})
	}
}
