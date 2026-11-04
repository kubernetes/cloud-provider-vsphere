#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
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

# Exercises hack/install-helm.sh without hitting the network.
set -o errexit
set -o nounset
set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT}/hack/install-helm.sh"

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

digest_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

tmp="$(mktemp -d)"
cleanup_test() {
  rm -rf "${tmp}"
}
trap cleanup_test EXIT

mkdir -p "${tmp}/src/linux-amd64"
printf '#!/bin/sh\necho helm-ok\n' >"${tmp}/src/linux-amd64/helm"
chmod +x "${tmp}/src/linux-amd64/helm"
tar -czf "${tmp}/fake.tar.gz" -C "${tmp}/src" linux-amd64
sum="$(digest_file "${tmp}/fake.tar.gz")"

dest="${tmp}/out/helm"
HELM_SKIP_REPO=1 \
  HELM_OS=linux \
  HELM_ARCH=amd64 \
  HELM_SHA256="${sum}" \
  HELM_URL="file://${tmp}/fake.tar.gz" \
  HELM_DEST="${dest}" \
  "${SCRIPT}"

[[ -x "${dest}" ]] || fail "install did not produce helm"
[[ "$(${dest})" == "helm-ok" ]] || fail "installed binary did not run"

bad="${tmp}/bad/helm"
set +e
HELM_SKIP_REPO=1 \
  HELM_OS=linux \
  HELM_ARCH=amd64 \
  HELM_SHA256="0000000000000000000000000000000000000000000000000000000000000000" \
  HELM_URL="file://${tmp}/fake.tar.gz" \
  HELM_DEST="${bad}" \
  "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "bad checksum was accepted"
[[ ! -e "${bad}" ]] || fail "bad checksum still installed a binary"

link="${tmp}/linkdest"
ln -s "${tmp}/src/linux-amd64/helm" "${link}"
set +e
HELM_SKIP_REPO=1 \
  HELM_OS=linux \
  HELM_ARCH=amd64 \
  HELM_SHA256="${sum}" \
  HELM_URL="file://${tmp}/fake.tar.gz" \
  HELM_DEST="${link}" \
  "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "symlink dest was accepted"
[[ -L "${link}" ]] || fail "symlink dest was replaced"

set +e
HELM_SKIP_REPO=1 \
  HELM_OS=windows \
  HELM_ARCH=amd64 \
  HELM_URL="file://${tmp}/fake.tar.gz" \
  HELM_DEST="${tmp}/unused/helm" \
  env -u HELM_SHA256 "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "unknown arch skipped checksum"

echo "ok"
