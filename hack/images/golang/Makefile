all: build

include ../../../hack/make/login-to-image-registry.mk

VERSION ?= $(shell git describe --exact-match 2>/dev/null || git describe --match=$$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
GO_VERSION ?= $(shell grep '^go:' <"../../../.travis.yml" | sed 's/^\(go:[[:space:]]\{0,\}\"\{0,1\}\)\([^[:space:]\"]\{1,\}\)\(\"\{0,1\}\)$$/\2/')

IMAGE_D := $(GO_VERSION)-$(VERSION).d
%-$(VERSION).d: Dockerfile ../../../go.mod ../../../go.sum
	docker build --build-arg GO_VERSION=$* -t $(REGISTRY)/golang-$*:$(VERSION) -f $< ../../..
	docker tag $(REGISTRY)/golang-$*:$(VERSION) $(REGISTRY)/golang-$*:latest
	@touch $@

build: $(IMAGE_D)
build-%: %-$(VERSION).d
	@true

.PHONY: rebuild
rebuild rebuild-%: MAKEFLAGS += --always-make
rebuild:
	$(MAKE) build
rebuild-%:
	$(MAKE) build-$*

.PHONY: push
push: push-$(GO_VERSION)
push-%: build-% login-to-image-registry
	docker push $(REGISTRY)/golang-$*:$(VERSION)
	docker push $(REGISTRY)/golang-$*:latest

.PHONY: ls-images
ls-images:
	docker images --filter=reference=$(REGISTRY)/golang-*:*

.PHONY: clean
DOCKER_RMI_FLAGS := --no-prune
clean: clean-$(GO_VERSION)
clean-%:
	rm -f $(IMAGE_D)
	docker rmi $(DOCKER_RMI_FLAGS) $(REGISTRY)/golang-$*:$(VERSION) $(REGISTRY)/golang-$*:latest 2>/dev/null || true

.PHONY: clobber
clobber: DOCKER_RMI_FLAGS :=
clobber: clean
	rm -f *.d
	docker rmi $$(docker images -qf reference=$(REGISTRY)/golang-*:*) 2>/dev/null || true

GO_VERSIONS := 1.11 1.11.0 1.11.1 1.11.2 1.11.3 1.11.4 1.11.5
GO_VERSIONS += 1.12 1.12.0
.PHONY: build-all push-all clean-all
build-all: $(addsuffix -$(VERSION).d,$(GO_VERSIONS))
push-all:  $(addprefix push-,$(GO_VERSIONS))
clean-all: $(addprefix clean-,$(GO_VERSIONS))
