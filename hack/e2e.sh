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
  kubectl --kubeconfig="${KUBECONFIG}" delete "ipaddressclaim.ipam.cluster.x-k8s.io" "${CONTROL_PLANE_IPCLAIM_NAME}" || true
  kubectl --kubeconfig="${KUBECONFIG}" delete "ipaddressclaim.ipam.cluster.x-k8s.io" "${WORKLOAD_IPCLAIM_NAME}" || true

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
  gcr.io/k8s-staging-capi-vsphere/extra/openvpn:latest

# Tail the vpn logs
docker logs vpn

# Sleep to allow vpn container to start running
sleep 30

function kubectl_get_jsonpath() {
  local OBJECT_KIND="${1}"
  local OBJECT_NAME="${2}"
  local JSON_PATH="${3}"
  local n=0
  until [ $n -ge 30 ]; do
    OUTPUT=$(kubectl --kubeconfig="${KUBECONFIG}" get "${OBJECT_KIND}.ipam.cluster.x-k8s.io" "${OBJECT_NAME}" -o=jsonpath="${JSON_PATH}")
    if [[ "${OUTPUT}" != "" ]]; then
      break
    fi
    n=$((n + 1))
    sleep 1
  done

  if [[ "${OUTPUT}" == "" ]]; then
    echo "Received empty output getting ${JSON_PATH} from ${OBJECT_KIND}/${OBJECT_NAME}" 1>&2
    return 1
  else
    echo "${OUTPUT}"
    return 0
  fi
}

function claim_ip() {
  IPCLAIM_NAME="$1"
  export IPCLAIM_NAME
  sed \
    -e "s/\${IPCLAIM_NAME}/${IPCLAIM_NAME}/" \
    -e "s/\${BUILD_ID}/${BUILD_ID}/" \
    -e "s/\${JOB_NAME}/${JOB_NAME}/" \
    "${REPO_ROOT}/hack/ipclaim-template.yaml" | kubectl --kubeconfig="${KUBECONFIG}" create -f - 1>&2
  IPADDRESS_NAME=$(kubectl_get_jsonpath ipaddressclaim "${IPCLAIM_NAME}" '{@.status.addressRef.name}')
  kubectl --kubeconfig="${KUBECONFIG}" get "ipaddresses.ipam.cluster.x-k8s.io" "${IPADDRESS_NAME}" -o=jsonpath='{@.spec.address}'
}

export KUBECONFIG="/root/ipam-conf/capv-services.conf"


# Retrieve an IP to be used as the kube-vip IP
CONTROL_PLANE_IPCLAIM_NAME="ip-claim-$(openssl rand -hex 20)"
CONTROL_PLANE_ENDPOINT_IP=$(claim_ip "${CONTROL_PLANE_IPCLAIM_NAME}")

# Retrieve an IP to be used for the workload cluster in v1a3/v1a4 -> v1b1 upgrade tests
WORKLOAD_IPCLAIM_NAME="workload-ip-claim-$(openssl rand -hex 20)"
WORKLOAD_CONTROL_PLANE_ENDPOINT_IP=$(claim_ip "${WORKLOAD_IPCLAIM_NAME}")

export CONTROL_PLANE_ENDPOINT_IP
export WORKLOAD_CONTROL_PLANE_ENDPOINT_IP

GCR_KEY_FILE="${GCR_KEY_FILE:-}"
login

make e2e
