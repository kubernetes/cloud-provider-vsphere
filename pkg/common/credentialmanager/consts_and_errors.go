/*
Copyright 2019 The Kubernetes Authors.

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
)

const (
	usernamePrefix = "username_"
	passwordPrefix = "password_"
	serverPrefix   = "server_"
)

// Errors
var (
	// ErrCredentialsNotFound is returned when no credentials are configured.
	ErrCredentialsNotFound = errors.New("Credentials not found")

	// ErrCredentialMissing is returned when the credentials do not contain a username and/or password.
	ErrCredentialMissing = errors.New("Username/Password is missing")

	// ErrUnknownSecretKey is returned when the supplied key does not return a secret.
	ErrUnknownSecretKey = errors.New("Unknown secret key")

	// ErrIncompleteCredentialSet is returned when the credentials do not contain all required values
	ErrIncompleteCredentialSet = errors.New("Credentials did not have all required values")
)
