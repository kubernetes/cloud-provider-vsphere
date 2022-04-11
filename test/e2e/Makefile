# Copyright 2021 The Kubernetes Authors.
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

REPO_ROOT := $(shell git rev-parse --show-toplevel)

TMP_DIR := /tmp
TMP_CAPV_DIR := $(TMP_DIR)/capv

TOOLS_DIR := $(REPO_ROOT)/hack/tools
TOOLS_BIN_DIR := $(TOOLS_DIR)/bin
GINKGO := $(TOOLS_BIN_DIR)/ginkgo
KIND := $(TOOLS_BIN_DIR)/kind

TOOLING_BINARIES := $(GINKGO) $(KIND)

# E2E_ARTIFACTS is the folder to store e2e test artifacts
E2E_ARTIFACTS ?= $(REPO_ROOT)/_e2e_artifacts

# E2E_CONF_FILE is the configuration file for e2e testing
E2E_CONF_FILE ?= ${REPO_ROOT}/test/e2e/config/vsphere-dev.yaml

# E2E_DATA_DIR contains provider manifests needed to create the bootsrap cluster, required by the E2E_CONF_FILE
E2E_DATA_DIR := ${REPO_ROOT}/test/e2e/data

# E2E_DATA_CAPV_VER defines the providers from which version of CAPV to use
E2E_DATA_CAPV_VER ?= release-1.0

all: run

$(TOOLING_BINARIES):
	make -C $(TOOLS_DIR) $(@F)

$(TMP_CAPV_DIR):
	git clone -b $(E2E_DATA_CAPV_VER) git@github.com:kubernetes-sigs/cluster-api-provider-vsphere.git $(TMP_CAPV_DIR)

$(E2E_DATA_DIR): $(TMP_CAPV_DIR)
	cp -r $(TMP_CAPV_DIR)/test/e2e/data $(E2E_DATA_DIR) && \
	cp $(TMP_CAPV_DIR)/metadata.yaml $(E2E_DATA_DIR)

run: $(TOOLING_BINARIES) $(E2E_DATA_DIR)
	$(GINKGO) -v . -- --e2e.config="$(E2E_CONF_FILE)" --e2e.artifacts-folder="$(E2E_ARTIFACTS)" --e2e.skip-resource-cleanup=true

clean:
	rm -rf $(E2E_DATA_DIR) $(TMP_CAPV_DIR)
