# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://suva.sh/posts/well-documented-makefiles

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := all

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq (,$(strip $(GOPROXY)))
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE := on

# Directories.
BIN_DIR := bin
SRCS := go.mod go.sum

# Binaries.
KIND := $(BIN_DIR)/kind
GINKGO := $(BIN_DIR)/ginkgo

all: kind ginkgo

kind: $(KIND) $(SRCS)
$(KIND): go.mod
	go build -tags=tools -o $@ sigs.k8s.io/kind

ginkgo: $(GINKGO) $(SRCS)
$(GINKGO): go.mod
	go build -tags=tools -o $@ github.com/onsi/ginkgo/ginkgo