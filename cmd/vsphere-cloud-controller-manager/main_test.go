/*
Copyright 2026 The Kubernetes Authors.

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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fsnotify/fsnotify"

	pvconfig "k8s.io/cloud-provider-vsphere/pkg/cloudprovider/vsphereparavirtual/config"
)

// writeFile is a test helper that writes content to dir/name.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
}

// newCredsDir creates a temp directory populated with the three supervisor
// credential keys and returns the directory path.
func newCredsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, dir, pvconfig.SupervisorClusterAccessCAFile, "ca-baseline")
	writeFile(t, dir, pvconfig.SupervisorClusterAccessNamespaceFile, "ns-baseline")
	writeFile(t, dir, pvconfig.SupervisorClusterAccessTokenFile, "token-baseline")
	return dir
}

func restartKeys() []string {
	return []string{
		pvconfig.SupervisorClusterAccessCAFile,
		pvconfig.SupervisorClusterAccessNamespaceFile,
	}
}

func TestRestartGuard_ParavirtualMount(t *testing.T) {
	t.Run("token-only rotation is skipped", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		// Simulate an atomic-writer ..data swap that only changed the token.
		writeFile(t, dir, pvconfig.SupervisorClusterAccessTokenFile, "token-rotated")
		event := fsnotify.Event{Name: filepath.Join(dir, "..data"), Op: fsnotify.Create}

		if guard.shouldRestart(event) {
			t.Fatalf("token-only rotation should not trigger a restart")
		}
	})

	t.Run("ca.crt change triggers restart", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		writeFile(t, dir, pvconfig.SupervisorClusterAccessCAFile, "ca-rotated")
		event := fsnotify.Event{Name: filepath.Join(dir, "..data"), Op: fsnotify.Create}

		if !guard.shouldRestart(event) {
			t.Fatalf("ca.crt change should trigger a restart")
		}
	})

	t.Run("namespace change triggers restart", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		writeFile(t, dir, pvconfig.SupervisorClusterAccessNamespaceFile, "ns-rotated")
		event := fsnotify.Event{Name: filepath.Join(dir, "..data"), Op: fsnotify.Create}

		if !guard.shouldRestart(event) {
			t.Fatalf("namespace change should trigger a restart")
		}
	})

	t.Run("missing restart key triggers restart", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		if err := os.Remove(filepath.Join(dir, pvconfig.SupervisorClusterAccessCAFile)); err != nil {
			t.Fatalf("failed to remove ca.crt: %v", err)
		}
		event := fsnotify.Event{Name: filepath.Join(dir, "..data"), Op: fsnotify.Create}

		if !guard.shouldRestart(event) {
			t.Fatalf("unreadable restart key should fail safe and trigger a restart")
		}
	})

	t.Run("chmod is always ignored", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		// Even with a real ca.crt change, a Chmod-only event is ignored.
		writeFile(t, dir, pvconfig.SupervisorClusterAccessCAFile, "ca-rotated")
		event := fsnotify.Event{Name: filepath.Join(dir, pvconfig.SupervisorClusterAccessCAFile), Op: fsnotify.Chmod}

		if guard.shouldRestart(event) {
			t.Fatalf("chmod events should never trigger a restart")
		}
	})

	t.Run("event outside the credentials mount triggers restart", func(t *testing.T) {
		dir := newCredsDir(t)
		guard := newRestartGuard(dir, restartKeys())

		event := fsnotify.Event{Name: "/config/cloud-config", Op: fsnotify.Write}

		if !guard.shouldRestart(event) {
			t.Fatalf("changes outside the credentials mount should trigger a restart")
		}
	})
}

func TestRestartGuard_DefaultRestartsOnAnyChange(t *testing.T) {
	// An empty secretMountDir (non-paravirtual mode) restarts on every non-Chmod
	// event and ignores Chmod.
	guard := newRestartGuard("", nil)

	tests := []struct {
		name  string
		event fsnotify.Event
		want  bool
	}{
		{"write triggers restart", fsnotify.Event{Name: "/config/cloud-config", Op: fsnotify.Write}, true},
		{"create triggers restart", fsnotify.Event{Name: "/config/cloud-config", Op: fsnotify.Create}, true},
		{"chmod is ignored", fsnotify.Event{Name: "/config/cloud-config", Op: fsnotify.Chmod}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := guard.shouldRestart(tt.event); got != tt.want {
				t.Fatalf("shouldRestart(%+v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

func TestIsUnder(t *testing.T) {
	tests := []struct {
		name string
		path string
		dir  string
		want bool
	}{
		{"file directly under dir", "/etc/cloud/ccm-provider/..data", "/etc/cloud/ccm-provider", true},
		{"dir itself", "/etc/cloud/ccm-provider", "/etc/cloud/ccm-provider", true},
		{"unrelated path", "/config/cloud-config", "/etc/cloud/ccm-provider", false},
		{"prefix but not a child", "/etc/cloud/ccm-provider-extra/token", "/etc/cloud/ccm-provider", false},
		{"empty dir matches nothing", "/etc/cloud/ccm-provider/..data", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUnder(tt.path, tt.dir); got != tt.want {
				t.Fatalf("isUnder(%q, %q) = %v, want %v", tt.path, tt.dir, got, tt.want)
			}
		})
	}
}
