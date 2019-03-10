all: build

# Get the absolute path and name of the current directory.
PWD := $(abspath .)
BASE_DIR := $(notdir $(PWD))

################################################################################
##                             VERIFY GO VERSION                              ##
################################################################################
# Go 1.11+ required for Go modules.
GO_VERSION_EXP := "go1.11"
GO_VERSION_ACT := $(shell a="$$(go version | awk '{print $$3}')" && test $$(printf '%s\n%s' "$${a}" "$(GO_VERSION_EXP)" | sort | tail -n 1) = "$${a}" && printf '%s' "$${a}")
ifndef GO_VERSION_ACT
$(error Requires Go $(GO_VERSION_EXP)+ for Go module support)
endif
MOD_NAME := $(shell head -n 1 <go.mod | awk '{print $$2}')

################################################################################
##                             VERIFY BUILD PATH                              ##
################################################################################
ifneq (on,$(GO111MODULE))
export GO111MODULE := on
# The cloud provider should not be cloned inside the GOPATH.
GOPATH := $(shell go env GOPATH)
ifeq (/src/$(MOD_NAME),$(subst $(GOPATH),,$(PWD)))
$(warning This project uses Go modules and should not be cloned into the GOPATH)
endif
endif

################################################################################
##                                DEPENDENCIES                                ##
################################################################################
# Ensure Mercurial is installed.
HAS_MERCURIAL := $(shell command -v hg 2>/dev/null)
ifndef HAS_MERCURIAL
.PHONY: install-hg
install-hg:
	pip install --user Mercurial
deps: install-hg
endif

# Verify the dependencies are in place.
.PHONY: deps
deps:
	go mod download && go mod verify

################################################################################
##                              BUILD BINARIES                                ##
################################################################################
# Unless otherwise specified the binaries should be built for linux-amd64.
GOOS ?= linux
GOARCH ?= amd64

