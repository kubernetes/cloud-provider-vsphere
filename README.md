# Cloud provider for vSphere

## Cloud controller manager for vSphere

This repository provides tools and scripts for building and testing `Kubernetes cloud-controller-manager` for vSphere. The project is under development and should not be used in production.

The vSphere cloud provider code locates at [Kubernetes repository directory](https://github.com/kubernetes/kubernetes/tree/master/pkg/cloudprovider/providers/vsphere). If you want to create issues or pull requests for cloud provider, please go to [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is an ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/community/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md).

## Building the cloud provider

This section outlines how to build the cloud provider with and without Docker.

### Building locally

Clone this repository to `$GOPATH/src/k8s.io/cloud-provider-vsphere`. Please note that this path is not the same as the project's location in GitHub. Failing to clone the repository to the prescribed path causes the Go dependency tool `dep` and builds to fail.

```shell
$ make
```

### Building with Docker

It is also possible to build the cloud provider with Docker in order to ensure a clean build environment. When building with Docker this repository may be cloned anywhere in or out of the `$GOPATH`. For example, the following script clones and builds the cloud-provider using a temporary directory:

**Note**: Python is used to resolve the temporary directory's real path. This step is required on macOS due to Docker's restrictions on which directories can be shared with a container. The script has been tested on Linux as well.

```shell
$ cd $(python -c "import os; print(os.path.realpath('$(mktemp -d)'))") && \
  git clone https://github.com/kubernetes/cloud-provider-vsphere . && \
  hack/make.sh
```

### The dep tool hangs

The `dep` tool may freeze when running locally and not via the `hack/make.sh` command that uses the Docker image. If this happens, please check the following:

1. The repository must be cloned to `$GOPATH/src/k8s.io/cloud-provider-vsphere`. This is not the same path as the project's location in GitHub, but rather reflects the project's Go packages' vanity import path.
2. The Mercurial client `hg` must be installed in order to fetch the dependency `bitbucket.org/ww/goautoneg`. Otherwise `dep` will [hang](https://github.com/kubernetes/test-infra/blob/master/docs/dep.md#tips) indefinitely without any indication as to the reason.

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

### NOTE
Currently this repository is used for building and testing cloud-controller-manager for vSphere, it references vSphere cloud provider implementation code as vendor dir. After handoff, the vSphere cloud provider implementation will be moved here.
