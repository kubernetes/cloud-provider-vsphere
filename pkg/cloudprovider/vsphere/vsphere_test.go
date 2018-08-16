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
	"log"
	"os"
	"strconv"
	"strings"
	"testing"

	lookup "github.com/vmware/govmomi/lookup/simulator"
	"github.com/vmware/govmomi/simulator"
	"github.com/vmware/govmomi/simulator/vpx"
	sts "github.com/vmware/govmomi/sts/simulator"

	"k8s.io/cloud-provider-vsphere/pkg/vclib"
)

// localhostCert was generated from crypto/tls/generate_cert.go with the following command:
//     go run generate_cert.go  --rsa-bits 512 --host 127.0.0.1,::1,example.com --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
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

func configFromEnv() (cfg Config, ok bool) {
	var InsecureFlag bool
	var err error
	cfg.Global.VCenterIP = os.Getenv("VSPHERE_VCENTER")
	cfg.Global.VCenterPort = os.Getenv("VSPHERE_VCENTER_PORT")
	cfg.Global.User = os.Getenv("VSPHERE_USER")
	cfg.Global.Password = os.Getenv("VSPHERE_PASSWORD")
	cfg.Global.Datacenters = os.Getenv("VSPHERE_DATACENTER")
	cfg.Network.PublicNetwork = os.Getenv("VSPHERE_PUBLIC_NETWORK")
	//cfg.Global.DefaultDatastore = os.Getenv("VSPHERE_DATASTORE")
	//cfg.Disk.SCSIControllerType = os.Getenv("VSPHERE_SCSICONTROLLER_TYPE")
	//cfg.Global.WorkingDir = os.Getenv("VSPHERE_WORKING_DIR")
	//cfg.Global.VMName = os.Getenv("VSPHERE_VM_NAME")
	if os.Getenv("VSPHERE_INSECURE") != "" {
		InsecureFlag, err = strconv.ParseBool(os.Getenv("VSPHERE_INSECURE"))
	} else {
		InsecureFlag = false
	}
	if err != nil {
		log.Fatal(err)
	}
	cfg.Global.InsecureFlag = InsecureFlag

	ok = (cfg.Global.VCenterIP != "" &&
		cfg.Global.User != "")

	return
}

// getVMUUID allows tests to override GetVMUUID
//var getVMUUID = GetVMUUID

// configFromSim starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with an empty tls.Config.
func configFromSim() (Config, func()) {
	return configFromSimWithTLS(new(tls.Config), true)
}

// configFromSimWithTLS starts a vcsim instance and returns config for use against the vcsim instance.
// The vcsim instance is configured with a tls.Config. The returned client
// config can be configured to allow/decline insecure connections.
func configFromSimWithTLS(tlsConfig *tls.Config, insecureAllowed bool) (Config, func()) {
	var cfg Config
	model := simulator.VPX()

	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}

	model.Service.TLS = tlsConfig
	s := model.Service.NewServer()

	// STS simulator
	path, handler := sts.New(s.URL, vpx.Setting)
	model.Service.ServeMux.Handle(path, handler)

	// Lookup Service simulator
	model.Service.RegisterSDK(lookup.New())

	cfg.Global.InsecureFlag = insecureAllowed

	cfg.Global.VCenterIP = s.URL.Hostname()
	cfg.Global.VCenterPort = s.URL.Port()
	cfg.Global.User = s.URL.User.Username()
	cfg.Global.Password, _ = s.URL.User.Password()
	cfg.Global.Datacenters = vclib.TestDefaultDatacenter
	cfg.Network.PublicNetwork = vclib.TestDefaultNetwork
	//cfg.Global.DefaultDatastore = vclib.TestDefaultDatastore
	//cfg.Disk.SCSIControllerType = os.Getenv("VSPHERE_SCSICONTROLLER_TYPE")
	//cfg.Global.WorkingDir = os.Getenv("VSPHERE_WORKING_DIR")
	//cfg.Global.VMName = os.Getenv("VSPHERE_VM_NAME")

	//if cfg.Global.WorkingDir == "" {
	//	cfg.Global.WorkingDir = "vm" // top-level Datacenter.VmFolder
	//}

	//uuid := simulator.Map.Any("VirtualMachine").(*simulator.VirtualMachine).Config.Uuid
	//getVMUUID = func() (string, error) { return uuid, nil }

	cfg.VirtualCenter = make(map[string]*VirtualCenterConfig)
	cfg.VirtualCenter[s.URL.Hostname()] = &VirtualCenterConfig{}

	return cfg, func() {
		//getVMUUID = GetVMUUID
		s.Close()
		model.Remove()
	}
}

