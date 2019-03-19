all: test

include ../../hack/make/docker.mk

# PROJECT_ROOT may be set externally when this Makefile is invoked with DinD.
PROJECT_ROOT ?= $(abspath ../..)

CLUSTER_NAME := ccm-integration-test
VERSION ?= $(shell git describe --exact-match 2> /dev/null || git describe --match=$(git rev-parse --short=8 HEAD) --always --dirty --abbrev=8)
CCM_IMAGE := gcr.io/cloud-provider-vsphere/vsphere-cloud-controller-manager:$(VERSION)

KIND_CONFIG := kind-config.yaml
.PHONY: $(KIND_CONFIG)
$(KIND_CONFIG): ../kind/kind-config.yaml
	@sed -e 's~{{PROJECT_ROOT}}~$(PROJECT_ROOT)~g' <$< >$@

KUBECONFIG_ORIG ?= $(HOME)/.kube/kind-config-$(CLUSTER_NAME)
KUBECONFIG_DIND ?= $(KUBECONFIG_ORIG)-dind
ifeq (true,$(DOCKER_SOCK_FROM_HOST))
export KUBECONFIG := $(KUBECONFIG_DIND)
else
export KUBECONFIG := $(KUBECONFIG_ORIG)
endif

cluster-up: $(KIND_CONFIG) | $(DOCKER_SOCK)
	kind create cluster --config "$(KIND_CONFIG)" --name "$(CLUSTER_NAME)"
ifeq (true,$(DOCKER_SOCK_FROM_HOST))
	ipv4=$$(docker exec "$(CLUSTER_NAME)-control-plane" ip route get dev eth0 1 | awk '{print $$NF;exit}') && \
	sed -e 's~server: .\{1,\}$$~server: https://'"$${ipv4}":6443'~g' <"$(KUBECONFIG_ORIG)" >"$(KUBECONFIG_DIND)"
endif

cluster-down: | $(DOCKER_SOCK)
	kind delete cluster --name "$(CLUSTER_NAME)"

deploy-vcsim:
	kubectl -n kube-system apply -f ../vcsim/deployment.yaml

create-vms:
	./create-vms.sh "$(CLUSTER_NAME)"

taint-nodes:
	for name in $$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do \
	  kubectl taint nodes "$${name}" node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule; \
	done

delete-vcsim:
	kubectl -n kube-system delete serviceaccount vcsim && \
	kubectl -n kube-system delete statefulset vcsim && \
	kubectl -n kube-system delete service vcsim

deploy-ccm:
	kubectl -n kube-system create configmap cloud-config --from-file=vsphere.conf && \
	kubectl -n kube-system create -f secrets.yaml && \
	kubectl -n kube-system apply -f ../../manifests/controller-manager/cloud-controller-manager-roles.yaml && \
	kubectl -n kube-system apply -f ../../manifests/controller-manager/cloud-controller-manager-role-bindings.yaml && \
	sed 's~gcr.io/cloud-provider-vsphere/vsphere-cloud-controller-manager:latest~$(CCM_IMAGE)~g' <../../manifests/controller-manager/vsphere-cloud-controller-manager-pod.yaml | kubectl -n kube-system apply -f -

delete-ccm:
	kubectl -n kube-system delete configmaps cloud-config && \
	kubectl -n kube-system delete secrets vccm && \
	kubectl -n kube-system delete serviceaccount cloud-controller-manager && \
	kubectl -n kube-system delete pod vsphere-cloud-controller-manager && \
	kubectl -n kube-system delete service vsphere-cloud-controller-manager && \
	kubectl -n kube-system delete ClusterRole system:cloud-controller-manager && \
	kubectl -n kube-system delete ClusterRoleBinding system:cloud-controller-manager

log-ccm:
	while ! kubectl -n kube-system logs vsphere-cloud-controller-manager >/dev/null 2>&1; do sleep 1; done && \
	sleep 3 && \
	kubectl -n kube-system logs vsphere-cloud-controller-manager

build-ccm-image:
	GOOS=linux $(MAKE) -C ../.. build-ccm-image

load-ccm-image: | $(DOCKER_SOCK)
	kind load docker-image --name "$(CLUSTER_NAME)" $(CCM_IMAGE)

test: | $(DOCKER_SOCK)
	$(MAKE) build-ccm-image
	$(MAKE) cluster-up
	$(MAKE) load-ccm-image
	$(MAKE) deploy-vcsim
	$(MAKE) create-vms
	$(MAKE) deploy-ccm
	$(MAKE) taint-nodes
	$(MAKE) log-ccm
	$(MAKE) cluster-down

.PHONY: test cluster-up cluster-down \
		deploy-vcsim delete-vcsim create-vms \
		build-ccm-image load-ccm-image \
		deploy-ccm delete-ccm log-ccm taint-nodes