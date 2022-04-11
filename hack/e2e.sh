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

on_exit() {
  # release IPClaim
  echo "Releasing IP claims"
  kubectl --kubeconfig="${KUBECONFIG}" delete ipclaim "${IPCLAIM_NAME}" || true
  kubectl --kubeconfig="${KUBECONFIG}" delete ipclaim "${WORKLOAD_IPCLAIM_NAME}" || true

  # kill the VPN
  docker kill vpn

  # logout of gcloud
  if [ "${AUTH}" ]; then
    gcloud auth revoke
  fi
}

trap on_exit EXIT

function login() {
  # If GCR_KEY_FILE is set, use that service account to login
  if [ "${GCR_KEY_FILE}" ]; then
    gcloud auth activate-service-account --key-file "${GCR_KEY_FILE}" || fatal "unable to login"
    AUTH=1
  fi
}

# convert vsphere credentials from test-infra to e2e config format
export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"
export VSPHERE_DATACENTER=""

make test-e2e