// configFromEnvOrSim returns config from configFromEnv if set, otherwise returns configFromSim.
func configFromEnvOrSim() (Config, func()) {
	cfg, ok := configFromEnv()
	if ok {
		return cfg, func() {}
	}
	return configFromSim()
}

func TestReadConfig(t *testing.T) {
	_, err := readConfig(nil)
	if err == nil {
		t.Errorf("Should fail when no config is provided: %s", err)
	}

	cfg, err := readConfig(strings.NewReader(`
[Global]
server = 0.0.0.0
port = 443
user = user
password = password
insecure-flag = true
datacenters = us-west
ca-file = /some/path/to/a/ca.pem
`))
	//vm-uuid = 1234
	//vm-name = vmname
	if err != nil {
		t.Fatalf("Should succeed when a valid config is provided: %s", err)
	}

	if cfg.Global.VCenterIP != "0.0.0.0" {
		t.Errorf("incorrect vcenter ip: %s", cfg.Global.VCenterIP)
	}

	if cfg.Global.Datacenters != "us-west" {
		t.Errorf("incorrect datacenter: %s", cfg.Global.Datacenters)
	}

	// if cfg.Global.VMUUID != "1234" {
	// 	t.Errorf("incorrect vm-uuid: %s", cfg.Global.VMUUID)
	// }

	// if cfg.Global.VMName != "vmname" {
	// 	t.Errorf("incorrect vm-name: %s", cfg.Global.VMName)
	// }

	if cfg.Global.CAFile != "/some/path/to/a/ca.pem" {
		t.Errorf("incorrect ca-file: %s", cfg.Global.CAFile)
	}
}

func TestNewVSphere(t *testing.T) {
	cfg, ok := configFromEnv()
	if !ok {
		t.Skipf("No config found in environment")
	}

	_, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}
}

func TestVSphereLogin(t *testing.T) {
	cfg, cleanup := configFromEnvOrSim()
	defer cleanup()

	// Create vSphere configuration object
	vs, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create vSphere client
	vcInstance, ok := vs.vsphereInstanceMap[cfg.Global.VCenterIP]
	if !ok {
		t.Fatalf("Couldn't get vSphere instance: %s", cfg.Global.VCenterIP)
	}

	err = vcInstance.conn.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vcInstance.conn.Logout(ctx)
}

func TestVSphereLoginByToken(t *testing.T) {
	cfg, cleanup := configFromSim()
	defer cleanup()

	// Configure for SAML token auth
	cfg.Global.User = localhostCert
	cfg.Global.Password = localhostKey

	// Create vSphere configuration object
	vs, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}

	ctx := context.Background()

	// Create vSphere client
	vcInstance, ok := vs.vsphereInstanceMap[cfg.Global.VCenterIP]
	if !ok {
		t.Fatalf("Couldn't get vSphere instance: %s", cfg.Global.VCenterIP)
	}

	err = vcInstance.conn.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vcInstance.conn.Logout(ctx)
}

