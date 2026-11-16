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

# Install a pinned Helm 3 release. The archive is staged privately and
# moved into place only after its sha256 matches.
set -o errexit
set -o nounset
set -o pipefail

HELM_VERSION="${HELM_VERSION:-v3.22.1}"

if [[ -n "${HELM_OS:-}" ]]; then
  GO_OS="${HELM_OS}"
else
  GO_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
fi
if [[ -n "${HELM_ARCH:-}" ]]; then
  GO_ARCH="${HELM_ARCH}"
else
  case "$(uname -m)" in
    x86_64|amd64) GO_ARCH=amd64 ;;
    aarch64|arm64) GO_ARCH=arm64 ;;
    ppc64le) GO_ARCH=ppc64le ;;
    *)
      echo "unsupported arch $(uname -m)" >&2
      exit 1
      ;;
  esac
fi

sha_for() {
  if [[ -n "${HELM_SHA256:-}" ]]; then
    printf '%s\n' "${HELM_SHA256}"
    return 0
  fi
  case "${HELM_VERSION}_${GO_OS}_${GO_ARCH}" in
    v3.22.1_linux_amd64) printf '%s\n' "896cf63684a42a723d61c980369d6e588330927deb13676ca0d85006dd6da0ea" ;;
    v3.22.1_linux_arm64) printf '%s\n' "4c29e286b279d170fa048ddfc3e4e83dcd069ca9c52102766ba672d7149e694d" ;;
    v3.22.1_linux_ppc64le) printf '%s\n' "48a9711b69f0b63504f710ba18766dd5f5424fe380e67304aa3c89e1c0b97b5c" ;;
    v3.22.1_darwin_amd64) printf '%s\n' "cf850a81b908f8eef0331e6b18776da2767bcac639ca84472311dad20008495c" ;;
    v3.22.1_darwin_arm64) printf '%s\n' "e73da8a0b0d5193774fdff17ce6319c7436bb22ee229b45fd64f3c40368c753a" ;;
    *)
      echo "no helm ${HELM_VERSION} checksum for ${GO_OS}/${GO_ARCH}" >&2
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
plat="${GO_OS}-${GO_ARCH}"
URL="${HELM_URL:-https://get.helm.sh/helm-${HELM_VERSION}-${plat}.tar.gz}"
DEST="${HELM_DEST:-/usr/local/bin/helm}"

if [[ -L "${DEST}" ]]; then
  echo "refusing to replace symlink at ${DEST}" >&2
  exit 1
fi

parent="$(dirname "${DEST}")"
mkdir -p "${parent}"

staging="$(mktemp -d)"
cleanup() {
  if [[ -n "${staging}" ]]; then
    rm -rf "${staging}"
  fi
}
trap cleanup EXIT

archive="${staging}/helm.tar.gz"
curl -fsSL -o "${archive}" "${URL}"

got="$(digest_file "${archive}")"
if [[ "${got}" != "${EXPECTED}" ]]; then
  echo "checksum mismatch for helm ${HELM_VERSION} ${plat}: got ${got}" >&2
  exit 1
fi

tar -xzf "${archive}" -C "${staging}"
bin="${staging}/${plat}/helm"
if [[ ! -f "${bin}" || -L "${bin}" ]]; then
  echo "archive did not contain a regular ${plat}/helm binary" >&2
  exit 1
fi
chmod 0755 "${bin}"

if [[ -w "${parent}" ]]; then
  mv "${bin}" "${DEST}"
else
  sudo mv "${bin}" "${DEST}"
  sudo chmod 0755 "${DEST}"
fi

if [[ -n "${HELM_SKIP_REPO:-}" ]]; then
  exit 0
fi

# ========== install cloud-provider-vsphere chart ==========
helm repo add vsphere-cpi https://kubernetes.github.io/cloud-provider-vsphere
helm repo update
