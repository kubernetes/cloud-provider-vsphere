#!/bin/sh

# Copyright 2018 The Kubernetes Authors.
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

# posix compliant
# verified by https://www.shellcheck.net

set -o nounset
set -o errexit
! /bin/sh -c 'set -o pipefail' >/dev/null 2>&1 || set -o pipefail

# Ensure the Docker socket is bind mounted into the container.
DOCKER_SOCK="${DOCKER_SOCK:-/var/run/docker.sock}"
[ -S "${DOCKER_SOCK}" ] || { echo "required: ${DOCKER_SOCK}" 1>&2; exit 1; }

# If a configuration file was provided then load it into this process.
if [ -f "${CONFIG_ENV-}" ]; then
  # shellcheck disable=1090
  set -o allexport && . "${CONFIG_ENV-}" && set +o allexport
fi

# require VAR1 [VAR2 VAR3 ...]
#   exits with an error if the provided environment variable names are undefined
require() {
  while [ -n "${1-}" ]; do
    { [ -n "$(eval "echo \${${1}}")" ] && shift; } || \
    { echo "${1} required" 1>&2; exit 1; }
  done
}

# Exit with an error if any of the environment variables below are undefined.
require VSPHERE_SERVER \
        VSPHERE_USERNAME \
        VSPHERE_PASSWORD \
        VSPHERE_DATACENTER \
        VSPHERE_DATASTORE \
        VSPHERE_FOLDER \
        VSPHERE_RESOURCE_POOL

# Export the environment variables for govc.
export  GOVC_URL="${GOVC_URL:-https://${VSPHERE_SERVER-}/sdk}" \
        GOVC_USERNAME="${GOVC_USERNAME:-${VSPHERE_USERNAME-}}" \
        GOVC_PASSWORD="${GOVC_PASSWORD:-${VSPHERE_PASSWORD-}}" \
        GOVC_DATACENTER="${GOVC_DATACENTER:-${VSPHERE_DATACENTER-}}" \
        GOVC_DATASTORE="${GOVC_DATASTORE:-${VSPHERE_DATASTORE-}}" \
        GOVC_FOLDER="${GOVC_FOLDER:-${VSPHERE_FOLDER-}}" \
        GOVC_RESOURCE_POOL="${GOVC_RESOURCE_POOL:-${VSPHERE_RESOURCE_POOL-}}"

# Export the vSphere credentials for Terraform.
export  TF_VAR_vsphere_server="${VSPHERE_SERVER-}" \
        TF_VAR_vsphere_user="${VSPHERE_USERNAME-}" \
        TF_VAR_vsphere_password="${VSPHERE_PASSWORD-}"

# Configure the external cloud provider.
export TF_VAR_cloud_provider="${CLOUD_PROVIDER:-external}"

# Configure the version of Kubernetes used to turn up the cluster.
export TF_VAR_k8s_version="${K8S_VERSION:-ci/latest}"

# Configure the e2e image and tests that are run and/or skipped.
export  KUBE_CONFORMANCE_IMAGE="${KUBE_CONFORMANCE_IMAGE:-gcr.io/heptio-images/kube-conformance:latest}" \
        E2E_FOCUS="${E2E_FOCUS:-\\\[Conformance\\\]}" \
        E2E_SKIP="${E2E_SKIP:-Alpha|\\\[(Disruptive|Feature:[^\\\]]+|Flaky)\\\]}"

# Configure the shape of the cluster.
export TF_VAR_ctl_count="${NUM_CONTROLLERS:-2}" \
       TF_VAR_wrk_count="${NUM_WORKERS:-3}"

# Mark both controller nodes as workers as well.
export TF_VAR_bth_count="${NUM_BOTH:-${TF_VAR_ctl_count}}"

# The cluster name is a combination of the build ID and the first seven
# characters of a hash of the job ID.
CLUSTER_NAME="prow-$(echo "${BUILD_ID:-1}-${PROW_JOB_ID:-$(date +%s)}" | { md5sum 2>/dev/null || md5; } | awk '{print $1}' | cut -c-7)"

# Write information about the build out to disk.
cat <<EOF >"${ARTIFACTS-}/build-info.json"
{
  "cluster-name": "${CLUSTER_NAME}",
  "k8s-version": "${TF_VAR_k8s_version}",
  "num-both": "${TF_VAR_bth_count}",
  "num-controllers": "${TF_VAR_ctl_count}",
  "num-workers": "${TF_VAR_wrk_count}",
  "cloud-provider": "${TF_VAR_cloud_provider}",
  "e2e-focus": "${E2E_FOCUS}",
  "e2e-skip": "${E2E_SKIP}",
  "kube-conformance-image": "${KUBE_CONFORMANCE_IMAGE}",
  "config-env": "${CONFIG_ENV}"
}
EOF

# If the first argument is simply "shell" then drop to a shell.
echo "${1-}" | grep -qF shell && exec /bin/bash

# Switch contexts to the Terraform project.
cd /tf || { echo "required: /tf" 1>&2; exit 1; }

# Execute the Terraform project's "prow" target to:
#
# 1. Turn up a cluster
# 2. Print the Kubernetes client and server versions
# 3. Use Sonobuoy to schedule the e2e conformance tests on the cluster
# 4. Follow the test logs in real-time until the tests are complete
# 5. Retrieve the test results and place them in the directory defined by
#    the environment variable ARTIFACTS
# 6. Destroy the cluster
#
# The command will exit with an exit code of 0 to indicate success. A non-zero
# exit code may be returned by any of the sub-operations, causing the command
# to fail. However, step six -- destroying the cluster -- is always attempted
# whether or not steps one through five were successful.
./entrypoint.sh "${CLUSTER_NAME}" prow