# Ensure the version is injected into the binaries via a linker flag.
VERSION := $(shell git describe --exact-match 2>/dev/null || git describe --match=$$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
LDFLAGS := "-w -s -X 'main.version=$(VERSION)'"

# The cloud controller binary.
CCM_BIN_NAME := vsphere-cloud-controller-manager
CCM_BIN := $(CCM_BIN_NAME).$(GOOS)_$(GOARCH)
$(CCM_BIN): cmd/$(CCM_BIN_NAME)/main.go
$(CCM_BIN): go.mod go.sum
$(CCM_BIN): $(addsuffix /*,$(shell go list -f '{{ join .Deps "\n" }}' ./cmd/$(CCM_BIN_NAME) | grep $(MOD_NAME) | sed 's~$(MOD_NAME)~.~'))
$(CCM_BIN):
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags $(LDFLAGS) -o $@ $<

# The default build target.
build: $(CCM_BIN)
build-with-docker:
	hack/make.sh

################################################################################
##                                CROSS BUILD                                 ##
################################################################################
BUILD_TARGETS ?= darwin_amd64 linux_amd64 linux_386 linux_arm linux_arm64 linux_ppc64le
BUILD_TARGETS := $(filter-out $(GOOS)_$(GOARCH),$(BUILD_TARGETS))
BUILD_TARGETS_BINS := $(addprefix $(CCM_BIN_NAME).,$(BUILD_TARGETS))
$(BUILD_TARGETS_BINS):
	GOOS=$(word 1,$(subst _, ,$(subst $(CCM_BIN_NAME).,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(CCM_BIN_NAME).,,$@))) $(MAKE) build
cross-build: $(CCM_BIN) $(BUILD_TARGETS_BINS)

################################################################################
##                                   DIST                                     ##
################################################################################
DIST_NAME := cloud-provider-vsphere-$(VERSION)
DIST_TGZ := $(DIST_NAME)-$(GOOS)_$(GOARCH).tar.gz
dist-tgz: $(DIST_TGZ)
$(DIST_TGZ): $(CCM_BIN)
	_temp_dir=$$(mktemp -d) && cp $< "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	tar czf $@ README.md LICENSE -C "$${_temp_dir}" "$(CCM_BIN_NAME)" && \
	rm -fr "$${_temp_dir}"

DIST_ZIP := $(DIST_NAME)-$(GOOS)_$(GOARCH).zip
dist-zip: $(DIST_ZIP)
$(DIST_ZIP): $(CCM_BIN)
	_temp_dir=$$(mktemp -d) && cp $< "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	zip -j $@ README.md LICENSE "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	rm -fr "$${_temp_dir}"

dist: $(DIST_ZIP) $(DIST_TGZ)

################################################################################
##                                CROSS DIST                                  ##
################################################################################
DIST_TARGETS := $(BUILD_TARGETS)
DIST_TARGETS := $(addprefix $(DIST_NAME)-,$(DIST_TARGETS))
DIST_TARGETS_TGZS := $(addsuffix .tar.gz,$(DIST_TARGETS))
DIST_TARGETS_ZIPS := $(addsuffix .zip,$(DIST_TARGETS))

$(DIST_TARGETS_TGZS):
	GOOS=$(word 1,$(subst _, ,$(subst $(DIST_NAME)-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(DIST_NAME)-,,$(subst .tar.gz,,$@)))) $(MAKE) dist-tgz
$(DIST_TARGETS_ZIPS):
	GOOS=$(word 1,$(subst _, ,$(subst $(DIST_NAME)-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(DIST_NAME)-,,$(subst .zip,,$@)))) $(MAKE) dist-zip
cross-dist-tgzs: $(DIST_TGZ) $(DIST_TARGETS_TGZS)
cross-dist-zips: $(DIST_ZIP) $(DIST_TARGETS_ZIPS)
cross-dist: cross-dist-tgzs cross-dist-zips

################################################################################
##                                 TESTING                                    ##
################################################################################
PKGS_WITH_TESTS := $(sort $(shell find . -name "*_test.go" -type f -exec dirname \{\} \;))
TEST_FLAGS ?= -v
.PHONY: unit
unit:
	env -u VSPHERE_SERVER -u VSPHERE_PASSWORD -u VSPHERE_USER go test $(TEST_FLAGS) -tags=unit $(PKGS_WITH_TESTS)

# The default test target.
.PHONY: test
test: unit

.PHONY: cover
cover: TEST_FLAGS += -cover
cover: test

################################################################################
##                                 LINTING                                    ##
################################################################################
.PHONY: fmt
fmt:
	find . -name "*.go" | grep -v vendor | xargs gofmt -s -w

.PHONY: vet
vet:
	go vet ./...

HAS_LINT := $(shell command -v golint 2>/dev/null)
.PHONY: lint
lint:
ifndef HAS_LINT
	cd / && GO111MODULE=off go get -u github.com/golang/lint/golint
endif
	go list ./... | xargs golint -set_exit_status | sed 's~$(PWD)~.~'

.PHONY: check
check:
	{ $(MAKE) fmt  || exit_code_1="$${?}"; } && \
	{ $(MAKE) vet  || exit_code_2="$${?}"; } && \
	{ $(MAKE) lint || exit_code_3="$${?}"; } && \
	{ [ -z "$${exit_code_1}" ] || echo "fmt  failed: $${exit_code_1}" 1>&2; } && \
	{ [ -z "$${exit_code_2}" ] || echo "vet  failed: $${exit_code_2}" 1>&2; } && \
	{ [ -z "$${exit_code_3}" ] || echo "lint failed: $${exit_code_3}" 1>&2; } && \
	{ [ -z "$${exit_code_1}" ] || exit "$${exit_code_1}"; } && \
	{ [ -z "$${exit_code_2}" ] || exit "$${exit_code_2}"; } && \
	{ [ -z "$${exit_code_3}" ] || exit "$${exit_code_3}"; } && \
	exit 0

.PHONY: check-warn
check-warn:
	-$(MAKE) check

################################################################################
##                                 BUILD IMAGES                               ##
################################################################################
REGISTRY ?= gcr.io/cloud-provider-vsphere
IMAGE_CONTROLLER_MANAGER := $(REGISTRY)/vsphere-cloud-controller-manager
IMAGE_CONTROLLER_MANAGER_TAR := image-ccm-$(VERSION).tar
ifneq ($(GOOS),linux)
$(IMAGE_CONTROLLER_MANAGER) $(IMAGE_CONTROLLER_MANAGER_TAR):
	$(error Please set GOOS=linux for building the image)
else
$(IMAGE_CONTROLLER_MANAGER) $(IMAGE_CONTROLLER_MANAGER_TAR): $(CCM_BIN)
	cp -f $< cluster/images/controller-manager/vsphere-cloud-controller-manager
	docker build -t $(IMAGE_CONTROLLER_MANAGER):$(VERSION) cluster/images/controller-manager
	docker tag $(IMAGE_CONTROLLER_MANAGER):$(VERSION) $(IMAGE_CONTROLLER_MANAGER):latest
	docker save $(IMAGE_CONTROLLER_MANAGER):$(VERSION) -o $@
	docker save $(IMAGE_CONTROLLER_MANAGER):$(VERSION) -o image-ccm-latest.tar
endif

image image-ccm: $(IMAGE_CONTROLLER_MANAGER_TAR)
images: $(IMAGE_CONTROLLER_MANAGER_TAR)

################################################################################
##                                  PUSH IMAGES                               ##
################################################################################
.PHONY: login-to-image-registry
login-to-image-registry:
# Push images to a gcr.io registry.
ifneq (,$(findstring gcr.io,$(REGISTRY))) # begin gcr.io
# Log into the registry with a gcr.io JSON key file.
ifneq (,$(strip $(GCR_KEY_FILE))) # begin gcr.io-key
	@echo "logging into gcr.io registry with key file"
	docker login -u _json_key --password-stdin https://gcr.io <"$(GCR_KEY_FILE)"
# Log into the registry with a Docker gcloud auth helper.
else # end gcr.io-key / begin gcr.io-gcloud
	@command -v gcloud >/dev/null 2>&1 || \
	  { echo 'gcloud auth helper unavailable' 1>&2; exit 1; }
	@grep -F 'gcr.io": "gcloud"' "$(HOME)/.docker/config.json" >/dev/null 2>&1 || \
	  { gcloud auth configure-docker --quiet || \
	    echo 'gcloud helper registration failed' 1>&2; exit 1; }
	@echo "logging into gcr.io registry with gcloud auth helper"
endif # end gcr.io-gcloud / end gcr.io
# Push images to a Docker registry.
else # begin docker
# Log into the registry with a Docker username and password.
ifneq (,$(strip $(DOCKER_USERNAME))) # begin docker-username
ifneq (,$(strip $(DOCKER_PASSWORD))) # begin docker-password
	@echo "logging into docker registry with username and password"
	docker login -u="$(DOCKER_USERNAME)" -p="$(DOCKER_PASSWORD)"
endif # end docker-password
endif # end docker-username
endif # end docker

.PHONY: upload-$(IMAGE_CONTROLLER_MANAGER)
upload-ccm-image: upload-$(IMAGE_CONTROLLER_MANAGER)
upload-$(IMAGE_CONTROLLER_MANAGER): $(IMAGE_CONTROLLER_MANAGER_TAR)
	docker push $(IMAGE_CONTROLLER_MANAGER):$(VERSION)
	docker push $(IMAGE_CONTROLLER_MANAGER):latest

.PHONY: upload-images
upload-images: images
	@echo "push images to $(REGISTRY)"
	$(MAKE) login-to-image-registry
	$(MAKE) upload-$(IMAGE_CONTROLLER_MANAGER)

################################################################################
##                               PRINT VERISON                                ##
################################################################################
.PHONY: version
version:
	@echo $(VERSION)

################################################################################
##                                 CLEAN                                      ##
################################################################################
.PHONY: clean
clean:
	@rm -f $(CCM_BIN) cloud-provider-vsphere-*.tar.gz cloud-provider-vsphere-*.zip image-ccm-*.tar image-ccm-latest.tar
	GO111MODULE=off go clean -i -x

CLEAN_TARGETS := $(addprefix clean-,$(BUILD_TARGETS))
.PHONY: $(CLEAN_TARGETS)
$(CLEAN_TARGETS):
	GOOS=$(word 1,$(subst _, ,$(subst clean-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst clean-,,$@))) $(MAKE) clean

.PHONY: cross-clean
cross-clean: clean $(CLEAN_TARGETS)

################################################################################
##                                TODO(akutz)                                 ##
################################################################################
TODO := docs godoc releasenotes translation
.PHONY: $(TODO)
$(TODO):
	@echo "$@ not yet implemented"
