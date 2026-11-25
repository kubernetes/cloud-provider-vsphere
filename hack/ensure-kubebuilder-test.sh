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

# Exercises hack/ensure-kubebuilder.sh without hitting the network.
set -o errexit
set -o nounset
set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="${ROOT}/hack/ensure-kubebuilder.sh"

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
name="kubebuilder_2.3.1_linux_amd64"
poison="/tmp/${name}"
created_poison=0
cleanup_test() {
  rm -rf "${tmp}"
  if [[ "${created_poison}" -eq 1 ]]; then
    rm -rf "${poison}"
  fi
}
trap cleanup_test EXIT

mkdir -p "${tmp}/src/${name}/bin"
printf '#!/bin/sh\necho ok\n' >"${tmp}/src/${name}/bin/kubebuilder"
chmod +x "${tmp}/src/${name}/bin/kubebuilder"
tar -czf "${tmp}/fake.tar.gz" -C "${tmp}/src" "${name}"
sum="$(digest_file "${tmp}/fake.tar.gz")"

if [[ ! -e "${poison}" ]]; then
  mkdir -p "${poison}"
  echo poison >"${poison}/poison"
  created_poison=1
fi

dest="${tmp}/out/kubebuilder"
KUBEBUILDER_GOOS=linux \
  KUBEBUILDER_GOARCH=amd64 \
  KUBEBUILDER_SHA256="${sum}" \
  KUBEBUILDER_URL="file://${tmp}/fake.tar.gz" \
  KUBEBUILDER_DEST="${dest}" \
  "${SCRIPT}"

[[ -x "${dest}/bin/kubebuilder" ]] || fail "install did not produce bin/kubebuilder"
if [[ "${created_poison}" -eq 1 && ! -f "${poison}/poison" ]]; then
  fail "shared /tmp staging path was clobbered"
fi

# A second run must not need the archive.
KUBEBUILDER_GOOS=linux \
  KUBEBUILDER_GOARCH=amd64 \
  KUBEBUILDER_SHA256="${sum}" \
  KUBEBUILDER_URL="file://${tmp}/missing.tar.gz" \
  KUBEBUILDER_DEST="${dest}" \
  "${SCRIPT}"

bad="${tmp}/bad"
set +e
KUBEBUILDER_GOOS=linux \
  KUBEBUILDER_GOARCH=amd64 \
  KUBEBUILDER_SHA256="0000000000000000000000000000000000000000000000000000000000000000" \
  KUBEBUILDER_URL="file://${tmp}/fake.tar.gz" \
  KUBEBUILDER_DEST="${bad}" \
  "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "bad checksum was accepted"
[[ ! -e "${bad}" ]] || fail "bad checksum still installed a tree"

link="${tmp}/linkdest"
ln -s "${tmp}/src" "${link}"
set +e
KUBEBUILDER_GOOS=linux \
  KUBEBUILDER_GOARCH=amd64 \
  KUBEBUILDER_SHA256="${sum}" \
  KUBEBUILDER_URL="file://${tmp}/fake.tar.gz" \
  KUBEBUILDER_DEST="${link}" \
  "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "symlink dest was accepted"
[[ -L "${link}" ]] || fail "symlink dest was replaced"

set +e
KUBEBUILDER_GOOS=windows \
  KUBEBUILDER_GOARCH=amd64 \
  KUBEBUILDER_URL="file://${tmp}/fake.tar.gz" \
  KUBEBUILDER_DEST="${tmp}/unused" \
  env -u KUBEBUILDER_SHA256 "${SCRIPT}"
rc=$?
set -e
[[ "${rc}" -ne 0 ]] || fail "unknown arch skipped checksum"

echo "ok"
