#!/bin/bash

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

# This script is used to squash all commits of the branch. 
# Usage: "./hack/git-squash.sh <squash-branch> <base-branch> <commit-message>"
# Note: squash-branch and base-branch are optional, default to current branch and master branch

set -o errexit
set -o nounset
set -o pipefail

if [ -n "$(git status --porcelain)" ]
then
    echo "git status wasn't clean, it's likely that there are changes not staged for commit - FAILED"
    exit 1
fi

red='\033[0;31m'
green='\033[0;32m'
color_off='\033[0m'

function print_usage {
    echo "Usage:"
    echo "  ./hack/git-squash.sh [squash-branch] [base-branch] \"commit-message\""
    echo "  base-branch defaults to 'master'"
} 

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo
    echo -e "${red}Error${color_off} - Wrong number of arguments"
    echo
    print_usage
    echo
    exit 1
fi

current_branch=$(git rev-parse --abbrev-ref HEAD)

if [[ $# -eq 1 ]]; then
    base_branch=master
    message=$1
fi

if [[ $# -eq 2 ]]; then
    base_branch=$1
    message=$2
fi

echo "Current branch: ${green}$current_branch${color_off}"
echo "Base branch:    ${green}$base_branch${color_off}"

git reset "$(git merge-base "$base_branch" "$current_branch")"
git add -A
git commit -m "$message"
