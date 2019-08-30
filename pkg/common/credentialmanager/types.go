/*
Copyright 2018 The Kubernetes Authors.

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

package credentialmanager

import (
	"sync"

	v1 "k8s.io/api/core/v1"
	clientv1 "k8s.io/client-go/listers/core/v1"
)

// SecretCache is used to cache information about Kubernetes secrets data.
type SecretCache struct {
	cacheLock     sync.Mutex
	VirtualCenter map[string]*Credential
	Secret        *v1.Secret
	SecretFile    map[string][]byte
}

// Credential is a vCenter credential that is retrieved or stored in a
// Kubernetes secret.
type Credential struct {
	User     string `gcfg:"user"`
	Password string `gcfg:"password"`
}

// CredentialManager is used to manage vCenter credentials stored as
// Kubernetes secrets.
type CredentialManager struct {
	SecretName             string
	SecretNamespace        string
	SecretLister           clientv1.SecretLister
	SecretsDirectory       string
	secretsDirectoryParsed bool // internal placeholder to identify we parsed the SecretsDirectory
	Cache                  *SecretCache
}
