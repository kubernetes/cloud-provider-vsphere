#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

# SCRIPT_ROOT: the directory in which this script is located
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
# CUSTOM_RESOURCE_PACKAGE: the name of the custom resource package that we are generating client code for
CUSTOM_RESOURCE_PACKAGE="nsxnetworking"
# CUSTOM_RESOURCE_VERSION: the version of the resource
CUSTOM_RESOURCE_VERSION="v1alpha1"

DIFFROOT="${SCRIPT_ROOT}/pkg"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/pkg"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"

"${SCRIPT_ROOT}/hack/update-codegen.sh"
printf "\xE2\x8F\xB3"
echo "diffing ${DIFFROOT} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]
then
  printf "\xE2\x9C\x85"
  echo "$CUSTOM_RESOURCE_PACKAGE:$CUSTOM_RESOURCE_VERSION up to date."
else
  printf "\xE2\x9D\x8C"
  echo "${DIFFROOT} is out of date. Please run hack/update-codegen.sh"
  exit 1
fi
