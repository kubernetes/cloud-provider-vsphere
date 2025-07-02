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

package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestReadOwnerRef(t *testing.T) {
	tests := []struct {
		fileExists bool
		apiVersion string
		kind       string
		name       string
		uid        string
	}{
		{
			true,
			"v1alpha1",
			"TanzuKubernetesCluster",
			"my-cluster",
			"798ea504-0a4d-4e3b-a67c-77812c89071c",
		},
		{
			true,
			"",
			"",
			"",
			"",
		},
		{
			false,
			"",
			"",
			"",
			"",
		},
	}

	for _, test := range tests {

		if test.fileExists {
			tmpfile, err := os.CreateTemp("", "TestReadOwnerRef")
			if err != nil {
				t.Errorf("Should be able to create tmpfile: %s", err)
			}
			defer os.Remove(tmpfile.Name()) // clean up

			ref := &metav1.OwnerReference{
				APIVersion: test.apiVersion,
				Kind:       test.kind,
				Name:       test.name,
				UID:        types.UID(test.uid),
			}
			content, _ := json.Marshal(ref)

			if _, err := tmpfile.Write(content); err != nil {
				t.Errorf("Should be able to write to tmpfile: %s", err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Errorf("Should be able to write to tmpfile: %s", err)
			}
			ownerRef, err := ReadOwnerRef(tmpfile.Name())
			if err != nil {
				t.Fatalf("Should succeed when a valid config is provided: %s", err)
			}

			if ownerRef.APIVersion != test.apiVersion {
				t.Errorf("incorrect apiversion: %s", ownerRef.APIVersion)
			}
			if ownerRef.Kind != test.kind {
				t.Errorf("incorrect kind: %s", ownerRef.Kind)
			}
			if ownerRef.Name != test.name {
				t.Errorf("incorrect name: %s", ownerRef.Name)
			}
			if string(ownerRef.UID) != test.uid {
				t.Errorf("incorrect uid: %s", ownerRef.UID)
			}
		} else {
			_, err := ReadOwnerRef("non-exists")
			if err == nil {
				t.Errorf("Should fail when an invalid config is provided")
			}
		}
	}
}

func TestReadSupervisorConfig(t *testing.T) {
	endpoint := "test.sv.proxy"
	port := "6443"

	err := os.Setenv(SupervisorAPIServerEndpointIPEnv, endpoint)
	if err != nil {
		t.Errorf("Should be able to set env var: %s", err)
	}

	err = os.Setenv(SupervisorAPIServerPortEnv, port)
	if err != nil {
		t.Errorf("Should be able to set env var: %s", err)
	}

	defer os.Setenv(SupervisorAPIServerEndpointIPEnv, "") // clean up
	defer os.Setenv(SupervisorAPIServerPortEnv, "")       // clean up

	svEndpoint, _ := readSupervisorConfig()

	if svEndpoint.Endpoint != endpoint {
		t.Fatalf("incorrect endpoint: %s", svEndpoint.Endpoint)
	}
	if svEndpoint.Port != port {
		t.Fatalf("incorrect port: %s", svEndpoint.Port)
	}

}

func TestGetNameSpace(t *testing.T) {
	tests := []struct {
		fileExists bool
		namespace  string
	}{
		{
			fileExists: false,
			namespace:  "",
		},
		{
			fileExists: true,
			namespace:  "test-ns",
		},
	}

	for _, test := range tests {

		if test.fileExists {
			dir, _ := os.Getwd()
			tmpfile, err := os.Create(dir + "/" + SupervisorClusterAccessNamespaceFile)
			if err != nil {
				t.Errorf("Should be able to create tmpfile: %s", err)
			}
			defer os.Remove(tmpfile.Name()) // clean up

			if _, err := tmpfile.Write([]byte(test.namespace)); err != nil {
				t.Errorf("Should be able to write to tmpfile: %s", err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Errorf("Should be able to write to tmpfile: %s", err)
			}
			ns, err := GetNameSpace(dir)
			if err != nil {
				t.Fatalf("Should succeed when a valid SV endpoint config is provided: %s", err)
			}
			if ns != test.namespace {
				t.Fatalf("incorrect namespace: %s", ns)
			}
		} else {
			_, err := GetNameSpace("non-exits")
			if err == nil {
				t.Errorf("Should fail when an invalid supervisor config is provided")
			}
		}
	}
}

func TestGetRestConfig(t *testing.T) {
	tests := []struct {
		fileExists bool
		fqdn       string
		endpoint   string
		port       string
		token      string
		ca         string
	}{
		{
			fileExists: false,
			fqdn:       "supervisor.default.svc",
			endpoint:   "192.163.1.100",
			port:       "6443",
			token:      "test-token",
			ca:         "test-ca",
		},
		{
			fileExists: true,
			fqdn:       "supervisor.default.svc",
			endpoint:   "192.163.1.200",
			port:       "6443",
			token:      "test-token",
			ca:         "test-ca",
		},
	}

	for _, test := range tests {
		dir, _ := os.Getwd()

		if test.fileExists {
			err := createTestFile(dir, SupervisorClusterAccessTokenFile, test.token)
			defer os.Remove(dir + "/" + SupervisorClusterAccessTokenFile)
			if err != nil {
				t.Errorf("failed to create test token file, %s", err)
			}

			err = createTestFile(dir, SupervisorClusterAccessCAFile, test.ca)
			defer os.Remove(dir + "/" + SupervisorClusterAccessCAFile)
			if err != nil {
				t.Errorf("failed to create test ca file, %s", err)
			}

			err = os.Setenv(SupervisorAPIServerEndpointIPEnv, test.endpoint)
			if err != nil {
				t.Errorf("Should be able to set env var: %s", err)
			}

			err = os.Setenv(SupervisorAPIServerPortEnv, test.port)
			if err != nil {
				t.Errorf("Should be able to set env var: %s", err)
			}

			defer os.Setenv(SupervisorAPIServerEndpointIPEnv, "") // clean up
			defer os.Setenv(SupervisorAPIServerPortEnv, "")       // clean up

			cfg, err := GetRestConfig(dir)
			if err != nil {
				t.Fatalf("Should succeed when a valid SV endpoint config is provided: %s", err)
			}
			if cfg.Host != "https://"+net.JoinHostPort(test.fqdn, test.port) {
				t.Fatalf("incorrect Host: %s", cfg.Host)
			}
			if cfg.BearerToken != test.token {
				t.Fatalf("incorrect Token: %s", cfg.BearerToken)
			}
		} else {
			err := os.Setenv(SupervisorAPIServerEndpointIPEnv, test.endpoint)
			if err != nil {
				t.Errorf("Should be able to set env var: %s", err)
			}

			err = os.Setenv(SupervisorAPIServerPortEnv, test.port)
			if err != nil {
				t.Errorf("Should be able to set env var: %s", err)
			}

			defer os.Setenv(SupervisorAPIServerEndpointIPEnv, "") // clean up
			defer os.Setenv(SupervisorAPIServerPortEnv, "")       // clean up

			_, err = GetRestConfig(dir)
			if err == nil {
				t.Errorf("Should fail when an invalid supervisor config is provided")
			}
		}
	}
}

func TestCheckPodIPPoolType(t *testing.T) {
	tests := []struct {
		vpcModeEnabled   bool
		podIPPoolType    string
		expectedErrorMsg string
		name             string
	}{
		{
			name:             "If VPC mode is not enabled, --pod-ip-pool-type should be empty",
			vpcModeEnabled:   false,
			podIPPoolType:    "",
			expectedErrorMsg: "",
		},
		{
			name:             "If VPC mode is not enabled, throw out error if --pod-ip-pool-type is not empty",
			vpcModeEnabled:   false,
			podIPPoolType:    "test-ns",
			expectedErrorMsg: "--pod-ip-pool-type can be set only when the network is VPC",
		},
		{
			name:             "If VPC mode is enabled, throw error if --pod-ip-pool-type is not Public or Private",
			vpcModeEnabled:   true,
			podIPPoolType:    "test-ns",
			expectedErrorMsg: "--pod-ip-pool-type can be either Public or Private in NSX-T VPC network, test-ns is not supported",
		},
		{
			name:             "If VPC mode is enabled, throw error if --pod-ip-pool-type is empty",
			vpcModeEnabled:   true,
			podIPPoolType:    "",
			expectedErrorMsg: "--pod-ip-pool-type is required in the NSX-T VPC network",
		},
		{
			name:             "Pod IP Pool type should be successfully set as Public",
			vpcModeEnabled:   true,
			podIPPoolType:    "Public",
			expectedErrorMsg: "",
		},
		{
			name:             "Pod IP Pool type should be successfully set as Private",
			vpcModeEnabled:   true,
			podIPPoolType:    "Private",
			expectedErrorMsg: "",
		},
	}

	for _, test := range tests {
		err := CheckPodIPPoolType(test.vpcModeEnabled, test.podIPPoolType)
		if test.expectedErrorMsg == "" {
			assert.Equal(t, err, nil)
		} else {
			assert.Equal(t, err.Error(), test.expectedErrorMsg)
		}
	}
}

func createTestFile(dir, filename, content string) error {
	tmpFile, err := os.Create(dir + "/" + filename)
	if err != nil {
		return fmt.Errorf("Should be able to create tmpfile: %s", err)
	}

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		return fmt.Errorf("Should be able to write to tmpTokenfile: %s", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("Should be able to write to tmpTokenfile: %s", err)
	}

	return nil
}
