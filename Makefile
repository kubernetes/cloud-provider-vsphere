# golang-client Makefile
# Follows the interface defined in the Golang CTI proposed
# in https://review.openstack.org/410355

all: build
#REPO_VERSION?=$(shell git describe --tags)

GIT_HOST = k8s.io

PWD := $(shell pwd)
BASE_DIR := $(shell basename $(PWD))
# Keep an existing GOPATH, make a private one if it is undefined
GOPATH_DEFAULT := $(PWD)/.go
export GOPATH ?= $(GOPATH_DEFAULT)
GOBIN_DEFAULT := $(GOPATH)/bin
export GOBIN ?= $(GOBIN_DEFAULT)
TESTARGS_DEFAULT := "-v"
export TESTARGS ?= $(TESTARGS_DEFAULT)
PKG := $(shell awk  -F "\"" '/^ignored = / { print $$2 }' Gopkg.toml)
DEST := $(GOPATH)/src/$(GIT_HOST)/$(BASE_DIR)
SOURCES := $(shell find . -name *.go -not -path "./vendor/**/*")
HAS_MERCURIAL := $(shell command -v hg;)
HAS_DEP := $(shell command -v dep;)
HAS_LINT := $(shell command -v golint;)
HAS_GOX := $(shell command -v gox;)
GOX_PARALLEL ?= 3
TARGETS ?= darwin/amd64 linux/amd64 linux/386 linux/arm linux/arm64 linux/ppc64le
DIST_DIRS         = find * -type d -exec

GOOS ?= $(shell go env GOOS)
VERSION ?= $(shell git describe --exact-match 2> /dev/null || \
                 git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
GOFLAGS   :=
TAGS      :=
LDFLAGSCCM := "-w -s -X 'main.version=${VERSION}'"
LDFLAGSCSI := "-w -s -X 'k8s.io/cloud-provider-vsphere/pkg/csi/service.version=${VERSION}'"
REGISTRY ?= gcr.io/cloud-provider-vsphere
PUSH_LATEST ?= TRUE

ifneq ("$(DEST)", "$(PWD)")
    $(error Please run 'make' from $(DEST). Current directory is $(PWD))
endif

# CTI targets

$(GOBIN):
	@echo "create gobin"
	mkdir -p $(GOBIN)

vendor: | $(GOBIN)
ifeq (0,$(shell { test ! -d vendor || test vendor -ot Gopkg.lock; } && echo 0))
vendor:
	@$(MAKE) --always-make Gopkg.lock
.PHONY: vendor
else
vendor: Gopkg.lock
endif
vendor-update: | $(GOBIN)
	@DEP_FLAGS=" -update" $(MAKE) --always-make Gopkg.lock
Gopkg.lock: Gopkg.toml $(SOURCES)
ifndef HAS_MERCURIAL
	pip install Mercurial
endif
ifndef HAS_DEP
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif
	dep ensure -v$(DEP_FLAGS) && touch vendor

build: vsphere-cloud-controller-manager vsphere-csi

vsphere-cloud-controller-manager: vendor $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGSCCM) \
		-o vsphere-cloud-controller-manager \
		cmd/vsphere-cloud-controller-manager/main.go

vsphere-csi: vendor $(SOURCES)
	CGO_ENABLED=0 GOOS=$(GOOS) go build \
		-ldflags $(LDFLAGSCSI) \
		-o vsphere-csi \
		cmd/vsphere-csi/main.go

test: unit

check: vendor fmt vet lint

unit: vendor
	go test -tags=unit $(shell go list ./...) $(TESTARGS)

fmt:
	hack/verify-gofmt.sh

lint:
ifndef HAS_LINT
		go get -u github.com/golang/lint/golint
		echo "installing lint"
endif
	hack/verify-golint.sh

vet:
	go vet ./...

cover: vendor
	go test -tags=unit $(shell go list ./...) -cover

docs:
	@echo "$@ not yet implemented"

godoc:
	@echo "$@ not yet implemented"

releasenotes:
	@echo "Reno not yet implemented for this repo"

translation:
	@echo "$@ not yet implemented"

# Do the work here

# Set up the development environment
env:
	@echo "PWD: $(PWD)"
	@echo "BASE_DIR: $(BASE_DIR)"
	@echo "GOPATH: $(GOPATH)"
	@echo "GOROOT: $(GOROOT)"
	@echo "DEST: $(DEST)"
	@echo "PKG: $(PKG)"
	go version
	go env

# Get our dev/test dependencies in place
bootstrap:
	tools/test-setup.sh

.bindep:
	virtualenv .bindep
	.bindep/bin/pip install -i https://pypi.python.org/simple bindep

bindep: .bindep
	@.bindep/bin/bindep -b -f bindep.txt || true

install-distro-packages:
	tools/install-distro-packages.sh

clean:
	rm -rf _dist .bindep vsphere-cloud-controller-manager vsphere-csi

realclean: clean
	rm -rf vendor
	if [ "$(GOPATH)" = "$(GOPATH_DEFAULT)" ]; then \
		rm -rf $(GOPATH); \
	fi

shell:
	$(SHELL) -i

images: image-controller-manager

image-controller-manager: vendor vsphere-cloud-controller-manager
ifeq ($(GOOS),linux)
	cp vsphere-cloud-controller-manager cluster/images/controller-manager
	docker build -t $(REGISTRY)/vsphere-cloud-controller-manager:$(VERSION) cluster/images/controller-manager
ifeq ("$(PUSH_LATEST)","TRUE")
	docker tag $(REGISTRY)/vsphere-cloud-controller-manager:$(VERSION) $(REGISTRY)/vsphere-cloud-controller-manager:latest
endif
	rm cluster/images/controller-manager/vsphere-cloud-controller-manager
else
	$(error Please set GOOS=linux for building the image)
endif

upload-images: images
	@echo "push images to $(REGISTRY)"

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
	docker push $(REGISTRY)/vsphere-cloud-controller-manager:$(VERSION)
ifeq ("$(PUSH_LATEST)","TRUE")
	docker push $(REGISTRY)/vsphere-cloud-controller-manager:latest
endif

version:
	@echo ${VERSION}

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross: vendor
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif
	CGO_ENABLED=0 gox -parallel=$(GOX_PARALLEL) -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) $(if $(TAGS),-tags '$(TAGS)',) -ldflags '$(LDFLAGS)' $(GIT_HOST)/$(BASE_DIR)/cmd/vsphere-cloud-controller-manager/

.PHONY: dist
dist: build-cross
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) tar -zcf cloud-provider-vsphere-$(VERSION)-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r cloud-provider-vsphere-$(VERSION)-{}.zip {} \; \
	)

.PHONY: bindep build clean cover docs fmt lint realclean \
	relnotes test translation version build-cross dist vendor-update
