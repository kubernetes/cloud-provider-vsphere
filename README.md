# Cloud provider for vSphere

## Cloud controller manager for vSphere

This repository provides tools and scripts for building and testing `Kubernetes cloud-controller-manager` for vSphere. The project is under development and should not be used in production.

The vSphere cloud provider code locates at [Kubernetes repository directory](https://github.com/kubernetes/kubernetes/tree/master/pkg/cloudprovider/providers/vsphere). If you want to create issues or pull requests for cloud provider, please go to [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is an ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md).

## Building the cloud provider

This section outlines how to build the cloud provider with and without Docker.

### Building locally

Build locally with the following command:

```shell
$ git clone https://github.com/kubernetes/cloud-provider-vsphere && \
  make -C cloud-provider-vsphere
```

The project uses [Go modules](https://github.com/golang/go/wiki/Modules) and:
* Requires Go 1.11+
* Should not be cloned into the `$GOPATH`

### Building with Docker

It is also possible to build the cloud provider with Docker in order to ensure a clean build environment:

```shell
$ git clone https://github.com/kubernetes/cloud-provider-vsphere && \
  make -C cloud-provider-vsphere build-with-docker
```

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

### NOTE
Currently this repository is used for building and testing cloud-controller-manager for vSphere, it references vSphere cloud provider implementation code as vendor dir. After handoff, the vSphere cloud provider implementation will be moved here.
