#!/bin/bash

# Copyright 2022 The Kubernetes Authors.
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

set -o errexit  # exits immediately on any unexpected error (does not bypass traps)
set -o nounset  # will error if variables are used without first being defined
set -o pipefail # any non-zero exit code in a piped command causes the pipeline to fail with that code

export PATH=${PWD}/hack/tools/bin:${PATH}
REPO_ROOT=$(git rev-parse --show-toplevel)

on_exit() {
  # release IPClaim
  echo "Releasing IP claims"
  kubectl --kubeconfig="${KUBECONFIG}" delete "$(append_api_group ipclaim)" "${IPCLAIM_NAME}" || true
  kubectl --kubeconfig="${KUBECONFIG}" delete "$(append_api_group ipclaim)" "${WORKLOAD_IPCLAIM_NAME}" || true

  # kill the VPN
  docker kill vpn

  # no need to revoke credentials as it is GCE-provided
}

trap on_exit EXIT

function login() {
  # If GCR_KEY_FILE is set, use that service account to login
  if [ "${GCR_KEY_FILE}" ]; then
    gcloud auth activate-service-account --key-file "${GCR_KEY_FILE}" || fatal "unable to login"
  fi
}

# convert vsphere credentials from test-infra to e2e config format
export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"
export VSPHERE_SSH_AUTHORIZED_KEY="${VM_SSH_PUB_KEY}"
export VSPHERE_SSH_PRIVATE_KEY="/root/ssh/.private-key/private-key"

# Run the vpn client in container
docker run --rm -d --name vpn -v "${HOME}/.openvpn/:${HOME}/.openvpn/" \
  -w "${HOME}/.openvpn/" --cap-add=NET_ADMIN --net=host --device=/dev/net/tun \
  gcr.io/cluster-api-provider-vsphere/extra/openvpn:latest

# Tail the vpn logs
docker logs vpn

# Sleep to allow vpn container to start running
sleep 30

function append_api_group() {
  resource=$1
  echo "${resource}.ipam.metal3.io"
}

# Retrieve an IP to be used as the kube-vip IP
KUBECONFIG="/root/ipam-conf/capv-services.conf"

function acquire_ip_for_management_cluster_cp() {
  IPCLAIM_NAME="ip-claim-$(openssl rand -hex 20)"
  sed "s/IPCLAIM_NAME/${IPCLAIM_NAME}/" "${REPO_ROOT}/hack/ipclaim-template.yaml" | kubectl --kubeconfig=${KUBECONFIG} create -f -

  IPADDRESS_NAME=$(kubectl --kubeconfig=${KUBECONFIG} get "$(append_api_group ipclaim)" "${IPCLAIM_NAME}" -o=jsonpath='{@.status.address.name}')
  CONTROL_PLANE_ENDPOINT_IP=$(kubectl --kubeconfig=${KUBECONFIG} get "$(append_api_group ipaddresses)" "${IPADDRESS_NAME}" -o=jsonpath='{@.spec.address}')
  export CONTROL_PLANE_ENDPOINT_IP
  echo "Acquired Control Plane IP: $CONTROL_PLANE_ENDPOINT_IP"
}

function acquire_ip_for_workload_cluster_cp() {
  WORKLOAD_IPCLAIM_NAME="workload-ip-claim-$(openssl rand -hex 20)"
  sed "s/IPCLAIM_NAME/${WORKLOAD_IPCLAIM_NAME}/" "${REPO_ROOT}/hack/ipclaim-template.yaml" | kubectl --kubeconfig=${KUBECONFIG} create -f -

  WORKLOAD_IPADDRESS_NAME=$(kubectl --kubeconfig=${KUBECONFIG} get "$(append_api_group ipclaim)" "${WORKLOAD_IPCLAIM_NAME}" -o=jsonpath='{@.status.address.name}')
  WORKLOAD_CONTROL_PLANE_ENDPOINT_IP=$(kubectl --kubeconfig=${KUBECONFIG} get "$(append_api_group ipaddresses)" "${WORKLOAD_IPADDRESS_NAME}" -o=jsonpath='{@.spec.address}')
  export WORKLOAD_CONTROL_PLANE_ENDPOINT_IP
  echo "Acquired Workload Cluster Control Plane IP: $WORKLOAD_CONTROL_PLANE_ENDPOINT_IP"
}

acquire_ip_for_management_cluster_cp
acquire_ip_for_workload_cluster_cp

GCR_KEY_FILE="${GCR_KEY_FILE:-}"
login

make e2e
