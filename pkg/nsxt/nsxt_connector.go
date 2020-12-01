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

package nsxt

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/core"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/protocol/client"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/security"
	"k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
)

type remoteBasicAuthHeaderProcessor struct {
}

func newRemoteBasicAuthHeaderProcessor() *remoteBasicAuthHeaderProcessor {
	return &remoteBasicAuthHeaderProcessor{}
}

func (processor remoteBasicAuthHeaderProcessor) Process(req *http.Request) error {
	oldAuthHeader := req.Header.Get("Authorization")
	newAuthHeader := strings.Replace(oldAuthHeader, "Basic", "Remote", 1)
	req.Header.Set("Authorization", newAuthHeader)
	return nil
}

// NewNsxtConnector creates a new NSXT connector
func NewNsxtConnector(nsxtConfig *config.NsxtConfig) (client.Connector, error) {
	url := fmt.Sprintf("https://%s", nsxtConfig.Host)
	securityCtx := core.NewSecurityContextImpl()
	securityContextNeeded := true
	if len(nsxtConfig.ClientAuthCertFile) > 0 {
		securityContextNeeded = false
	}

	if securityContextNeeded {
		if len(nsxtConfig.VMCAccessToken) > 0 {
			if nsxtConfig.VMCAuthHost == "" {
				return nil, fmt.Errorf("vmc auth host must be provided if auth token is provided")
			}

			apiToken, err := getAPIToken(nsxtConfig.VMCAuthHost, nsxtConfig.VMCAccessToken)
			if err != nil {
				return nil, err
			}

			securityCtx.SetProperty(security.AUTHENTICATION_SCHEME_ID, security.OAUTH_SCHEME_ID)
			securityCtx.SetProperty(security.ACCESS_TOKEN, apiToken)
		} else {
			if nsxtConfig.User == "" {
				return nil, fmt.Errorf("username must be provided")
			}

			if nsxtConfig.Password == "" {
				return nil, fmt.Errorf("password must be provided")
			}

			securityCtx.SetProperty(security.AUTHENTICATION_SCHEME_ID, security.USER_PASSWORD_SCHEME_ID)
			securityCtx.SetProperty(security.USER_KEY, nsxtConfig.User)
			securityCtx.SetProperty(security.PASSWORD_KEY, nsxtConfig.Password)
		}
	}

	tlsConfig, err := getConnectorTLSConfig(nsxtConfig.InsecureFlag, nsxtConfig.ClientAuthCertFile, nsxtConfig.ClientAuthKeyFile, nsxtConfig.CAFile)
	if err != nil {
		return nil, err
	}
	httpClient := http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyFromEnvironment,
			TLSClientConfig: tlsConfig,
		},
	}

	connector := client.NewRestConnector(url, httpClient)
	connector.SetSecurityContext(securityCtx)
	if nsxtConfig.RemoteAuth {
		connector.AddRequestProcessor(newRemoteBasicAuthHeaderProcessor())
	}

	return connector, nil
}

// getConnectorTLSConfig loads certificates to build TLS configuration
func getConnectorTLSConfig(insecure bool, clientCertFile string, clientKeyFile string, caFile string) (*tls.Config, error) {
	tlsConfig := tls.Config{InsecureSkipVerify: insecure}

	if len(clientCertFile) > 0 {
		if len(clientKeyFile) == 0 {
			return nil, fmt.Errorf("Please provide key file for client certificate")
		}

		cert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("Failed to load client cert/key pair: %v", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if len(caFile) > 0 {
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)

		tlsConfig.RootCAs = caCertPool
	}

	tlsConfig.BuildNameToCertificate()

	return &tlsConfig, nil
}

type jwtToken struct {
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    string `json:"expires_in"`
	Scope        string `json:"scope"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// getAPIToken gets JWT access token
func getAPIToken(vmcAuthHost string, vmcAccessToken string) (string, error) {

	payload := strings.NewReader("refresh_token=" + vmcAccessToken)
	req, _ := http.NewRequest("POST", "https://"+vmcAuthHost, payload)

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(res.Body)
		return "", fmt.Errorf("Unexpected status code %d trying to get auth token. %s", res.StatusCode, string(b))
	}

	defer res.Body.Close()
	token := jwtToken{}
	err = json.NewDecoder(res.Body).Decode(&token)
	if err != nil {
		return "", errors.Wrapf(err, "Decoding token failed with")
	}

	return token.AccessToken, nil
}
