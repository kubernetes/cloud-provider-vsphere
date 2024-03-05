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


dependencies=("k8s.io/api" "k8s.io/client-go" "k8s.io/cloud-provider" "k8s.io/apimachinery" "k8s.io/code-generator" "k8s.io/component-base" "k8s.io/klog/v2")

compare_versions() {
    version1=$1
    version2=$2

    if [[ $(echo -e "$version1\n$version2" | sort -V | tail -n 1) == "$version1" ]]; then
        return 0
    else
        return 1
    fi
}

check_and_bump_dependency() {
  dep=$1
  current_version=$(go list -m -f '{{.Version}}' "${dep}")
  # latest_stable_version=$(go list -m -u -json ${dep} | jq -r .Version)
  latest_version=$(go list -m -versions -json "${dep}" | jq -r '.Versions[-1]')

  echo "Current $dep version: $current_version"
  echo "Latest $dep version: $latest_version"

  compare_versions "$current_version" "$latest_version"
  result=$?

  # Bump the version if needed
  if [ $result -eq 1 ]; then
      echo "Updating $dep to the $latest_version..."
      go get -u k8s.io/api@"${latest_version}"
  else
      echo "$dep is already up to date."
  fi
}

# Loop through the list of dependencies
for dep in "${dependencies[@]}"; do
    check_and_bump_dependency "$dep"
done

go mod tidy