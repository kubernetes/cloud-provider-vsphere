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
	"errors"
	"io/ioutil"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
)

// Errors
var (
	// ErrCredentialsNotFound is returned when no credentials are configured.
	ErrCredentialsNotFound = errors.New("Credentials not found")

	// ErrCredentialMissing is returned when the credentials do not contain a username and/or password.
	ErrCredentialMissing = errors.New("Username/Password is missing")

	// ErrUnknownSecretKey is returned when the supplied key does not return a secret.
	ErrUnknownSecretKey = errors.New("Unknown secret key")
)

// GetCredential returns credentials for the given vCenter Server.
// GetCredential returns error if Secret is not added or SecretDirectory is not set (ie No Creds).
func (secretCredentialManager *SecretCredentialManager) GetCredential(server string) (*Credential, error) {
	//get the creds using the K8s listener if it exists
	if secretCredentialManager.SecretLister != nil {
		err := secretCredentialManager.updateCredentialsMapK8s()
		if err != nil {
			statusErr, ok := err.(*apierrors.StatusError)
			if (ok && statusErr.ErrStatus.Code != http.StatusNotFound) || !ok {
				return nil, err
			}
			// Handle secrets deletion by finding credentials from cache
			klog.Warningf("secret %q not found in namespace %q", secretCredentialManager.SecretName, secretCredentialManager.SecretNamespace)
		}
	}

	//get the creds using the Secrets File if it exists
	if secretCredentialManager.SecretsDirectory != "" {
		err := secretCredentialManager.updateCredentialsMapFile()
		if err != nil {
			klog.Warningf("Failed parsing SecretsDirectory %q: %q", secretCredentialManager.SecretsDirectory, err)
		}
	}

	credential, found := secretCredentialManager.Cache.GetCredential(server)
	if !found {
		klog.Errorf("credentials not found for server %q", server)
		return nil, ErrCredentialsNotFound
	}
	return &credential, nil
}

func (secretCredentialManager *SecretCredentialManager) updateCredentialsMapK8s() error {
	secret, err := secretCredentialManager.SecretLister.Secrets(secretCredentialManager.SecretNamespace).Get(secretCredentialManager.SecretName)
	if err != nil {
		klog.Warningf("Cannot get secret %s in namespace %s. error: %q", secretCredentialManager.SecretName, secretCredentialManager.SecretNamespace, err)
		return err
	}
	cacheSecret := secretCredentialManager.Cache.GetSecret()
	if cacheSecret != nil &&
		cacheSecret.GetResourceVersion() == secret.GetResourceVersion() {
		klog.V(2).Infof("VCP SecretCredentialManager: Secret %q will not be updated in cache. Since, secrets have same resource version %q", secretCredentialManager.SecretName, cacheSecret.GetResourceVersion())
		return nil
	}
	secretCredentialManager.Cache.UpdateSecret(secret)
	return secretCredentialManager.Cache.parseSecret()
}

func (secretCredentialManager *SecretCredentialManager) updateCredentialsMapFile() error {
	//Secretsdirectory was parsed before, no need to do it again
	if secretCredentialManager.SecretsDirectoryParse {
		return nil
	}

	//take the mounted secrets in the form of files and make it looks like we
	//parsed it from a k8s secret so we can reuse the SecretCache.parseSecret() func
	data := make(map[string][]byte)

	files, err := ioutil.ReadDir(secretCredentialManager.SecretsDirectory)
	if err != nil {
		secretCredentialManager.SecretsDirectoryParse = true
		klog.Warningf("Failed to find secrets directory %s. error: %q", secretCredentialManager.SecretsDirectory, err)
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			klog.Warningf("Skipping parse of directory: %s", f.Name())
			continue
		}

		fullFilePath := secretCredentialManager.SecretsDirectory + "/" + f.Name()
		contents, err := ioutil.ReadFile(fullFilePath)
		if err != nil {
			klog.Warningf("Cannot read  file %s. error: %q", fullFilePath, err)
			continue
		}

		data[f.Name()] = contents
	}

	secretCredentialManager.SecretsDirectoryParse = true
	secretCredentialManager.Cache.UpdateSecretFile(data)
	return secretCredentialManager.Cache.parseSecret()
}

// GetSecret returns a Kubernetes secret.
func (cache *SecretCache) GetSecret() *corev1.Secret {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	return cache.Secret
}

// UpdateSecret updates a Kubernetes secret with the provided data.
func (cache *SecretCache) UpdateSecret(secret *corev1.Secret) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.Secret = secret
}

// UpdateSecretFile updates a Kubernetes secret with the provided data.
func (cache *SecretCache) UpdateSecretFile(data map[string][]byte) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	cache.SecretFile = data
}

// GetCredential returns the vCenter credentials from a Kubernetes secret
// for the provided vCenter.
func (cache *SecretCache) GetCredential(server string) (Credential, bool) {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()
	credential, found := cache.VirtualCenter[server]
	if !found {
		return Credential{}, found
	}
	return *credential, found
}

func (cache *SecretCache) parseSecret() error {
	cache.cacheLock.Lock()
	defer cache.cacheLock.Unlock()

	var data map[string][]byte
	if cache.Secret != nil {
		klog.V(3).Infof("parseSecret using k8s secret")
		data = cache.Secret.Data
	} else if cache.SecretFile != nil {
		klog.V(3).Infof("parseSecret using secrets directory")
		data = cache.SecretFile
	}

	return parseConfig(data, cache.VirtualCenter)
}

// parseConfig returns vCenter ip/fdqn mapping to its credentials viz. Username and Password.
func parseConfig(data map[string][]byte, config map[string]*Credential) error {
	if len(data) == 0 {
		return ErrCredentialMissing
	}
	for credentialKey, credentialValue := range data {
		credentialKey = strings.ToLower(credentialKey)
		if strings.HasSuffix(credentialKey, "password") {
			vcServer := strings.Split(credentialKey, ".password")[0]
			if _, ok := config[vcServer]; !ok {
				config[vcServer] = &Credential{}
			}
			config[vcServer].Password = string(credentialValue)
		} else if strings.HasSuffix(credentialKey, "username") {
			vcServer := strings.Split(credentialKey, ".username")[0]
			if _, ok := config[vcServer]; !ok {
				config[vcServer] = &Credential{}
			}
			config[vcServer].User = string(credentialValue)
		} else {
			klog.Errorf("Unknown secret key %s", credentialKey)
			return ErrUnknownSecretKey
		}
	}
	for vcServer, credential := range config {
		if credential.User == "" || credential.Password == "" {
			klog.Errorf("Username/Password is missing for server %s", vcServer)
			return ErrCredentialMissing
		}
	}
	return nil
}