/*
func TestVSphereLoginWithCaCert(t *testing.T) {
	caCertPEM, err := ioutil.ReadFile(fixtures.CaCertPath)
	if err != nil {
		t.Fatalf("Could not read ca cert from file")
	}

	serverCert, err := tls.LoadX509KeyPair(fixtures.ServerCertPath, fixtures.ServerKeyPath)
	if err != nil {
		t.Fatalf("Could not load server cert and server key from files: %#v", err)
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caCertPEM); !ok {
		t.Fatalf("Cannot add CA to CAPool")
	}

	tlsConfig := tls.Config{
		Certificates: []tls.Certificate{serverCert},
		RootCAs:      certPool,
	}

	cfg, cleanup := configFromSimWithTLS(&tlsConfig, false)
	defer cleanup()

	cfg.Global.CAFile = fixtures.CaCertPath

	// Create vSphere configuration object
	vs, err := newVSphere(cfg)
	if err != nil {
		t.Fatalf("Failed to construct/authenticate vSphere: %s", err)
	}

	ctx := context.Background()

	// Create vSphere client
	vcInstance, ok := vs.vsphereInstanceMap[cfg.Global.VCenterIP]
	if !ok {
		t.Fatalf("Couldn't get vSphere instance: %s", cfg.Global.VCenterIP)
	}

	err = vcInstance.conn.Connect(ctx)
	if err != nil {
		t.Errorf("Failed to connect to vSphere: %s", err)
	}
	vcInstance.conn.Logout(ctx)
}
*/
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
			expectedError:    ErrUsernameMissing,
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
			expectedError:    ErrPasswordMissing,
		},
		{
			testName: "SecretNamespace, Username and Password missing in old configuration",
			conf: `[Global]
			server = 0.0.0.0
			datacenters = us-west
			secret-name = "vccreds"
			`,
			expectedError: ErrUsernameMissing,
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
		cfg, err := readConfig(strings.NewReader(testcase.conf))
		if err != nil {
			t.Fatalf("readConfig: Should succeed when a valid config is provided: %v", err)
		}
		vs, err = buildVSphereFromConfig(cfg)
		if err != nil { // testcase.expectedError {
			t.Fatalf("buildVSphereFromConfig: Should succeed when a valid config is provided: %v", err)
		}
		if testcase.expectedIsSecretProvided && (vs.cfg.Global.SecretNamespace == "" || vs.cfg.Global.SecretName == "") {
			t.Fatalf("SecretName and SecretNamespace was expected in config %s. error: %s",
				testcase.conf, err)
		}
		if !testcase.expectedIsSecretProvided {
			for _, vsInstance := range vs.vsphereInstanceMap {
				if vsInstance.conn.Username != testcase.expectedUsername {
					t.Fatalf("Expected username %s doesn't match actual username %s in config %s. error: %s",
						testcase.expectedUsername, vsInstance.conn.Username, testcase.conf, err)
				}
				if vsInstance.conn.Password != testcase.expectedPassword {
					t.Fatalf("Expected password %s doesn't match actual password %s in config %s. error: %s",
						testcase.expectedPassword, vsInstance.conn.Password, testcase.conf, err)
				}
			}
		}
		// Check, if all the expected thumbprints are configured
		for instanceName, expectedThumbprint := range testcase.expectedThumbprints {
			instanceConfig, ok := vs.vsphereInstanceMap[instanceName]
			if !ok {
				t.Fatalf("Could not find configuration for instance %s", instanceName)
			}
			if actualThumbprint := instanceConfig.conn.Thumbprint; actualThumbprint != expectedThumbprint {
				t.Fatalf(
					"Expected thumbprint for instance '%s' to be '%s', got '%s'",
					instanceName, expectedThumbprint, actualThumbprint,
				)
			}
		}
		// Check, if all all connections are configured with the global CA certificate
		if expectedCaPath := cfg.Global.CAFile; expectedCaPath != "" {
			for name, instance := range vs.vsphereInstanceMap {
				if actualCaPath := instance.conn.CACert; actualCaPath != expectedCaPath {
					t.Fatalf(
						"Expected CA certificate path for instance '%s' to be the globally configured one ('%s'), got '%s'",
						name, expectedCaPath, actualCaPath,
					)
				}
			}
		}
	}
}
