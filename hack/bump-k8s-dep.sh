#!/bin/bash

# Copyright 2019 The Kubernetes Authors.
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

# This script is used build new container images of the CAPV manager and
# clusterctl. When invoked without arguments, the default behavior is to build
# new ci images

set -o errexit
set -o nounset
set -o pipefail

# Check if the first input does not exist or is not equal to "test"
dependencies=("k8s.io/api" "k8s.io/client-go" "k8s.io/apimachinery" "k8s.io/klog/v2")
if [ -z "${1:-}" ] || [ "${1}" != "test" ]; then
  dependencies+=("k8s.io/cloud-provider" "k8s.io/code-generator" "k8s.io/component-base")
fi

# Define color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RESET='\033[0m'  # Reset color to default

check_and_bump_dependency() {
  dep=$1
  current_version=$(go list -m -f '{{.Version}}' "${dep}")
  latest_version=$(go list -m -versions -json "${dep}" | jq -r '.Versions[-1]')
  # latest_stable_version=$(go list -m -u -json ${dep} | jq -r .Version)

  # filter out the alpha release skip for test
  # if [[ $latest_version =~ alpha\.([0-9]+)$ ]]; then
  #   echo -e "${BLUE} Skip auto bump for alpha release: [$dep@$latest_version]${RESET}"
  #   return
  # fi

  # Bump the version if needed
  if [ "$current_version" == "$latest_version" ]; then
    echo -e "${BLUE} $dep@$current_version is already up to date.${RESET}"
  else
    echo -e "${GREEN} Updating $dep to $latest_version ...${RESET}"
    go get -u "${dep}"@"${latest_version}"
  fi
}

# Loop through the list of dependencies
for dep in "${dependencies[@]}"; do
  check_and_bump_dependency "$dep"
done

go mod tidy
