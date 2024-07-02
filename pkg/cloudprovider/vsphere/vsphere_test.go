/*
Copyright 2016 The Kubernetes Authors.

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

package vsphere

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	_ "github.com/vmware/govmomi/lookup/simulator"
	"github.com/vmware/govmomi/simulator"
	_ "github.com/vmware/govmomi/sts/simulator"
	_ "github.com/vmware/govmomi/vapi/simulator"

	ccfg "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/config"
	vcfg "k8s.io/cloud-provider-vsphere/pkg/common/config"
	cm "k8s.io/cloud-provider-vsphere/pkg/common/connectionmanager"
	"k8s.io/cloud-provider-vsphere/pkg/common/vclib"
)

// localhostCert was generated from crypto/tls/generate_cert.go with the following command:
//
//	go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var localhostCert = `-----BEGIN CERTIFICATE-----
MIIBjzCCATmgAwIBAgIRAKpi2WmTcFrVjxrl5n5YDUEwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAgFw03MDAxMDEwMDAwMDBaGA8yMDg0MDEyOTE2
MDAwMFowEjEQMA4GA1UEChMHQWNtZSBDbzBcMA0GCSqGSIb3DQEBAQUAA0sAMEgC
QQC9fEbRszP3t14Gr4oahV7zFObBI4TfA5i7YnlMXeLinb7MnvT4bkfOJzE6zktn
59zP7UiHs3l4YOuqrjiwM413AgMBAAGjaDBmMA4GA1UdDwEB/wQEAwICpDATBgNV
HSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MC4GA1UdEQQnMCWCC2V4
YW1wbGUuY29thwR/AAABhxAAAAAAAAAAAAAAAAAAAAABMA0GCSqGSIb3DQEBCwUA
A0EAUsVE6KMnza/ZbodLlyeMzdo7EM/5nb5ywyOxgIOCf0OOLHsPS9ueGLQX9HEG
//yjTXuhNcUugExIjM/AIwAZPQ==
-----END CERTIFICATE-----`

// localhostKey is the private key for localhostCert.
var localhostKey = `-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBAL18RtGzM/e3XgavihqFXvMU5sEjhN8DmLtieUxd4uKdvsye9Phu
R84nMTrOS2fn3M/tSIezeXhg66quOLAzjXcCAwEAAQJBAKcRxH9wuglYLBdI/0OT
BLzfWPZCEw1vZmMR2FF1Fm8nkNOVDPleeVGTWoOEcYYlQbpTmkGSxJ6ya+hqRi6x
goECIQDx3+X49fwpL6B5qpJIJMyZBSCuMhH4B7JevhGGFENi3wIhAMiNJN5Q3UkL
IuSvv03kaPR5XVQ99/UeEetUgGvBcABpAiBJSBzVITIVCGkGc7d+RCf49KTCIklv
bGWObufAR8Ni4QIgWpILjW8dkGg8GOUZ0zaNA6Nvt6TIv2UWGJ4v5PoV98kCIQDx
rIiZs5QbKdycsv9gQJzwQAogC8o04X3Zz3dsoX+h4A==
-----END RSA PRIVATE KEY-----`

// configFromSim starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with an empty tls.Config.
func configFromSim(multiDc bool) (*vcfg.Config, func()) {
	return configFromSimWithTLS(new(tls.Config), true, multiDc)
}

// configFromSimWithTLS starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with a tls.Config. The returned client
// config can be configured to allow/decline insecure connections.
func configFromSimWithTLS(tlsConfig *tls.Config, insecureAllowed bool, multiDc bool) (*vcfg.Config, func()) {
	cfg := &vcfg.Config{}
	model := simulator.VPX()

	if multiDc {
		model.Datacenter = 2
		model.Datastore = 1
		model.Cluster = 1
		model.Host = 0
	}

	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}

	// Adds vAPI, STS, Lookup Service endpoints to vcsim
	model.Service.RegisterEndpoints = true

	model.Service.TLS = tlsConfig
	s := model.Service.NewServer()

	cfg.Global.InsecureFlag = insecureAllowed

	cfg.Global.VCenterIP = s.URL.Hostname()
	cfg.Global.VCenterPort = s.URL.Port()
	cfg.Global.User = s.URL.User.Username()
	cfg.Global.Password, _ = s.URL.User.Password()

	if multiDc {
		cfg.Global.Datacenters = "DC0,DC1"
	} else {
		cfg.Global.Datacenters = vclib.TestDefaultDatacenter
	}
	cfg.VirtualCenter = make(map[string]*vcfg.VirtualCenterConfig)
	cfg.VirtualCenter[s.URL.Hostname()] = &vcfg.VirtualCenterConfig{
		User:             cfg.Global.User,
		Password:         cfg.Global.Password,
		TenantRef:        cfg.Global.VCenterIP,
		VCenterIP:        cfg.Global.VCenterIP,
		VCenterPort:      cfg.Global.VCenterPort,
		InsecureFlag:     cfg.Global.InsecureFlag,
		Datacenters:      cfg.Global.Datacenters,
		IPFamilyPriority: []string{"ipv4"},
	}

	// Configure region and zone categories
	cfg.Labels.Region = "k8s-region"
	cfg.Labels.Zone = "k8s-zone"

	return cfg, func() {
		s.Close()
		model.Remove()
	}
}

// configFromEnvOrSim builds a config from configFromSim and overrides using configFromEnv
func configFromEnvOrSim(multiDc bool) (*vcfg.Config, func()) {
	cfg, fin := configFromSim(multiDc)
	if err := cfg.FromEnv(); err != nil {
		return nil, nil
	}
	return cfg, fin
}

func TestNewVSphere(t *testing.T) {
	cfg := &ccfg.CPIConfig{}
	if err := cfg.FromCPIEnv(); err != nil {
		t.Skipf("No config found in environment")
	}

	_, err := newVSphere(cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
}

func TestVSphereLogin(t *testing.T) {
	initCfg, cleanup := configFromEnvOrSim(false)
	defer cleanup()
	cfg := &ccfg.CPIConfig{}
	cfg.Config = *initCfg

	// Create vSphere configuration object
	vs, err := newVSphere(cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
	vs.connectionManager = cm.NewConnectionManager(&cfg.Config, nil, nil)
	defer vs.connectionManager.Logout()

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create vSphere client
	vcInstance, ok := vs.connectionManager.VsphereInstanceMap[cfg.Global.VCenterIP]
	if !ok {
		t.Fatalf("Couldn't get vSphere instance: %s", cfg.Global.VCenterIP)
	}

	err = vcInstance.Conn.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vcInstance.Conn.Logout(ctx)
}

func TestVSphereLoginByToken(t *testing.T) {
	initCfg, cleanup := configFromEnvOrSim(false)
	defer cleanup()
	cfg := &ccfg.CPIConfig{}
	cfg.Config = *initCfg

	// Configure for SAML token auth
	cfg.Global.User = localhostCert
	cfg.Global.Password = localhostKey

	// Create vSphere configuration object
	vs, err := newVSphere(cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
	vs.connectionManager = cm.NewConnectionManager(&cfg.Config, nil, nil)

	ctx := context.Background()

	// Create vSphere client
	vcInstance, ok := vs.connectionManager.VsphereInstanceMap[cfg.Global.VCenterIP]
	if !ok {
		t.Fatalf("Couldn't get vSphere instance: %s", cfg.Global.VCenterIP)
	}

	err = vcInstance.Conn.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vcInstance.Conn.Logout(ctx)
}

func TestAlphaDualStackConfig(t *testing.T) {
	var testCases = []struct {
		testName               string
		conf                   string
		enableDualStackFeature bool
		expectedError          error
	}{
		{
			testName: "Verifying dual stack env var required when providing two ip families",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			[VirtualCenter "127.0.0.1"]
			user = user
			password = password
			ip-family = ipv6,ipv4`,
			enableDualStackFeature: false,
			expectedError:          fmt.Errorf("mulitple IP families specified for virtual center %q but ENABLE_ALPHA_DUAL_STACK env var is not set", "127.0.0.1"),
		},
		{
			testName: "Verifying dual stack env var existing when providing two ip families",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			[VirtualCenter "127.0.0.1"]
			user = user
			password = password
			ip-family = ipv6,ipv4`,
			enableDualStackFeature: true,
			expectedError:          nil,
		},
		{
			testName: "Dual stack env var not required when providing single ip family",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			[VirtualCenter "127.0.0.1"]
			user = user
			password = password
			ip-family = ipv6`,
			enableDualStackFeature: false,
			expectedError:          nil,
		},
	}
	for _, testcase := range testCases {
		t.Run(testcase.testName, func(t *testing.T) {
			cfg, err := ccfg.ReadCPIConfig([]byte(testcase.conf))
			if err != nil {
				t.Fatalf("error reading CPI config: %v", err)
			}

			if testcase.enableDualStackFeature {
				err := os.Setenv("ENABLE_ALPHA_DUAL_STACK", "1")
				if err != nil {
					t.Fatalf("Received error %s when setting env var ENABLE_ALPHA_DUAL_STACK", err)
				}
				defer func() {
					err := os.Unsetenv("ENABLE_ALPHA_DUAL_STACK")
					if err != nil {
						t.Fatalf("Received error %s when unsetting env var", err)
					}
				}()
			}

			_, err = buildVSphereFromConfig(cfg, nil, nil, nil)
			if !reflect.DeepEqual(err, testcase.expectedError) {
				t.Logf("actual error: %v", err)
				t.Logf("expected error: %v", err)
				t.Error("unexpected error")
			}
		})
	}
}

func TestSecretVSphereConfig(t *testing.T) {
	var vs *VSphere
	var (
		username = "user"
		password = "password"
	)
	var testcases = []struct {
		testName                 string
		conf                     string
		expectedIsSecretProvided bool
		expectedUsername         string
		expectedPassword         string
		expectedError            error
		expectedThumbprints      map[string]string
	}{
		{
			testName: "Username and password with old configuration",
			conf: `[Global]
			server = 0.0.0.0
			user = user
			password = password
			datacenters = us-west
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
		},
		{
			testName: "SecretName and SecretNamespace in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			datacenters = us-west
			secret-name = "vccreds"
			secret-namespace = "kube-system"
			`,
			expectedIsSecretProvided: true,
			expectedError:            nil,
		},
		{
			testName: "SecretName and SecretNamespace with Username and Password in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			user = user
			password = password
			datacenters = us-west
			secret-name = "vccreds"
			secret-namespace = "kube-system"
			`,
			expectedIsSecretProvided: true,
			expectedError:            nil,
		},
		{
			testName: "SecretName and SecretNamespace with Username missing in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			password = password
			datacenters = us-west
			secret-name = "vccreds"
			secret-namespace = "kube-system"
			`,
			expectedIsSecretProvided: true,
			expectedError:            nil,
		},
		{
			testName: "SecretNamespace missing with Username and Password in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			user = user
			password = password
			datacenters = us-west
			secret-name = "vccreds"
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
		},
		{
			testName: "SecretNamespace and Username missing in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			password = password
			datacenters = us-west
			secret-name = "vccreds"
			`,
			expectedPassword: password,
			expectedError:    vcfg.ErrUsernameMissing,
		},
		{
			testName: "SecretNamespace and Password missing in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			user = user
			datacenters = us-west
			secret-name = "vccreds"
			`,
			expectedUsername: username,
			expectedError:    vcfg.ErrPasswordMissing,
		},
		{
			testName: "SecretNamespace, Username and Password missing in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			datacenters = us-west
			secret-name = "vccreds"
			`,
			expectedError: vcfg.ErrUsernameMissing,
		},
		{
			testName: "Username and password with new configuration but username and password in global section",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			[VirtualCenter "0.0.0.0"]
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
		},
		{
			testName: "Username and password with new configuration, username and password in virtualcenter section",
			conf: `[Global]
			server = 0.0.0.0
			port = 443
			insecure-flag = true
			datacenters = us-west
			[VirtualCenter "0.0.0.0"]
			user = user
			password = password
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
		},
		{
			testName: "SecretName and SecretNamespace with new configuration",
			conf: `[Global]
			server = 0.0.0.0
			secret-name = "vccreds"
			secret-namespace = "kube-system"
			datacenters = us-west
			[VirtualCenter "0.0.0.0"]
			`,
			expectedIsSecretProvided: true,
			expectedError:            nil,
		},
		{
			testName: "SecretName and SecretNamespace with Username missing in new configuration",
			conf: `[Global]
			server = 0.0.0.0
			port = 443
			insecure-flag = true
			datacenters = us-west
			secret-name = "vccreds"
			secret-namespace = "kube-system"
			[VirtualCenter "0.0.0.0"]
			password = password
			`,
			expectedIsSecretProvided: true,
			expectedError:            nil,
		},
		{
			testName: "virtual centers with a thumbprint",
			conf: `[Global]
			server = global
			user = user
			password = password
			datacenters = us-west
			thumbprint = "thumbprint:global"
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
			expectedThumbprints: map[string]string{
				"global": "thumbprint:global",
			},
		},
		{
			testName: "Multiple virtual centers with different thumbprints",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			[VirtualCenter "0.0.0.0"]
			thumbprint = thumbprint:0
			[VirtualCenter "no_thumbprint"]
			[VirtualCenter "1.1.1.1"]
			thumbprint = thumbprint:1
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
			expectedThumbprints: map[string]string{
				"0.0.0.0": "thumbprint:0",
				"1.1.1.1": "thumbprint:1",
			},
		},
		{
			testName: "Multiple virtual centers use the global CA cert",
			conf: `[Global]
			user = user
			password = password
			datacenters = us-west
			ca-file = /some/path/to/my/trusted/ca.pem
			[VirtualCenter "0.0.0.0"]
			user = user
			password = password
			[VirtualCenter "1.1.1.1"]
			user = user
			password = password
			`,
			expectedUsername: username,
			expectedPassword: password,
			expectedError:    nil,
		},
	}

	for _, testcase := range testcases {
		t.Logf("Executing Testcase: %s", testcase.testName)
		cfg, err := ccfg.ReadCPIConfig([]byte(testcase.conf))
		if err != nil {
			if testcase.expectedError != nil {
				if err != testcase.expectedError {
					t.Fatalf("readConfig: expected err: %s, received err: %s", testcase.expectedError, err)
				} else {
					continue
				}
			} else {
				t.Fatalf("readConfig: unexpected error returned: %v", err)
			}
		}
		vs, err = buildVSphereFromConfig(cfg, nil, nil, nil)
		if err != nil { // testcase.expectedError {
			t.Fatalf("buildVSphereFromConfig: Should succeed when a valid config is provided: %v", err)
		}
		vs.connectionManager = cm.NewConnectionManager(&cfg.Config, nil, nil)

		if testcase.expectedIsSecretProvided && (vs.cfg.Global.SecretNamespace == "" || vs.cfg.Global.SecretName == "") {
			t.Fatalf("SecretName and SecretNamespace was expected in config %s. error: %s",
				testcase.conf, err)
		}
		if !testcase.expectedIsSecretProvided {
			for _, vsInstance := range vs.connectionManager.VsphereInstanceMap {
				if vsInstance.Conn.Username != testcase.expectedUsername {
					t.Fatalf("Expected username %s doesn't match actual username %s in config %s. error: %s",
						testcase.expectedUsername, vsInstance.Conn.Username, testcase.conf, err)
				}
				if vsInstance.Conn.Password != testcase.expectedPassword {
					t.Fatalf("Expected password %s doesn't match actual password %s in config %s. error: %s",
						testcase.expectedPassword, vsInstance.Conn.Password, testcase.conf, err)
				}
			}
		}
		// Check, if all the expected thumbprints are configured
		for instanceName, expectedThumbprint := range testcase.expectedThumbprints {
			instanceConfig, ok := vs.connectionManager.VsphereInstanceMap[instanceName]
			if !ok {
				t.Fatalf("Could not find configuration for instance %s", instanceName)
			}
			if actualThumbprint := instanceConfig.Conn.Thumbprint; actualThumbprint != expectedThumbprint {
				t.Fatalf(
					"Expected thumbprint for instance '%s' to be '%s', got '%s'",
					instanceName, expectedThumbprint, actualThumbprint,
				)
			}
		}
		// Check, if all connections are configured with the global CA certificate
		if expectedCaPath := cfg.Global.CAFile; expectedCaPath != "" {
			for name, instance := range vs.connectionManager.VsphereInstanceMap {
				if actualCaPath := instance.Conn.CACert; actualCaPath != expectedCaPath {
					t.Fatalf(
						"Expected CA certificate path for instance '%s' to be the globally configured one ('%s'), got '%s'",
						name, expectedCaPath, actualCaPath,
					)
				}
			}
		}
	}
}
