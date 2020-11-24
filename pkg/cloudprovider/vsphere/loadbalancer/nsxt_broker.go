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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/vmware/vsphere-automation-sdk-go/lib/vapi/std"
	vapi_errors "github.com/vmware/vsphere-automation-sdk-go/lib/vapi/std/errors"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/bindings"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/core"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/data"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/protocol/client"
	"github.com/vmware/vsphere-automation-sdk-go/runtime/security"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/ip_pools"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/infra/realized_state"
	"github.com/vmware/vsphere-automation-sdk-go/services/nsxt/model"

	"k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphere/loadbalancer/config"
)

// NsxtBroker is an internal interface to enable mocking the nsxt backend
type NsxtBroker interface {
	ReadLoadBalancerService(id string) (model.LBService, error)
	CreateLoadBalancerService(service model.LBService) (model.LBService, error)
	ListLoadBalancerServices() ([]model.LBService, error)
	UpdateLoadBalancerService(service model.LBService) (model.LBService, error)
	DeleteLoadBalancerService(id string) error
	CreateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error)
	ListLoadBalancerVirtualServers() ([]model.LBVirtualServer, error)
	UpdateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error)
	DeleteLoadBalancerVirtualServer(id string) error
	CreateLoadBalancerPool(pool model.LBPool) (model.LBPool, error)
	ReadLoadBalancerPool(id string) (model.LBPool, error)
	ListLoadBalancerPools() ([]model.LBPool, error)
	UpdateLoadBalancerPool(pool model.LBPool) (model.LBPool, error)
	DeleteLoadBalancerPool(id string) error
	ListIPPools() ([]model.IpAddressPool, error)
	AllocateFromIPPool(ipPoolID string, allocation model.IpAddressAllocation) (model.IpAddressAllocation, string, error)
	ListIPPoolAllocations(ipPoolID string) ([]model.IpAddressAllocation, error)
	ReleaseFromIPPool(ipPoolID, ipAllocationID string) error
	GetRealizedExternalIPAddress(ipAllocationPath string, timeout time.Duration) (*string, error)
	ListAppProfiles() ([]*data.StructValue, error)

	CreateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error)
	ListLoadBalancerMonitorProfiles() ([]*data.StructValue, error)
	ReadLoadBalancerTCPMonitorProfile(id string) (model.LBTcpMonitorProfile, error)
	UpdateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error)
	DeleteLoadBalancerMonitorProfile(id string) error
}

type nsxtBroker struct {
	lbServicesClient        infra.LbServicesClient
	lbVirtServersClient     infra.LbVirtualServersClient
	lbPoolsClient           infra.LbPoolsClient
	ipPoolsClient           infra.IpPoolsClient
	ipAllocationsClient     ip_pools.IpAllocationsClient
	lbAppProfilesClient     infra.LbAppProfilesClient
	lbMonitorProfilesClient infra.LbMonitorProfilesClient
	realizedEntitiesClient  realized_state.RealizedEntitiesClient
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

// NewNsxtBroker creates a new NsxtBroker using the configuration
func NewNsxtBroker(nsxtConfig *config.NsxtConfig) (NsxtBroker, error) {
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

	// perform API call to check connector
	_, err = infra.NewDefaultLbMonitorProfilesClient(connector).List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Connection to NSX-T API failed. Please check your connection settings.")
	}
	return NewNsxtBrokerFromConnector(connector), nil
}

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

