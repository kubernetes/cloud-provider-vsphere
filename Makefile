all: build

# Get the absolute path and name of the current directory.
PWD := $(abspath .)
BASE_DIR := $(notdir $(PWD))

# PROJECT_ROOT is used when host access is required when running
# Docker-in-Docker (DinD).
export PROJECT_ROOT ?= $(PWD)

# BUILD_OUT is the root directory containing the build output.
export BUILD_OUT ?= .build

# BIN_OUT is the directory containing the built binaries.
export BIN_OUT ?= $(BUILD_OUT)/bin

# ARTIFACTS is the directory containing artifacts uploaded to the Kubernetes
# test grid at the end of a Prow job.
export ARTIFACTS ?= $(BUILD_OUT)/artifacts

-include hack/make/docker.mk

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
##                                VERSIONS                                    ##
################################################################################
# Ensure the version is injected into the binaries via a linker flag.
export VERSION ?= $(shell git describe --always --dirty)

# Load the image registry include.
include hack/make/login-to-image-registry.mk

# Define the images.
IMAGE_CCM := $(REGISTRY)/vsphere-cloud-controller-manager

.PHONY: version print-ccm-image

version:
	@echo $(VERSION)

# Printing the image versions are defined early so Go modules aren't forced.
print-ccm-image:
	@echo $(IMAGE_CCM):$(VERSION)

################################################################################
##                              BUILD BINARIES                                ##
################################################################################
# Unless otherwise specified the binaries should be built for linux-amd64.
GOOS ?= linux
GOARCH ?= amd64

LDFLAGS := $(shell cat hack/make/ldflags.txt)
LDFLAGS_CCM := $(LDFLAGS) -X "main.version=$(VERSION)"

