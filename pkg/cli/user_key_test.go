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

package cli

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestCreateCertPrivateKeyMode(t *testing.T) {
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })

	dir := t.TempDir()
	u := &User{cert: filepath.Join(dir, "solution.crt")}
	if err := u.createCert(); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "solution.key")
	info, err := os.Lstat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatal("private key path is a symlink")
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("private key mode = %04o, want 0600", got)
	}
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(raw)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		t.Fatalf("private key pem type = %v", block)
	}
	if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(keyPath, []byte("old-key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := u.createCert(); err == nil {
		t.Fatal("expected an error when the private key already exists")
	}
	got, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-key" {
		t.Fatalf("existing private key was replaced: %q", got)
	}
}

func TestCreateCertRejectsKeySymlink(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "sentinel")
	if err := os.WriteFile(sentinel, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, "solution.key")
	if err := os.Symlink(sentinel, keyPath); err != nil {
		t.Fatal(err)
	}
	u := &User{cert: filepath.Join(dir, "solution.crt")}
	if err := u.createCert(); err == nil {
		t.Fatal("expected an error when the private key path is a symlink")
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("symlink target changed: %q", got)
	}
	info, err := os.Lstat(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("symlink was replaced with a regular file")
	}
}
