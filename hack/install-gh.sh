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

GH_VER=2.50.0
# Set the installation command based on the operating system  
case "$(uname -s)" in
    Linux*)
        curl -fsSL https://github.com/cli/cli/releases/download/v${GH_VER}/gh_${GH_VER}_linux_amd64.tar.gz -o gh.tar.gz
        tar -xzf gh.tar.gz
        sudo mv ./gh_${GH_VER}_linux_amd64/bin/gh /usr/local/bin/gh
        rm -rf ./gh*
        echo 'gh has been successfully installed to /usr/local/bin/gh'
        ;;
    Darwin*)
        # For macOS, use Homebrew or download the binary directly  
        if command -v brew >/dev/null 2>&1; then
            brew install gh
        else
            echo "macOS users are recommended to install gh using Homebrew. If Homebrew is not installed, please install it first and try again."
            exit 1
        fi
        ;;
    *)
        echo "Unsupported operating system."
        exit 1
        ;;
esac

# Check if gh is successfully installed  
if command -v gh >/dev/null 2>&1; then
    echo "gh has been successfully installed."
else
    echo "Failed to install gh. Please check the output for more information."
    exit 1
fi