# The cloud controller binary.
CCM_BIN_NAME := vsphere-cloud-controller-manager
CCM_BIN := $(BIN_OUT)/$(CCM_BIN_NAME).$(GOOS)_$(GOARCH)
build-ccm: $(CCM_BIN)
ifndef CCM_BIN_SRCS
CCM_BIN_SRCS := cmd/$(CCM_BIN_NAME)/main.go go.mod go.sum
CCM_BIN_SRCS += $(addsuffix /*.go,$(shell go list -f '{{ join .Deps "\n" }}' ./cmd/$(CCM_BIN_NAME) | grep $(MOD_NAME) | sed 's~$(MOD_NAME)~.~'))
export CCM_BIN_SRCS
endif
$(CCM_BIN): $(CCM_BIN_SRCS)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags '$(LDFLAGS_CCM)' -o $(abspath $@) $<
	@touch $@

# The default build target.
build build-bins: $(CCM_BIN)
build-with-docker:
	hack/make.sh

################################################################################
##                                   DIST                                     ##
################################################################################
DIST_CCM_NAME := cloud-provider-vsphere-$(VERSION)
DIST_CCM_TGZ := $(BUILD_OUT)/dist/$(DIST_CCM_NAME)-$(GOOS)_$(GOARCH).tar.gz
dist-ccm-tgz: $(DIST_CCM_TGZ)
$(DIST_CCM_TGZ): $(CCM_BIN)
	_temp_dir=$$(mktemp -d) && cp $< "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	tar czf $(abspath $@) README.md LICENSE -C "$${_temp_dir}" "$(CCM_BIN_NAME)" && \
	rm -fr "$${_temp_dir}"

DIST_CCM_ZIP := $(BUILD_OUT)/dist/$(DIST_CCM_NAME)-$(GOOS)_$(GOARCH).zip
dist-ccm-zip: $(DIST_CCM_ZIP)
$(DIST_CCM_ZIP): $(CCM_BIN)
	_temp_dir=$$(mktemp -d) && cp $< "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	zip -j $(abspath $@) README.md LICENSE "$${_temp_dir}/$(CCM_BIN_NAME)" && \
	rm -fr "$${_temp_dir}"

dist-ccm: dist-ccm-tgz dist-ccm-zip

dist: dist-ccm

################################################################################
##                                DEPLOY                                      ##
################################################################################
# The deploy target is for use by Prow.
.PHONY: deploy
deploy: | $(DOCKER_SOCK)
	$(MAKE) check
	$(MAKE) build-bins
	$(MAKE) unit-test
	$(MAKE) build-images
	$(MAKE) integration-test
	$(MAKE) push-bins
	$(MAKE) push-images

################################################################################
##                                 CLEAN                                      ##
################################################################################
.PHONY: clean
clean:
	@rm -f Dockerfile*
	@rm -f $(CCM_BIN) cloud-provider-vsphere-*.tar.gz cloud-provider-vsphere-*.zip \
		image-*.tar image-*.d
	GO111MODULE=off go clean -i -x . ./cmd/$(CCM_BIN_NAME)

.PHONY: clean-d
clean-d:
	@find . -name "*.d" -type f -delete

################################################################################
##                                CROSS BUILD                                 ##
################################################################################

# Defining X_BUILD_DISABLED prevents the cross-build and cross-dist targets
# from being defined. This is to improve performance when invoking x-build
# or x-dist targets that invoke this Makefile. The nested call does not need
# to provide cross-build or cross-dist targets since it's the result of one.
ifndef X_BUILD_DISABLED

export X_BUILD_DISABLED := 1

# Modify this list to add new cross-build and cross-dist targets.
X_TARGETS ?= darwin_amd64 linux_amd64 linux_386 linux_arm linux_arm64 linux_ppc64le

X_TARGETS := $(filter-out $(GOOS)_$(GOARCH),$(X_TARGETS))

X_CCM_BINS := $(addprefix $(CCM_BIN_NAME).,$(X_TARGETS))
$(X_CCM_BINS):
	GOOS=$(word 1,$(subst _, ,$(subst $(CCM_BIN_NAME).,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(CCM_BIN_NAME).,,$@))) $(MAKE) build-ccm

x-build-ccm: $(CCM_BIN) $(X_CCM_BINS)

x-build: x-build-ccm

################################################################################
##                                CROSS DIST                                  ##
################################################################################

X_DIST_CCM_TARGETS := $(X_TARGETS)
X_DIST_CCM_TARGETS := $(addprefix $(DIST_CCM_NAME)-,$(X_DIST_CCM_TARGETS))
X_DIST_CCM_TGZS := $(addsuffix .tar.gz,$(X_DIST_CCM_TARGETS))
X_DIST_CCM_ZIPS := $(addsuffix .zip,$(X_DIST_CCM_TARGETS))
$(X_DIST_CCM_TGZS):
	GOOS=$(word 1,$(subst _, ,$(subst $(DIST_CCM_NAME)-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(DIST_CCM_NAME)-,,$(subst .tar.gz,,$@)))) $(MAKE) dist-ccm-tgz
$(X_DIST_CCM_ZIPS):
	GOOS=$(word 1,$(subst _, ,$(subst $(DIST_CCM_NAME)-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst $(DIST_CCM_NAME)-,,$(subst .zip,,$@)))) $(MAKE) dist-ccm-zip

x-dist-ccm-tgzs: $(DIST_CCM_TGZ) $(X_DIST_CCM_TGZS)
x-dist-ccm-zips: $(DIST_CCM_ZIP) $(X_DIST_CCM_ZIPS)
x-dist-ccm: x-dist-ccm-tgzs x-dist-ccm-zips

x-dist: x-dist-ccm

################################################################################
##                               CROSS CLEAN                                  ##
################################################################################

X_CLEAN_TARGETS := $(addprefix clean-,$(X_TARGETS))
.PHONY: $(X_CLEAN_TARGETS)
$(X_CLEAN_TARGETS):
	GOOS=$(word 1,$(subst _, ,$(subst clean-,,$@))) GOARCH=$(word 2,$(subst _, ,$(subst clean-,,$@))) $(MAKE) clean

.PHONY: x-clean
x-clean: clean $(X_CLEAN_TARGETS)

endif # ifndef X_BUILD_DISABLED

################################################################################
##                                 TESTING                                    ##
################################################################################
ifndef PKGS_WITH_TESTS
export PKGS_WITH_TESTS := $(sort $(shell find . -name "*_test.go" -type f -exec dirname \{\} \;))
endif
TEST_FLAGS ?= -v
.PHONY: unit build-unit-tests
unit unit-test:
	env -u VSPHERE_SERVER -u VSPHERE_PASSWORD -u VSPHERE_USER go test $(TEST_FLAGS) -tags=unit $(PKGS_WITH_TESTS)
build-unit-tests:
	$(foreach pkg,$(PKGS_WITH_TESTS),go test $(TEST_FLAGS) -c -tags=unit $(pkg); )

# The default test target.
.PHONY: test build-tests
test: unit
build-tests: build-unit-tests

.PHONY: cover
cover: TEST_FLAGS += -cover
cover: test

.PHONY: integration-test
integration-test: | $(DOCKER_SOCK)
	$(MAKE) -C test/integration

.PHONY: conformance-test
conformance-test: | $(DOCKER_SOCK)
ifeq (true,$(DOCKER_IN_DOCKER_ENABLED))
	hack/images/ci/conformance.sh
else
	docker run -it --rm  \
	  -e "PROJECT_ROOT=$(PROJECT_ROOT)"  \
	  -v $(DOCKER_SOCK):$(DOCKER_SOCK) \
	  -v "$(PWD)":/go/src/k8s.io/cloud-provider-vsphere \
	  -e "CLOUD_PROVIDER=$${CLOUD_PROVIDER}" \
	  -e "E2E_FOCUS=$${E2E_FOCUS}" \
	  -e "E2E_SKIP=$${E2E_SKIP}" \
	  -e "K8S_VERSION=$${K8S_VERSION}" \
	  -e "KUBE_CONFORMANCE_IMAGE=$${KUBE_CONFORMANCE_IMAGE:-akutz/kube-conformance:latest}" \
	  -e "NUM_BOTH=$${NUM_BOTH}" \
	  -e "NUM_CONTROLLERS=$${NUM_CONTROLLERS}" \
	  -e "NUM_WORKERS=$${NUM_WORKERS}" \
	  -e "ARTIFACTS=/artifacts"    -v "$(abspath $(ARTIFACTS))":/artifacts \
	  -e "CONFIG_ENV=/config.env"  -v "$${CONFIG_ENV:-$(abspath config.env)}":/config.env:ro \
	  gcr.io/cloud-provider-vsphere/ci \
	  make conformance-test
endif

.PHONY: quick-conformance-test
quick-conformance-test: export NUM_CONTROLLERS=1
quick-conformance-test: export NUM_WORKERS=1
quick-conformance-test: export E2E_FOCUS=should provide DNS for the cluster[[:space:]]{0,}\\[Conformance\\]
quick-conformance-test: conformance-test

################################################################################
##                                 LINTING                                    ##
################################################################################
.PHONY: fmt vet lint
fmt:
	hack/check-format.sh

vet:
	hack/check-vet.sh

lint:
	hack/check-lint.sh

.PHONY: check
check:
	JUNIT_REPORT="$(abspath $(ARTIFACTS)/junit_check.xml)" hack/check.sh

.PHONY: mdlint
mdlint:
	hack/check-mdlint.sh

.PHONY: shellcheck
shellcheck:
	hack/check-shell.sh

################################################################################
##                                 BUILD IMAGES                               ##
################################################################################
IMAGE_CCM_D := image-ccm-$(VERSION).d
build-ccm-image ccm-image: $(IMAGE_CCM_D)
$(IMAGE_CCM): $(IMAGE_CCM_D)
ifneq ($(GOOS),linux)
$(IMAGE_CCM_D):
	$(error Please set GOOS=linux for building $@)
else
$(IMAGE_CCM_D): $(CCM_BIN) | $(DOCKER_SOCK)
	cp -f $< cluster/images/controller-manager/vsphere-cloud-controller-manager
	docker build -t $(IMAGE_CCM):$(VERSION) cluster/images/controller-manager
	docker tag $(IMAGE_CCM):$(VERSION) $(IMAGE_CCM):latest
	@rm -f cluster/images/controller-manager/vsphere-cloud-controller-manager && touch $@
endif

build-images images: build-ccm-image

################################################################################
##                                  PUSH IMAGES                               ##
################################################################################
.PHONY: push-$(IMAGE_CCM) upload-$(IMAGE_CCM)
push-ccm-image upload-ccm-image: upload-$(IMAGE_CCM)
push-$(IMAGE_CCM) upload-$(IMAGE_CCM): $(IMAGE_CCM_D) login-to-image-registry | $(DOCKER_SOCK)
	docker push $(IMAGE_CCM):$(VERSION)
	docker push $(IMAGE_CCM):latest

.PHONY: push-images upload-images
push-images upload-images: upload-ccm-image

################################################################################
##                                  PUSH BINS                                 ##
################################################################################
.PHONY: push-bins
push-bins: build-bins
	gsutil cp $(CCM_BIN) gs://artifacts.cloud-provider-vsphere.appspot.com/binaries/$(VERSION)/

################################################################################
##                                  CI IMAGE                                  ##
################################################################################
build-ci-image:
	$(MAKE) -C hack/images/ci build

push-ci-image:
	$(MAKE) -C hack/images/ci push

print-ci-image:
	@$(MAKE) --no-print-directory -C hack/images/ci print

################################################################################
##                                TODO(akutz)                                 ##
################################################################################
TODO := docs godoc releasenotes translation
.PHONY: $(TODO)
$(TODO):
	@echo "$@ not yet implemented"
