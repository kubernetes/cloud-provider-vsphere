# Cloud provider for vSphere

## Cloud controller manager for vSphere

This repository provides tools and scripts for building and testing `Kubernetes cloud-controller-manager` for vSphere. The project is under development and should not be used in production.

The vSphere cloud provider code locates at [Kubernetes repository directory](https://github.com/kubernetes/kubernetes/tree/master/pkg/cloudprovider/providers/vsphere). If you want to create issues or pull requests for cloud provider, please go to [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is an ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/community/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md).

## Building Locally

Clone this repository to `$GOPATH/src/k8s.io/cloud-provider-vsphere`. Please note that this path is not the same as the project's location in GitHub. Failing to clone the repository to the prescribed path causes the Go dependency tool `dep` and builds to fail.

Once the project is cloned locally, use the `Makefile` to build the cloud provider:

```shell
$ make
```

### The dep tool hangs
The `dep` tool may freeze when running it locally and not via the `hack/make.sh` command that uses the Docker image. If this happens, please check the following:

1. The repository must be cloned to `$GOPATH/src/k8s.io/cloud-provider-vsphere`. This is not the same path as the project's location in GitHub, but rather reflects the project's Go packages' vanity import path.
2. The Mercurial client `hg` must be installed in order to fetch the dependency `bitbucket.org/ww/goautoneg`. Otherwise `dep` freezes for no apparent reason.

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

### NOTE
Currently this repository is used for building and testing cloud-controller-manager for vSphere, it references vSphere cloud provider implementation code as vendor dir. After handoff, the vSphere cloud provider implementation will be moved here.