func getAPIToken(vmcAuthHost string, vmcAccessToken string) (string, error) {

	payload := strings.NewReader("refresh_token=" + vmcAccessToken)
	req, _ := http.NewRequest("POST", "https://"+vmcAuthHost, payload)

	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return "", err
	}

	if res.StatusCode != 200 {
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

// NewNsxtBrokerFromConnector creates a new NsxtBroker to the real API
func NewNsxtBrokerFromConnector(connector client.Connector) NsxtBroker {
	return &nsxtBroker{
		lbServicesClient:        infra.NewDefaultLbServicesClient(connector),
		lbVirtServersClient:     infra.NewDefaultLbVirtualServersClient(connector),
		lbPoolsClient:           infra.NewDefaultLbPoolsClient(connector),
		ipPoolsClient:           infra.NewDefaultIpPoolsClient(connector),
		ipAllocationsClient:     ip_pools.NewDefaultIpAllocationsClient(connector),
		lbAppProfilesClient:     infra.NewDefaultLbAppProfilesClient(connector),
		lbMonitorProfilesClient: infra.NewDefaultLbMonitorProfilesClient(connector),
		realizedEntitiesClient:  realized_state.NewDefaultRealizedEntitiesClient(connector),
	}
}

func (b *nsxtBroker) ReadLoadBalancerService(id string) (model.LBService, error) {
	return b.lbServicesClient.Get(id)
}

func (b *nsxtBroker) CreateLoadBalancerService(service model.LBService) (model.LBService, error) {
	id := uuid.New().String()
	result, err := b.lbServicesClient.Update(id, service)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerServices() ([]model.LBService, error) {
	result, err := b.lbServicesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbServicesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerService(service model.LBService) (model.LBService, error) {
	result, err := b.lbServicesClient.Update(*service.Id, service)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerService(id string) error {
	err := b.lbServicesClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) CreateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error) {
	id := uuid.New().String()
	result, err := b.lbVirtServersClient.Update(id, server)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerVirtualServers() ([]model.LBVirtualServer, error) {
	result, err := b.lbVirtServersClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbVirtServersClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerVirtualServer(server model.LBVirtualServer) (model.LBVirtualServer, error) {
	result, err := b.lbVirtServersClient.Update(*server.Id, server)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerVirtualServer(id string) error {
	err := b.lbVirtServersClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) CreateLoadBalancerPool(pool model.LBPool) (model.LBPool, error) {
	id := uuid.New().String()
	result, err := b.lbPoolsClient.Update(id, pool)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ReadLoadBalancerPool(id string) (model.LBPool, error) {
	result, err := b.lbPoolsClient.Get(id)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) ListLoadBalancerPools() ([]model.LBPool, error) {
	result, err := b.lbPoolsClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbPoolsClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) UpdateLoadBalancerPool(pool model.LBPool) (model.LBPool, error) {
	result, err := b.lbPoolsClient.Update(*pool.Id, pool)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerPool(id string) error {
	err := b.lbPoolsClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) ListAppProfiles() ([]*data.StructValue, error) {
	result, err := b.lbAppProfilesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbAppProfilesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) CreateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	id := uuid.New().String()
	result, err := b.createOrUpdateLoadBalancerTCPMonitorProfile(id, monitor)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) createOrUpdateLoadBalancerTCPMonitorProfile(id string, monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	monitor.ResourceType = model.LBMonitorProfile_RESOURCE_TYPE_LBTCPMONITORPROFILE
	converter := newNsxtTypeConverter()
	value, err := converter.convertLBTCPMonitorProfileToStructValue(monitor)
	if err != nil {
		return model.LBTcpMonitorProfile{}, errors.Wrapf(err, "converting LBTcpMonitorProfile failed")
	}
	result, err := b.lbMonitorProfilesClient.Update(id, value)
	if err != nil {
		return model.LBTcpMonitorProfile{}, nicerVAPIError(err)
	}
	return converter.convertStructValueToLBTCPMonitorProfile(result)
}

func (b *nsxtBroker) ListLoadBalancerMonitorProfiles() ([]*data.StructValue, error) {
	result, err := b.lbMonitorProfilesClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.lbMonitorProfilesClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) ReadLoadBalancerTCPMonitorProfile(id string) (model.LBTcpMonitorProfile, error) {
	itf, err := b.lbMonitorProfilesClient.Get(id)
	if err != nil {
		return model.LBTcpMonitorProfile{}, errors.Wrapf(nicerVAPIError(err), "getting LBTcpMonitorProfile %s failed", id)
	}
	return newNsxtTypeConverter().convertStructValueToLBTCPMonitorProfile(itf)
}

func (b *nsxtBroker) UpdateLoadBalancerTCPMonitorProfile(monitor model.LBTcpMonitorProfile) (model.LBTcpMonitorProfile, error) {
	result, err := b.createOrUpdateLoadBalancerTCPMonitorProfile(*monitor.Id, monitor)
	return result, nicerVAPIError(err)
}

func (b *nsxtBroker) DeleteLoadBalancerMonitorProfile(id string) error {
	err := b.lbMonitorProfilesClient.Delete(id, nil)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) ListIPPools() ([]model.IpAddressPool, error) {
	result, err := b.ipPoolsClient.List(nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.ipPoolsClient.List(result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) AllocateFromIPPool(ipPoolID string, allocation model.IpAddressAllocation) (model.IpAddressAllocation, string, error) {
	id := uuid.New().String()
	err := b.ipAllocationsClient.Patch(ipPoolID, id, allocation)
	if err != nil {
		return allocation, "", nicerVAPIError(err)
	}
	allocated, err := b.ipAllocationsClient.Get(ipPoolID, id)
	if err != nil {
		return allocation, "", nicerVAPIError(err)
	}
	ipAddress, err := b.GetRealizedExternalIPAddress(*allocated.Path, 15*time.Second)
	if err != nil {
		return allocated, "", nicerVAPIError(err)
	}
	if ipAddress == nil {
		return allocated, "", fmt.Errorf("no IP address allocated for %s", *allocated.Path)
	}
	return allocated, *ipAddress, nil
}

func (b *nsxtBroker) ListIPPoolAllocations(ipPoolID string) ([]model.IpAddressAllocation, error) {
	result, err := b.ipAllocationsClient.List(ipPoolID, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, nicerVAPIError(err)
	}
	list := result.Results
	count := int(*result.ResultCount)
	for len(list) < count {
		result, err = b.ipAllocationsClient.List(ipPoolID, result.Cursor, nil, nil, nil, nil, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		list = append(list, result.Results...)
	}
	return list, nil
}

func (b *nsxtBroker) ReleaseFromIPPool(ipPoolID, ipAllocationID string) error {
	err := b.ipAllocationsClient.Delete(ipPoolID, ipAllocationID)
	return nicerVAPIError(err)
}

func (b *nsxtBroker) GetRealizedExternalIPAddress(ipAllocationPath string, timeout time.Duration) (*string, error) {
	// wait for realized state
	limit := time.Now().Add(timeout)
	sleepIncr := 100 * time.Millisecond
	sleepMax := 1000 * time.Millisecond
	sleep := sleepIncr
	for time.Now().Before(limit) {
		time.Sleep(sleep)
		sleep += sleepIncr
		if sleep > sleepMax {
			sleep = sleepMax
		}
		list, err := b.realizedEntitiesClient.List(ipAllocationPath, nil)
		if err != nil {
			return nil, nicerVAPIError(err)
		}
		for _, realizedResource := range list.Results {
			for _, attr := range realizedResource.ExtendedAttributes {
				if *attr.Key == "allocation_ip" {
					return &attr.Values[0], nil
				}
			}
		}
	}
	return nil, fmt.Errorf("Timeout of wait for realized state of IP allocation")
}

func nicerVAPIError(err error) error {
	switch vapiError := err.(type) {
	case vapi_errors.InvalidRequest:
		// Connection errors end up here
		return nicerVapiErrorData("InvalidRequest", vapiError.Data, vapiError.Messages)
	case vapi_errors.NotFound:
		return nicerVapiErrorData("NotFound", vapiError.Data, vapiError.Messages)
	case vapi_errors.Unauthorized:
		return nicerVapiErrorData("Unauthorized", vapiError.Data, vapiError.Messages)
	case vapi_errors.Unauthenticated:
		return nicerVapiErrorData("Unauthenticated", vapiError.Data, vapiError.Messages)
	case vapi_errors.InternalServerError:
		return nicerVapiErrorData("InternalServerError", vapiError.Data, vapiError.Messages)
	case vapi_errors.ServiceUnavailable:
		return nicerVapiErrorData("ServiceUnavailable", vapiError.Data, vapiError.Messages)
	}

	return err
}

func nicerVapiErrorData(errorMsg string, apiErrorDataValue *data.StructValue, messages []std.LocalizableMessage) error {
	if apiErrorDataValue == nil {
		if len(messages) > 0 {
			return fmt.Errorf("%s (%s)", errorMsg, messages[0].DefaultMessage)
		}
		return fmt.Errorf("%s (no additional details provided)", errorMsg)
	}

	var typeConverter = bindings.NewTypeConverter()
	typeConverter.SetMode(bindings.REST)
	rawData, err := typeConverter.ConvertToGolang(apiErrorDataValue, model.ApiErrorBindingType())

	if err != nil {
		return fmt.Errorf("%s (failed to extract additional details due to %s)", errorMsg, err)
	}
	apiError := rawData.(model.ApiError)
	details := fmt.Sprintf(" %s: %s (code %v)", errorMsg, *apiError.ErrorMessage, *apiError.ErrorCode)

	if len(apiError.RelatedErrors) > 0 {
		details += "\nRelated errors:\n"
		for _, relatedErr := range apiError.RelatedErrors {
			details += fmt.Sprintf("%s (code %v)", *relatedErr.ErrorMessage, relatedErr.ErrorCode)
		}
	}
	return fmt.Errorf(details)
}
