# Release Guide for CPI

When a new k8s version is available, we should bump our [k8s dependencies of CPI](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/go.mod) before cutting a new CPI release.

In this tutorial, we provide detailed steps on how to cut an official release of CPI.

## Create a PR to bump k8s dependencies

We recommend upgrading and downgrading of CPI dependencies using `go get`, which will automatically update the `go.mod` file.

For example, to upgrade a dependency to the latest version:

```shell
go get k8s.io/cloud-provider/app@v0.22.1
```

Remember to update `version` value in the [Dockerfile for image building](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/cluster/images/controller-manager/Dockerfile#L36).

Sample PR: [Bump k8s dependencies to 1.22 and go to 1.16](https://github.com/kubernetes/cloud-provider-vsphere/pull/496)

## Test before release

Before we release a new version, we should always make sure we've fully tested CPI. To build a docker image for testing, you can run:

```shell
make docker-image IMAGE=<image_name>
```

## Create a sample release YAML

For each release, we should provide its release YAML under [this folder](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/releases). Please refer to [this PR](https://github.com/kubernetes/cloud-provider-vsphere/pull/487) to add the corresponding release YAML.

## Create a GitHub Release

Normally, we need to cut alpha and beta releases before the official release. For example, before cutting `1.22.0` official release, we need to first create an alpha release named `v1.22.0-alpha.1`, and ensure that this alpha version of CPI is working. If a new bug occurs, we should fix that and cut another release named `v1.22.0-alpha.2`. Once the latest alpha release is stable, we are ready to cut a beta release named `v1.22.0-beta.1` and follow the same pattern as alpha releases. When the beta release is stable, we are ready to cut the official release `1.22.0`.

To create a new release, please refer to the following workflow:

```shell
# if is to cut a minor release, do below on master branch first
# if it's for a patch release, do below on release branch
$ git pull --rebase
# release_name can be v1.22.0-alpha.1, v1.22.0-beta.1, v1.22.0, etc
$ git tag -a <release_name>
# push to master branch first before checking out a new release branch
$ git push <remote_name> <branch_name> --tags
# skip below if it's for cutting patch release
# check out a minor branch if there isn't any for that minor version
$ git checkout -b <release_name>
# push
$ git push <remote_name> <release_name>
```

Now we can open up the [release page](https://github.com/kubernetes/cloud-provider-vsphere/releases), and click on `Draft a new release`. Use the tag we just created and edit the release message by refering to major PRs for important user-facing features instead of minor bug fixes.

Press `Publish Release` to publish the release from the existing tag. As soon as you publish the release on GitHub, we can see it under the release tab, which was previously showing just the tag names.

Please go to [post-release-pipeline](https://prow.k8s.io/view/gs/kubernetes-jenkins/logs/post-cloud-provider-vsphere-release/) to check the release logs and make sure new image is published in `gcr.io/cloud-provider-vsphere/cpi/release/manager` with the correct version tag.
