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

set -o errexit
set -o nounset
set -o pipefail

KIND_VERSION="v0.5.1"
KIND_PATH="/usr/local/bin/kind"

# Check if KIND already exists
if ! [[ -x "${KIND_PATH}" ]]; then
  wget "https://github.com/kubernetes-sigs/kind/releases/download/${KIND_VERSION}/kind-linux-amd64" \
    --no-verbose -O "${KIND_PATH}"
  chmod +x "${KIND_PATH}"
fi
