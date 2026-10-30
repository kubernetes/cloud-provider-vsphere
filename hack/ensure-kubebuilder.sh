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

# Install the kubebuilder bundle used by `make unit` into a private
# staging directory, and only move it into place after the sha256 matches.
set -o errexit
set -o nounset
set -o pipefail

VERSION="${1:-${KUBE_BUILDER_VERSION:-2.3.1}}"
if [[ -n "${KUBEBUILDER_GOOS:-}" ]]; then
  GO_OS="${KUBEBUILDER_GOOS}"
else
  GO_OS="$(go env GOOS)"
fi
if [[ -n "${KUBEBUILDER_GOARCH:-}" ]]; then
  GO_ARCH="${KUBEBUILDER_GOARCH}"
else
  GO_ARCH="$(go env GOARCH)"
fi

DEST="${KUBEBUILDER_DEST:-/usr/local/kubebuilder}"
NAME="kubebuilder_${VERSION}_${GO_OS}_${GO_ARCH}"

if [[ -x "${DEST}/bin/kubebuilder" && ! -L "${DEST}" ]]; then
  exit 0
fi

sha_for() {
  if [[ -n "${KUBEBUILDER_SHA256:-}" ]]; then
    printf '%s\n' "${KUBEBUILDER_SHA256}"
    return 0
  fi
  case "${GO_OS}_${GO_ARCH}" in
    linux_amd64) printf '%s\n' "ff496970f209706763f2aba2bdcefc2de8d00085b3b972b5790117b59ea4ed10" ;;
    linux_arm64) printf '%s\n' "153ff1bc656820917fe5b3eb393bf7b607e04c299f8602817f8088040ecffb40" ;;
    linux_ppc64le) printf '%s\n' "25c684402841cbb1d87daf05d2c5d76a16e9f8d3c2ad949a439dd947b5e52150" ;;
    darwin_amd64) printf '%s\n' "39314d45053b6c31eb17e35c9b8d923f8a38277a9a136448345dd4b7f0f308f4" ;;
    *)
      echo "no kubebuilder ${VERSION} checksum for ${GO_OS}/${GO_ARCH}" >&2
      return 1
      ;;
  esac
}

digest_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

EXPECTED="$(sha_for)"
URL="${KUBEBUILDER_URL:-https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${VERSION}/${NAME}.tar.gz}"

if [[ -L "${DEST}" ]]; then
  echo "refusing to replace symlink at ${DEST}" >&2
  exit 1
fi

parent="$(dirname "${DEST}")"
mkdir -p "${parent}"
lockdir="${DEST}.lock"
if ! mkdir "${lockdir}" 2>/dev/null; then
  echo "kubebuilder install already in progress (${lockdir})" >&2
  exit 1
fi

staging=""
cleanup() {
  if [[ -n "${staging}" ]]; then
    rm -rf "${staging}"
  fi
  if [[ -n "${lockdir}" ]]; then
    rmdir "${lockdir}" 2>/dev/null || true
  fi
}
trap cleanup EXIT

staging="$(mktemp -d)"
archive="${staging}/${NAME}.tar.gz"
curl -fsSL -o "${archive}" "${URL}"

got="$(digest_file "${archive}")"
if [[ "${got}" != "${EXPECTED}" ]]; then
  echo "checksum mismatch for ${NAME}: got ${got}" >&2
  exit 1
fi

tar -xzf "${archive}" -C "${staging}"
if [[ ! -d "${staging}/${NAME}" ]]; then
  echo "archive did not contain ${NAME}" >&2
  exit 1
fi
if [[ -L "${staging}/${NAME}" ]]; then
  echo "archive root ${NAME} is a symlink" >&2
  exit 1
fi

rm -rf "${DEST}"
mv "${staging}/${NAME}" "${DEST}"
