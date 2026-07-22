# Release Guide for CPI

This document provides a comprehensive, step-by-step guide on how to prepare, build, test, and publish a new release of the vSphere Cloud Provider Interface (CPI). It covers the requirements for Beta, RC, and Official (minor & patch) releases.

---

## Phase 1: Bump Kubernetes and Cloud Provider Dependencies (All Releases: Beta, RC, Official)

When a new Kubernetes version is released, you must bump the Kubernetes dependencies of CPI before cutting a new release.

### 1. Automatic Dependency Bumping via GitHub Workflows

You can trigger dependency bumps using the following GitHub Actions workflows:
- [Bump Kubernetes Dependencies Workflow](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/.github/workflows/bump-k8s-dep.yml)
- [Bump Test/E2E Kubernetes Dependencies Workflow](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/.github/workflows/bump-test-k8s-dep.yml)

### 2. Manual Dependency Bumping

Alternatively, you can manually bump dependencies using `go get` in the root and E2E test directories. Using `go get` ensures that `go.mod` and `go.sum` are updated cleanly.

* **Sample Dependency Bump PR**: Refer to [PR #1820](https://github.com/kubernetes/cloud-provider-vsphere/pull/1820) for an example of bumping Kubernetes version dependencies.

For example, to upgrade a dependency to a target version:

```shell
go get k8s.io/cloud-provider/app@v0.37.0-beta.0
```

After updating dependencies, ensure the packages are tidy:

```shell
go mod tidy
```

Remember to also update the version value in the [Dockerfile for image building](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/cluster/images/controller-manager/Dockerfile).

> **Note on Beta/RC Releases**: The `ARG VERSION` value in the `Dockerfile` is **only** updated for GA (official minor and patch) releases (typically managed by the `./hack/update-docs.sh` script). For Beta and RC releases, you do not need to update `ARG VERSION` in the `Dockerfile` manually. This is because our build processes (including the `Makefile` and Prow workflows) automatically override the version dynamically by passing the `--build-arg "VERSION=${VERSION}"` flag during image compilation.

---

## Phase 2: Testing and CI Maintenance (All Releases: Beta, RC, Official)

Before cutting any release, you must verify that CPI passes E2E and unit tests.

### 1. Build and Test Locally

To compile and build a local Docker image for testing, run:

```shell
make docker-image IMAGE=<image_name>
```

### 2. CI and Testbed Maintenance

The CPI project runs its E2E suite via Prow jobs. Maintain the CI jobs and test configurations as follows:

* **CI Jobs Config**: Maintained under the Kubernetes `test-infra` repository:
  [Kubernetes test-infra CPI jobs](https://github.com/kubernetes/test-infra/tree/master/config/jobs/kubernetes/cloud-provider-vsphere)
* **E2E Testbed Configurations**: Maintained inside [vsphere-ci.yaml](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/test/e2e/config/vsphere-ci.yaml).
* **Keep CAPI and CAPV Releases Up-to-Date**: You should ensure that the Cluster API (CAPI) and Cluster API Provider vSphere (CAPV) release versions are kept up-to-date in E2E tests. These are used during the E2E test runs to verify compatibility.
* **Update Kubernetes and OVA Versions in CI**: We should also update the Kubernetes version used in CI accordingly, along with the corresponding OVA version. The list of compatible, published OVA templates can be found in the [CAPV Kubernetes Versions with Published OVAs](https://github.com/kubernetes-sigs/cluster-api-provider-vsphere#kubernetes-versions-with-published-ovas) documentation.
* **Sample CAPI/CAPV Version Update and E2E configuration Update**: Refer to [PR #1821](https://github.com/kubernetes/cloud-provider-vsphere/pull/1821) for an example of updating the CAPI & CAPV release dependencies for E2E testing.

---

## Phase 3: Update Release Documents (Official Release Only)

If you are publishing an **Official (Minor or Patch) Release**, you must update the documentation manifests in the repository. Skip this step for Beta or RC releases.

### 1. Run the Doc Update Script

Run the automated document update script by passing the target release version:

```shell
./hack/update-docs.sh <version>
```

*Example:* `./hack/update-docs.sh v1.37.0`

This script automates several tasks:
- Modifies YAML files (DaemonSet, Pod templates, disable-node-deletion config).
- Updates the release information and versions in the root `README.md` and `releases/README.md`.
- Packages the Helm chart, generates the updated repository index (`index.yaml`), and copies the release template to `releases/v1.XX/vsphere-cloud-controller-manager.yaml`.
- Automatically checks out a new branch called `pre-<version>-document-update` and opens a draft PR.

### 2. Document Update Troubleshooting

The automated documentation update script may occasionally fail on a release branch due to environment or tool discrepancies. If generation fails:
1. Compare your branch's files and diff with a successful document update PR. Refer to [PR #1093](https://github.com/kubernetes/cloud-provider-vsphere/pull/1093) or [PR #1287](https://github.com/kubernetes/cloud-provider-vsphere/pull/1287).
2. Manually fix any misalignment and commit the doc updates to the branch.

---

## Phase 4: Git Tagging and Creating GitHub Releases (All Releases: Beta, RC, Official)

Once dependencies, code, and documentation (if applicable) are ready, proceed with tagging the release.

### 1. Tag the Release Branch

Run the following commands to pull the latest changes, apply the tag, and push them to the upstream repository:

```shell
# Checkout target release branch (e.g. master or release-1.37)
git checkout <release-branch>

# Pull latest changes from upstream securely with rebase
git pull upstream <release-branch> -r

# Tag the release. Tag name format: v<Major>.<Minor>.<Patch>[-beta.X, -rc.X]
git tag -a <version> -m "Release <version>"

# Push the tags to the upstream repository
git push upstream <release-branch> --tags
```

### 2. Local Tag Alignment Troubleshooting

If there is a tag misalignment issue or a non-existent/incorrect tag on your local machine, delete the local tag and pull fresh tags:

```shell
# Delete incorrect local tag
git tag -d <incorrect-tag>

# Fetch clean tags from upstream
git fetch upstream --tags
```

---

## Phase 5: GitHub Release Notes (All Releases: Beta, RC, Official)

### 1. Release Notes Generation

As soon as you push the tag, a release note will be generated automatically by the [generate-release-notes.yml](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/.github/workflows/generate-release-notes.yml) GitHub workflow. 
Navigate to the [GitHub Releases page](https://github.com/kubernetes/cloud-provider-vsphere/releases) to view, edit, and publish the drafted release.

> **Note on Manual Release Fallback**: Release notes are typically generated automatically by the GitHub Actions workflow. However, if the workflow fails or experiences delays, you can manually create a new release. Navigate to the GitHub Releases page, click on **Draft a new release**, select the tag you pushed, and manually compile and publish the release notes.

---

## Phase 6: Image Management and Promotion (All Releases: Beta, RC, Official)

Images are managed via Kubernetes community-controlled registries. The process involves pushing images to a staging repository first, then promoting them to the official registry.

### 1. Image Registries

- **Official Registry**: `registry.k8s.io/cloud-pv-vsphere/cloud-provider-vsphere`

### 2. Pushing to Staging

As soon as a release is published on GitHub, the staging image is automatically built and pushed to the staging registry. The staging push configuration is defined in the [cloudbuild.yaml](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/cloudbuild.yaml) file.
- Check the Prow [Post-Release Push Images Pipeline](https://prow.k8s.io/view/gs/kubernetes-jenkins/logs/post-cloud-provider-vsphere-push-images/) to verify that the staging image built successfully and is tagged with the correct version.

### 3. Promoting to the Official Registry (`registry.k8s.io`)

All releases (Beta, RC, and Official) **must** be promoted to `registry.k8s.io` before they can be consumed by users.

#### Option 1: Promote Using Makefile (Recommended)

Our repository provides a convenient `make promote-images` command that automatically runs `kpromo` to create a promotion pull request.

**Prerequisites & Environment Variables**:
- **`GITHUB_TOKEN`**: You must expose a GitHub personal access token with repository write/pull-request permissions in your environment so `kpromo` can push your fork and create the PR on `kubernetes/k8s.io`.
  ```shell
  export GITHUB_TOKEN="<your_github_token>"
  ```
- **`USER_FORK`**: Specify your personal GitHub username/ID which holds your fork of the `kubernetes/k8s.io` repository.
- **`RELEASE_TAG`** *(Optional)*: By default, the `Makefile` reads the latest tag on your branch. You can override this to specify the version you want to promote (e.g. `RELEASE_TAG=v1.37.0-beta.0`).

**Steps**:
1. Fork the [kubernetes/k8s.io](https://github.com/kubernetes/k8s.io) repository to your personal GitHub account.
2. Run the promote command in our repository:
   ```shell
   make promote-images USER_FORK=<your_github_username> RELEASE_TAG=<version>
   ```
   *Example:*
   ```shell
   make promote-images USER_FORK=mygithubid RELEASE_TAG=v1.37.0-beta.0
   ```
   This will download `kpromo`, use it to make changes in your fork, and submit a PR to `kubernetes/k8s.io` targeting the `cloud-pv-vsphere` project.

#### Option 2: Promote Manually via `kpromo` CLI

If you prefer to run `kpromo` manually or the Makefile helper is unavailable, follow these steps:

1. **Install and Configure `kpromo`**:
   Install the image promotion tool [kpromo](https://github.com/kubernetes-sigs/promo-tools/blob/main/docs/promotion-pull-requests.md).
2. **Create a Promotion Pull Request**:
   Create a pull request in the [kubernetes/k8s.io](https://github.com/kubernetes/k8s.io) repository to define the image promotion.
   - *Sample Promotion PRs*: Refer to [k8s.io PR #6888](https://github.com/kubernetes/k8s.io/pull/6888) and [k8s.io PR #7494](https://github.com/kubernetes/k8s.io/pull/7494).

#### Verify the Promotion

Once the promotion PR (from Option 1 or Option 2) is merged, verify that the image is available in the official registry using the [Container Registry Explorer](https://explore.ggcr.dev/?repo=registry.k8s.io%2Fcloud-pv-vsphere%2Fcloud-provider-vsphere).

---

## Phase 7: Update `gh-pages` Branch (Official Release Only)

For **Official Releases**, you must publish the new Helm charts to the `gh-pages` branch so users can consume the updated chart.
- Create a PR merging the updated Helm files (such as `index.yaml` and the packaged `.tgz` charts) into the `gh-pages` branch.
- *Sample PR:* Refer to [PR #982](https://github.com/kubernetes/cloud-provider-vsphere/pull/982) for guidance.

### Verification via Helm CLI

Once your PR merging the updated chart and `index.yaml` into `gh-pages` has been merged and the GitHub pages build is complete, you should verify that the new chart version is successfully hosted and indexed. Use the Helm CLI as follows:

```shell
# 1. Add the vSphere CPI Helm repository if you haven't already
helm repo add vsphere-cpi https://kubernetes.github.io/cloud-provider-vsphere

# 2. Update your local Helm chart repositories cache
helm repo update

# 3. Search all available versions to verify the new release is listed
helm search repo vsphere-cpi/vsphere-cpi --versions
```

---

## Phase 8: Update Dependabot Configuration (Official Minor Releases Only)

Dependabot is configured to automatically bump dependencies on the `master` branch and the three latest active release branches.

After cutting a new minor release, update the list of tracked release branches within `.github/dependabot.yml` to:
1. Add the newest release branch.
2. Drop the oldest release branch.
