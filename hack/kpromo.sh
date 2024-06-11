#!/bin/bash

# Copyright 2024 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# ========== install kpromo ==========
KPROMO_VER=v4.0.5
KPROMO_PKG=sigs.k8s.io/promo-tools/v4/cmd/kpromo

go install ${KPROMO_PKG}@${KPROMO_VER}

KPROMO_BIN="$(go env GOPATH)/bin/kpromo"

# ========== find user's fork ==========
USER_FORK=${1:-}
if [ -z "${USER_FORK}" ]; then
    # for git@github.com:<username>/cloud-provider-vsphere.git style URLs
    USER_FORK=$(git config --get remote.origin.url | cut -d: -f2 | cut -d/ -f1)
fi
if [ -z "${USER_FORK}" ]; then
    # only works on https://github.com/<username>/cluster-api.git style URLs
    USER_FORK=$(git config --get remote.origin.url | cut -d/ -f4) 
fi

# ========== extract all the reviewers ==========
APPROVERS_URL="https://raw.githubusercontent.com/kubernetes/k8s.io/main/registry.k8s.io/images/k8s-staging-cloud-pv-vsphere/OWNERS"
REVIEWERS_ARR=$(curl -sSL "${APPROVERS_URL}" | sed -n '/approvers:/,/^$/ {/^approvers:/!p;}' | sed 's/^- //g' | tr -s '\n' )  
REVIEWERS=$(echo "${REVIEWERS_ARR}" | awk '{print "@" $0}' | tr '\n' ' ' ) 

# ========== git current tag ==========
TAG=$(git describe --always)

# ========== run kpromo command to sumbit PR ==========
GCP_PROJECT=cloud-pv-vsphere
IMAGE_NAME=cloud-pv-vsphere
KPROMO_CMD="${KPROMO_BIN} pr --fork ${USER_FORK} --project ${GCP_PROJECT} --reviewers ${REVIEWERS} --tag ${TAG} --image ${IMAGE_NAME}"
echo "Run KPROMO Command: ${KPROMO_CMD}"
$KPROMO_CMD
