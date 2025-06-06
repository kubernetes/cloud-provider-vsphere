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

# In CI, ARTIFACTS is set to a different directory. This stores the value of
# ARTIFACTS in ORIGINAL_ARTIFACTS and replaces ARTIFACTS by a temporary directory
# which gets cleaned up from credentials at the end of the test.
export ORIGINAL_ARTIFACTS=""
export ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
if [[ "${ARTIFACTS}" != "${REPO_ROOT}/_artifacts" ]]; then
  ORIGINAL_ARTIFACTS="${ARTIFACTS}"
  ARTIFACTS=$(mktemp -d)
fi

export BOSKOS_RESOURCE_OWNER=cloud-provider-vsphere
if [[ "${JOB_NAME}" != "" ]]; then
  export BOSKOS_RESOURCE_OWNER="${JOB_NAME}/${BUILD_ID}"
fi
export BOSKOS_RESOURCE_TYPE="gcve-vsphere-project"
# Fallback for mirror-prow.
if [[ "${GOVC_URL:-}" == "10.2.224.4" ]]; then
  export BOSKOS_RESOURCE_TYPE=vsphere-project-cloud-provider
fi

on_exit() {
  # Stop boskos heartbeat
  [[ -z ${HEART_BEAT_PID:-} ]] || kill -9 "${HEART_BEAT_PID}"

  # If Boskos is being used then release the vsphere project.
  [ -z "${BOSKOS_HOST:-}" ] || docker run -e VSPHERE_USERNAME -e VSPHERE_PASSWORD gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest release --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-name="${BOSKOS_RESOURCE_NAME}" --vsphere-server="${VSPHERE_SERVER}" --vsphere-tls-thumbprint="${VSPHERE_TLS_THUMBPRINT}" --vsphere-folder="${BOSKOS_RESOURCE_FOLDER}" --vsphere-resource-pool="${BOSKOS_RESOURCE_POOL}"

  # Cleanup VSPHERE_PASSWORD from temporary artifacts directory.
  if [[ "${ORIGINAL_ARTIFACTS}" != "" ]]; then
    # unpack pod-logs.tar.gz files to replace secrets in them
    find "${ARTIFACTS}" -type f -name pod-logs.tar.gz | while IFS= read -r tarball; do
      echo "Unpacking ${tarball} for secrets replacement"
      mkdir -p "${tarball}-unpacked"
      tar -xzf "${tarball}" -C "${tarball}-unpacked"
      rm "${tarball}"
    done
    # Delete non-text files from artifacts directory to not leak files accidentially
    find "${ARTIFACTS}" -type f -exec file --mime-type {} \; | grep -v -E -e "text/plain|text/xml|application/json|inode/x-empty" | while IFS= read -r line
    do
      file="$(echo "${line}" | cut -d ':' -f1)"
      mimetype="$(echo "${line}" | cut -d ':' -f2)"
      echo "Deleting file ${file} of type ${mimetype}"
      rm "${file}"
    done || true
    # Replace secret and base64 secret in all files.
    if [ -n "$VSPHERE_PASSWORD" ]; then
      grep -I -r -l -e "${VSPHERE_PASSWORD}" "${ARTIFACTS}" | while IFS= read -r file
      do
        echo "Cleaning up VSPHERE_PASSWORD from file ${file}"
        sed -i "s/${VSPHERE_PASSWORD}/REDACTED/g" "${file}"
      done || true
      VSPHERE_PASSWORD_B64=$(echo -n "${VSPHERE_PASSWORD}" | base64 --wrap=0)
      grep -I -r -l -e "${VSPHERE_PASSWORD_B64}" "${ARTIFACTS}" | while IFS= read -r file
      do
        echo "Cleaning up VSPHERE_PASSWORD_B64 from file ${file}"
        sed -i "s/${VSPHERE_PASSWORD_B64}/REDACTED/g" "${file}"
      done || true
    fi
    # re-packing pod-logs.tar.gz-unpacked
    find "${ARTIFACTS}" -type d -name pod-logs.tar.gz-unpacked | while IFS= read -r tarballDirectory; do
      tarball="${tarballDirectory%-unpacked}"
      echo "Packing ${tarballDirectory} to ${tarball} after secrets replacement"
      tar -czf "${tarball}" -C . "${tarballDirectory}"
      rm -r "${tarballDirectory}"
    done
    # Move all artifacts to the original artifacts location.
    mv "${ARTIFACTS}"/* "${ORIGINAL_ARTIFACTS}/"
  fi
}

trap on_exit EXIT

# convert vsphere credentials from test-infra to e2e config format
export VSPHERE_SERVER="${GOVC_URL}"
export VSPHERE_USERNAME="${GOVC_USERNAME}"
export VSPHERE_PASSWORD="${GOVC_PASSWORD}"

SSH_KEY_DIR=$(mktemp -d)
export VSPHERE_SSH_PRIVATE_KEY
VSPHERE_SSH_PRIVATE_KEY="${SSH_KEY_DIR}/ssh-key"
ssh-keygen -t ed25519 -f "${VSPHERE_SSH_PRIVATE_KEY}" -N ""
export VSPHERE_SSH_AUTHORIZED_KEY
VSPHERE_SSH_AUTHORIZED_KEY="$(cat "${VSPHERE_SSH_PRIVATE_KEY}.pub")"

# Fallback for mirror-prow.
if [[ "${GOVC_URL:-}" == "10.2.224.4" ]]; then
  export VSPHERE_SSH_AUTHORIZED_KEY="${VM_SSH_PUB_KEY:-}"
  export VSPHERE_SSH_PRIVATE_KEY="/root/ssh/.private-key/private-key"
fi

# If BOSKOS_HOST is set then acquire a vsphere-project from Boskos.
if [ -n "${BOSKOS_HOST:-}" ]; then
  # Check out the account from Boskos and store the produced environment
  # variables in a temporary file.
  account_env_var_file="$(mktemp)"
  docker run gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest acquire --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-type="${BOSKOS_RESOURCE_TYPE}" 1>"${account_env_var_file}"
  checkout_account_status="${?}"

  # If the checkout process was a success then load the account's
  # environment variables into this process.
  # shellcheck disable=SC1090
  [ "${checkout_account_status}" = "0" ] && . "${account_env_var_file}"
  export BOSKOS_RESOURCE_NAME=${BOSKOS_RESOURCE_NAME}
  export VSPHERE_FOLDER=${BOSKOS_RESOURCE_FOLDER}
  export VSPHERE_RESOURCE_POOL=${BOSKOS_RESOURCE_POOL}
  export CONTROL_PLANE_ENDPOINT_IP="${BOSKOS_RESOURCE_IP_POOL_IP_0}"
  export WORKLOAD_CONTROL_PLANE_ENDPOINT_IP="${BOSKOS_RESOURCE_IP_POOL_IP_1}"

  # Always remove the account environment variable file. It contains
  # sensitive information.
  rm -f "${account_env_var_file}"

  if [ ! "${checkout_account_status}" = "0" ]; then
    echo "error getting vsphere project from Boskos" 1>&2
    exit "${checkout_account_status}"
  fi

  # Run the heartbeat to tell boskos periodically that we are still
  # using the checked out account.
  docker run gcr.io/k8s-staging-capi-vsphere/extra/boskosctl:latest heartbeat --boskos-host="${BOSKOS_HOST}" --resource-owner="${BOSKOS_RESOURCE_OWNER}" --resource-name="${BOSKOS_RESOURCE_NAME}" >>"${ARTIFACTS}/boskos-heartbeat.log" 2>&1 &
  HEART_BEAT_PID=$!
else
  echo "error getting vsphere project from Boskos, BOSKOS_HOST not set" 1>&2
  exit 1
fi

# build cloud-provider-vsphere image and save it as tarball
CPI_IMAGE_NAME="gcr.io/k8s-staging-cloud-pv-vsphere/cloud-provider-vsphere"
GOPROXY="${GOPROXY:-}"

echo "building ${CPI_IMAGE_NAME}:dev"
echo "GOPROXY=${GOPROXY}"
docker build \
  -f cluster/images/controller-manager/Dockerfile \
  -t "${CPI_IMAGE_NAME}:dev" \
  --build-arg "VERSION=dev" \
  --build-arg "GOPROXY=${GOPROXY}" \
  .

export DOCKER_IMAGE_TAR="/tmp/images/image.tar"
mkdir -p /tmp/images

docker save "${CPI_IMAGE_NAME}:dev" > ${DOCKER_IMAGE_TAR}

if [ "$#" -eq 1 ] && [ "$1" = "latest-k8s-version" ]; then
  E2E_ARTIFACTS=${ARTIFACTS} make e2e-latest-k8s-version
else 
  E2E_ARTIFACTS=${ARTIFACTS} make e2e
fi
