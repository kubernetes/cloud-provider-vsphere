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

REPO_ROOT=$(realpath $(dirname $(dirname "${BASH_SOURCE[0]}")))

# ========== check the input release version ==========
release_version=${1:-}

if [ -z "${release_version}" ]; then
    read -p "Please enter the release version: " release_version
else
    release_version=$1
fi

if [ -z "${release_version}" ]; then
    echo "Error: No release version provided."
    exit 1
fi

if ! $REPO_ROOT/hack/match-release-tag.sh ${release_version} >/dev/null 2>&1; then
    echo "Error: Release version is not in semver format."
    exit 1
fi

echo "Release version provided: ${release_version}"

# ========== install yq ==========
YQ_VER=v4.44.1
YQ_BINARY=yq_linux_amd64

install_yq() {
    wget https://github.com/mikefarah/yq/releases/download/${YQ_VER}/${YQ_BINARY} -O /usr/bin/yq &&\
        chmod +x /usr/bin/yq
}

if ! command -v yq &> /dev/null; then
    echo "yq is not installed."
    install_yq
else
    echo "yq is installed."
fi

# ========== update yaml file ==========
yq -i ".appVersion = \"${release_version:1}\"" $REPO_ROOT/charts/vsphere-cpi/Chart.yaml
yq -i ".version = \"${release_version:1}\"" $REPO_ROOT/charts/vsphere-cpi/Chart.yaml

yq -i ".daemonset.tag = \"${release_version}\"" $REPO_ROOT/charts/vsphere-cpi/values.yaml

yq -i "(. | select(.kind == \"DaemonSet\")).spec.template.spec.containers[0].image = \"registry.k8s.io/cloud-pv-vsphere/cloud-provider-vsphere:${release_version}\"" \
 $REPO_ROOT/docs/book/tutorials/disable-node-deletion.yaml

yq -i "(. | select(.kind == \"DaemonSet\")).spec.template.spec.containers[0].image = \"registry.k8s.io/cloud-pv-vsphere/cloud-provider-vsphere:${release_version}\"" \
 $REPO_ROOT/manifests/controller-manager/vsphere-cloud-controller-manager-ds.yaml

yq -i "(. | select(.kind == \"Pod\")).spec.containers[0].image = \"registry.k8s.io/cloud-pv-vsphere/cloud-provider-vsphere:${release_version}\"" \
 $REPO_ROOT/manifests/controller-manager/vsphere-cloud-controller-manager-pod.yaml

# ========== update README ==========

major_minor_version=$(echo "${release_version}" | cut -d '.' -f 1,2)

sed -i "s/latest version of cloud provider vsphere(\(.*\))/latest version of cloud provider vsphere(${release_version})/g" $REPO_ROOT/releases/README.md
sed -i "s/the major version of '[0-9]\+\.[0-9]\+.x' is '[0-9]\+\.[0-9]\+'/the major version of '${major_minor_version:1}.x' is '${major_minor_version:1}'/g" $REPO_ROOT/releases/README.md
sed -i "s/VERSION=[0-9]\+\.[0-9]\+/VERSION=${major_minor_version:1}/g" $REPO_ROOT/releases/README.md

TABLE_MARKER="<!-- RELEASE_TABLE -->"

update_readme_table() {
    new_release_row="| ${major_minor_version}.X            | ${major_minor_version}.X                                | release-${major_minor_version:1}          |"
    awk -v table="$TABLE_MARKER" -v row="$new_release_row" '
        BEGIN { found = 0; header_row_found = 0 }
        {
            if ($0 ~ table) {
                found = 1
            }
            if (found && header_row_found && $0 ~ /^\|/) {
                print row
                found = 0
            }
            if (found && $0 ~ /^\|-/) {
                header_row_found = 1
            }
            print
        }
    ' "$REPO_ROOT/README.md" > tmpfile && mv tmpfile "$REPO_ROOT/README.md"
}

if ! grep -q "${major_minor_version}.X" "$REPO_ROOT/README.md"; then
    echo "updating README for release branch release-${major_minor_version:1}"
    update_readme_table
fi

# TODO: need to change latest version in the image path

# ========== update release yaml ==========
SEMVER_REGEX='^[[:space:]]{0,}v[[:digit:]]{1,}\.[[:digit:]]{1,}\.[[:digit:]]{1,}(-(alpha|beta|rc)\.[[:digit:]]{1,}){0,1}[[:space:]]{0,}$'
git fetch --tags

if [ ! -d "$REPO_ROOT/releases/${major_minor_version}/vsphere-cloud-controller-manager.yaml" ]; then  
    mkdir -p $REPO_ROOT/releases/${major_minor_version}
    latest_release=$(git tag --sort=-v:refname | grep -E ${SEMVER_REGEX} | sed -n '1p' | cut -d '.' -f 1,2)
    cp  $REPO_ROOT/releases/${latest_release}/vsphere-cloud-controller-manager.yaml $REPO_ROOT/releases/${major_minor_version}/vsphere-cloud-controller-manager.yaml
fi

yq -i "(. | select(.kind == \"DaemonSet\")).spec.template.spec.containers[0].image = \"registry.k8s.io/cloud-pv-vsphere/cloud-provider-vsphere:${release_version}\"" \
 $REPO_ROOT/releases/${major_minor_version}/vsphere-cloud-controller-manager.yaml

# ========== update Dockerfile ==========
sed -i "s/ARG VERSION=.*/ARG VERSION=${release_version:1}/g" $REPO_ROOT/cluster/images/controller-manager/Dockerfile

# ========== run helm command to update package and index ==========
cd $REPO_ROOT/charts

if ! command -v helm &> /dev/null; then
    echo "helm is not installed."
    $REPO_ROOT/hack/install-helm.sh
else
    echo "helm is installed."
fi

helm package vsphere-cpi --version $release_version --app-version $release_version

# ========== submit PR ==========
if ! command -v gh &> /dev/null; then
    echo "gh is not installed."
    $REPO_ROOT/hack/install-gh.sh
else
    echo "gh is installed."
fi

git checkout -b pre-${release_version}-document-update

git add .

git commit -m "update docuemnt for ${release_version} release"

git push -u origin pre-${release_version}-document-update

gh repo set-default kubernetes/cloud-provider-vsphere

gh pr create --title ":book: pre ${release_version} release document update" --body "This is an auto generated PR to update cloud-provider-vsphere repo document to ${release_version}"
