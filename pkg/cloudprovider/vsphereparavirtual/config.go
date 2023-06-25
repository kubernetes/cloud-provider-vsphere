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

package vsphereparavirtual

import (
	"encoding/json"
	"net"
	"os"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	// VsphereParavirtualCloudProviderConfigPath is the path for vsphere paravirtual cloud provider config file
	VsphereParavirtualCloudProviderConfigPath string = "/etc/kubernetes/guestclusters/tanzukubernetescluster/ownerref.json"
	// SupervisorClusterConfigPath is the path for supervisor access related files,
	// like secret related file
	SupervisorClusterConfigPath = "/etc/cloud/ccm-provider"
	// SupervisorClusterAccessTokenFile is the access token file for supervisor access
	SupervisorClusterAccessTokenFile = "token"
	// SupervisorClusterAccessCAFile is the CA file for supervisor access
	SupervisorClusterAccessCAFile = "ca.crt"
	// SupervisorClusterAccessNamespaceFile is the namespace for supervisor access
	SupervisorClusterAccessNamespaceFile = "namespace"
	// SupervisorAPIServerPortEnv reads supervisor service endpoint info from env
	SupervisorAPIServerPortEnv string = "SUPERVISOR_APISERVER_PORT"
	// SupervisorAPIServerEndpointIPEnv reads supervisor API server endpoint IP from env
	SupervisorAPIServerEndpointIPEnv string = "SUPERVISOR_APISERVER_ENDPOINT_IP"
	// SupervisorServiceAccountNameEnv reads supervisor service account name from env
	SupervisorServiceAccountNameEnv string = "SUPERVISOR_CLUSTER_SERVICEACCOUNT_SECRET_NAME"
	// SupervisorAPIServerFQDN reads supervisor service API server's fully qualified domain name from env
	SupervisorAPIServerFQDN string = "supervisor.default.svc"
)

// SupervisorEndpoint is the supervisor cluster endpoint
type SupervisorEndpoint struct {
	// supervisor cluster proxy service hostname
	Endpoint string
	// supervisor cluster proxy service  port
	Port string
}

func readOwnerRef(path string) (*metav1.OwnerReference, error) {
	ownerRef := &metav1.OwnerReference{}
	d, err := os.ReadFile(path)
	if err != nil {
		return ownerRef, errors.Wrapf(err, "Failed Reading OwnerReference Config file %s", path)
	}
	err = json.Unmarshal(d, ownerRef)
	if err != nil {
		return ownerRef, errors.Wrapf(err, "Failed Unmarshalling OwnerReference Config file %s", path)
	}
	return ownerRef, nil
}

func readSupervisorConfig() (*SupervisorEndpoint, error) {
	remoteVip := os.Getenv(SupervisorAPIServerEndpointIPEnv)
	if remoteVip == "" {
		// call os.Exit(1) for the pod to restart
		klog.Fatalf("%s is missing in env vars", SupervisorAPIServerEndpointIPEnv)
	}

	remotePort := os.Getenv(SupervisorAPIServerPortEnv)

	if remotePort == "" {
		// call os.Exit(1) for the pod to restart
		klog.Fatalf("%s is missing in env vars", SupervisorAPIServerPortEnv)

	}

	klog.V(6).Infof("Configured with remote apiserver %s:%s", remoteVip, remotePort)
	return &SupervisorEndpoint{
		Endpoint: remoteVip,
		Port:     remotePort,
	}, nil

}

func getNameSpace(svConfigPath string) (string, error) {
	namespaceFile := svConfigPath + "/" + SupervisorClusterAccessNamespaceFile
	namespace, err := os.ReadFile(namespaceFile)
	if err != nil {
		klog.Errorf("Failed to read namespace from %s: %v", namespaceFile, err)
		return "", err
	}
	return string(namespace), nil
}

func getRestConfig(svConfigPath string) (*rest.Config, error) {
	svEndpoint, err := readSupervisorConfig()
	if err != nil {
		klog.Errorf("Failed to read supervisor endpoint info from env: %v", err)
		return nil, err
	}

	tokenFile := svConfigPath + "/" + SupervisorClusterAccessTokenFile
	token, err := os.ReadFile(tokenFile)

	if err != nil {
		klog.Errorf("Failed to read token from %s: %v", tokenFile, err)
		return nil, err
	}

	rootCAFile := svConfigPath + "/" + SupervisorClusterAccessCAFile
	rootCA, err := os.ReadFile(rootCAFile)

	if err != nil {
		klog.Errorf("Failed to read ca cert from %s: %v", rootCAFile, err)
		return nil, err
	}

	return &rest.Config{
		Host: "https://" + net.JoinHostPort(svEndpoint.Endpoint, svEndpoint.Port),
		TLSClientConfig: rest.TLSClientConfig{
			CAData:     rootCA,
			ServerName: SupervisorAPIServerFQDN,
		},
		BearerToken: string(token),
	}, nil
}
