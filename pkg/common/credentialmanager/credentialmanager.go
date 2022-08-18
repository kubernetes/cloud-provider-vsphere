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
	"io/ioutil"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/client-go/listers/core/v1"
	klog "k8s.io/klog/v2"
)

// NewCredentialManager returns a new CredentialManager object.
func NewCredentialManager(secretName string, secretNamespace string, secretsDirectory string,
	secretLister v1.SecretLister) *CredentialManager {

	return &CredentialManager{
		SecretName:             secretName,
		SecretNamespace:        secretNamespace,
		SecretsDirectory:       secretsDirectory,
		SecretLister:           secretLister,
		secretsDirectoryParsed: false,
		Cache: &SecretCache{
			VirtualCenter: make(map[string]*Credential),
		},
	}
}

// GetCredential returns credentials for the given vCenter Server.
// GetCredential returns error if Secret is not added or SecretDirectory is not set (ie No Creds).
func (credentialManager *CredentialManager) GetCredential(server string) (*Credential, error) {
	//get the creds using the K8s listener if it exists
	if credentialManager.SecretLister != nil {
		klog.V(4).Info("SecretLister is valid. Retrieving secrets.")
		err := credentialManager.updateCredentialsMapK8s()
		if err != nil {
			klog.Errorf("updateCredentialsMapK8s failed. err=%s", err)
			statusErr, ok := err.(*apierrors.StatusError)
			if (ok && statusErr.ErrStatus.Code != http.StatusNotFound) || !ok {
				return nil, err
			}
			// Handle secrets deletion by finding credentials from cache
			klog.Warningf("secret %q not found in namespace %q", credentialManager.SecretName, credentialManager.SecretNamespace)
		}
	}

	//get the creds using the Secrets File if it exists
	if credentialManager.SecretsDirectory != "" {
		klog.V(4).Infof("SecretsDirectory is not empty. SecretsDirectory=%s", credentialManager.SecretsDirectory)
		err := credentialManager.updateCredentialsMapFile()
		if err != nil {
			klog.Warningf("Failed parsing SecretsDirectory %q: %q", credentialManager.SecretsDirectory, err)
		}
	}

	credential, found := credentialManager.Cache.GetCredential(server)
	if !found {
		klog.Errorf("credentials not found for server %s", server)
		return nil, ErrCredentialsNotFound
	}
	return &credential, nil
}

func (credentialManager *CredentialManager) updateCredentialsMapK8s() error {
	klog.V(4).Info("updateCredentialsMapK8s called")
	secret, err := credentialManager.SecretLister.Secrets(credentialManager.SecretNamespace).Get(credentialManager.SecretName)
	if err != nil {
		klog.Warningf("Cannot get secret %s in namespace %s. error: %q", credentialManager.SecretName, credentialManager.SecretNamespace, err)
		return err
	}
	cacheSecret := credentialManager.Cache.GetSecret()
	if cacheSecret != nil &&
		cacheSecret.GetResourceVersion() == secret.GetResourceVersion() {
		klog.V(2).Infof("Secret %q will not be updated in cache. Since, secrets have same resource version %q", credentialManager.SecretName, cacheSecret.GetResourceVersion())
		return nil
	}
	credentialManager.Cache.UpdateSecret(secret)
	err = credentialManager.Cache.parseSecret()
	if err != nil {
		klog.Errorf("parseSecret failed with err=%q", err)
	}

	return err
}

