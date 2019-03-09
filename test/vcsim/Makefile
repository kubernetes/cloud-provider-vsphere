all: build

IMAGE := akutz/vcsim
TAG ?= $(shell date +v%Y%m%d)-$(shell git describe --tags --always --dirty)

build: image
image: $(KEEPALIVE)
	docker build -t "$(IMAGE):$(TAG)" .
	docker tag "$(IMAGE):$(TAG)" "$(IMAGE):latest"

push: image
	docker push "$(IMAGE):$(TAG)"
	docker push "$(IMAGE):latest"

.PHONY: build image push
