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
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/pkg/errors"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/core"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/protocol/client"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/security"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
	nsxtcfg "k8s.io/cloud-provider-vsphere/pkg/nsxt/config"
	klog "k8s.io/klog/v2"
)

// ConnectorManager manages NSXT connection
type ConnectorManager struct {
	config    *config.Config
	connector client.Connector
}

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

// NewConnectorManager creates a new NSXT connector
func NewConnectorManager(nsxtConfig *config.Config) (*ConnectorManager, error) {
	cm := &ConnectorManager{}
	if nsxtConfig == nil {
		return cm, nil
	}
	cm.config = nsxtConfig
	url := fmt.Sprintf("https://%s", nsxtConfig.Host)
	var securityCtx *core.SecurityContextImpl
	securityContextNeeded := true
	if len(nsxtConfig.ClientAuthCertFile) > 0 {
		securityContextNeeded = false
	}

	if securityContextNeeded {
		securityCtx = core.NewSecurityContextImpl()
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
		} else if nsxtConfig.User != "" && nsxtConfig.Password != "" {
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
	if securityCtx != nil {
		connector.SetSecurityContext(securityCtx)
	}
	if nsxtConfig.RemoteAuth {
		connector.AddRequestProcessor(newRemoteBasicAuthHeaderProcessor())
	}
	cm.connector = connector

	return cm, nil
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
		caCert, err := os.ReadFile(caFile)
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
		b, _ := io.ReadAll(res.Body)
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

// GetConnector gets NSXT connector
func (cm *ConnectorManager) GetConnector() client.Connector {
	return cm.connector
}

// AddSecretListener adds secret informer add, update, delete callbacks
func (cm *ConnectorManager) AddSecretListener(secretInformer v1.SecretInformer) error {
	if cm.config == nil {
		return errors.New("config is not available for NSXT connector manager")
	}
	if cm.config.SecretName == "" || cm.config.SecretNamespace == "" {
		klog.V(6).Infof("No need to initialize NSXT secret manager as secret is not provided")
		return nil
	}
	if secretInformer == nil {
		return errors.New("failed to initialize NSXT secret manager as secret informer is nil")
	}

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    cm.secretAdded,
		UpdateFunc: cm.secretUpdated,
		DeleteFunc: cm.secretDeleted,
	})

	return nil
}

// isForNsxtSecret checks if secret is for nsxt config
func (cm *ConnectorManager) isForNsxtSecret(secret *corev1.Secret) bool {
	if secret.GetName() == cm.config.SecretName && secret.GetNamespace() == cm.config.SecretNamespace {
		return true
	}
	return false
}

// secretAdded handles secret added event
func (cm *ConnectorManager) secretAdded(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if secret == nil || !ok {
		return
	}
	if cm.isForNsxtSecret(secret) {
		cm.updateConnectorContext(secret)
	}
}

// secretUpdated handles secret updated event
func (cm *ConnectorManager) secretUpdated(oldObj, newObj interface{}) {
	oldSecret, ok := oldObj.(*corev1.Secret)
	if oldSecret == nil || !ok {
		return
	}
	newSecret, ok := newObj.(*corev1.Secret)
	if newSecret == nil || !ok {
		return
	}
	if cm.isForNsxtSecret(newSecret) && !reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
		cm.updateConnectorContext(newSecret)
	}
}

// secretDeleted handles secret deleted event
func (cm *ConnectorManager) secretDeleted(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if secret == nil || !ok {
		return
	}
	if cm.isForNsxtSecret(secret) {
		cm.resetConnectorContext()
	}
}

// updateConnectorContext updates security context of connector
func (cm *ConnectorManager) updateConnectorContext(secret *corev1.Secret) {
	var username, password string
	for key, value := range secret.Data {
		if key == nsxtcfg.UsernameKeyInSecret {
			username = string(value)
		}
		if key == nsxtcfg.PasswordKeyInSecret {
			password = string(value)
		}
	}
	if username == "" || password == "" {
		klog.Warningf("NSXT username and password should be both provided in secret")
		return
	}
	klog.V(6).Infof("Updating security context for NSXT connection")
	securityCtx := core.NewSecurityContextImpl()
	securityCtx.SetProperty(security.AUTHENTICATION_SCHEME_ID, security.USER_PASSWORD_SCHEME_ID)
	securityCtx.SetProperty(security.USER_KEY, username)
	securityCtx.SetProperty(security.PASSWORD_KEY, password)
	cm.connector.SetSecurityContext(securityCtx)
}

// resetConnectorContext resets security context of connector
func (cm *ConnectorManager) resetConnectorContext() {
	klog.V(6).Infof("Resetting security context for NSXT connection")
	securityCtx := core.NewSecurityContextImpl()
	cm.connector.SetSecurityContext(securityCtx)
}