func (credentialManager *CredentialManager) updateCredentialsMapFile() error {
	//Secretsdirectory was parsed before, no need to do it again
	if credentialManager.secretsDirectoryParsed {
		return nil
	}

	//take the mounted secrets in the form of files and make it looks like we
	//parsed it from a k8s secret so we can reuse the SecretCache.parseSecret() func
	data := make(map[string][]byte)

	files, err := ioutil.ReadDir(credentialManager.SecretsDirectory)
	if err != nil {
		credentialManager.secretsDirectoryParsed = true
		klog.Warningf("Failed to find secrets directory %s. error: %q", credentialManager.SecretsDirectory, err)
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			klog.Warningf("Skipping parse of directory: %s", f.Name())
			continue
		}

		fullFilePath := credentialManager.SecretsDirectory + "/" + f.Name()
		contents, err := ioutil.ReadFile(fullFilePath)
		if err != nil {
			klog.Warningf("Cannot read  file %s. error: %q", fullFilePath, err)
			continue
		}

		data[f.Name()] = contents
	}

	credentialManager.secretsDirectoryParsed = true
	credentialManager.Cache.UpdateSecretFile(data)
	return credentialManager.Cache.parseSecret()
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
	unknownKeys := map[string][]byte{}
	for credentialKey, credentialValue := range data {
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
			unknownKeys[credentialKey] = credentialValue
		}
	}

	// Attempt to parse server/username/password from keys in the
	// alternative format. Iterate leftover key/values, looking for
	// entries that look like this:
	//   server_a: fd01::1
	//   username_a: vcenter-user
	//   password_a: vcenter-pass
	// This alternative format is needed because IPv6 addresses have colons,
	// making the original Secret format unusable.
	potentialAltFormatKeys := unknownKeys
	for credentialKey := range potentialAltFormatKeys {
		if strings.HasPrefix(credentialKey, serverPrefix) {
			parts := strings.Split(credentialKey, serverPrefix)
			if parts[1] != "" {
				serverKeySuffix := parts[1]
				passwordKey := passwordPrefix + serverKeySuffix
				usernameKey := usernamePrefix + serverKeySuffix
				serverKey := serverPrefix + serverKeySuffix

				var serverName, password, username []byte
				var ok bool
				serverName = data[serverKey]
				if _, ok := config[string(serverName)]; !ok {
					config[string(serverName)] = &Credential{}
				}

				if username, ok = data[usernameKey]; !ok {
					klog.Errorf("%s is missing for server %s", usernameKey, serverName)
					return ErrCredentialMissing
				}
				config[string(serverName)].User = string(username)

				if password, ok = data[passwordKey]; !ok {
					klog.Errorf("%s is missing for server %s", passwordKey, serverName)
					return ErrCredentialMissing
				}
				config[string(serverName)].Password = string(password)

				delete(unknownKeys, passwordKey)
				delete(unknownKeys, usernameKey)
				delete(unknownKeys, serverKey)
			} else {
				klog.Error("server secret key missing suffix")
				return ErrUnknownSecretKey
			}
		}
	}

	// Return errors if there are incomplete secret sets found. Return an error
	// when a username or password was found but no server address was found.
	// Return an error if username or password keys have no identifier suffix.
	for credentialKey := range unknownKeys {
		if strings.HasPrefix(credentialKey, usernamePrefix) {
			parts := strings.Split(credentialKey, usernamePrefix)
			if parts[1] == "" {
				klog.Errorf("Found username key with no suffix identifier.")
				return ErrUnknownSecretKey
			}
			identifier := parts[1]
			klog.Errorf("Found username key \"%s\" without a matching \"%s\" identifier", credentialKey, serverPrefix+identifier)
			return ErrIncompleteCredentialSet
		}
		if strings.HasPrefix(credentialKey, passwordPrefix) {
			parts := strings.Split(credentialKey, passwordPrefix)
			if parts[1] == "" {
				klog.Errorf("Found password key with no suffix identifier.")
				return ErrUnknownSecretKey
			}
			identifier := parts[1]
			klog.Errorf("Found password key \"%s\" without a matching \"%s\" identifier", credentialKey, serverPrefix+identifier)
			return ErrIncompleteCredentialSet
		}
	}

	for credentialKey := range unknownKeys {
		klog.Errorf("Unknown secret key %s", credentialKey)
		return ErrUnknownSecretKey
	}

	for vcServer, credential := range config {
		if credential.User == "" || credential.Password == "" {
			klog.Errorf("Username/Password is missing for server %s", vcServer)
			return ErrCredentialMissing
		}
	}
	return nil
}
