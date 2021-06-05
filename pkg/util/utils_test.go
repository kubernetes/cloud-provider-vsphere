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

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsIPv4(t *testing.T) {
	testCases := []struct {
		name           string
		testIP         string
		expectedResult bool
	}{
		{
			name:           "valid IPv4 address",
			testIP:         "100.96.1.0/24",
			expectedResult: true,
		},
		{
			name:           "empty IP address",
			testIP:         "",
			expectedResult: false,
		},
		{
			name:           "invalid IPv4 address",
			testIP:         "fe80::20c:29ff:fe0b:b407/64",
			expectedResult: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedResult, IsIPv4(testCase.testIP))
		})
	}
}
