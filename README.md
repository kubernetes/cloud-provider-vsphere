# Cloud provider for vSphere

## Cloud controller manager for vSphere

This repository provides tools and scripts for building and testing `Kubernetes cloud-controller-manager` for vSphere. The project is under development and should not be used in production.

The in-tree vSphere cloud provider code is located within the [Kubernetes repository](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/legacy-cloud-providers/vsphere). If you want to create issues or pull requests for the in-tree cloud provider, please go to the [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20180530-cloud-controller-manager.md).

## Quickstart

Get started with Cloud controller manager for vSphere with Kubeadm with this [quickstart](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html).

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

### vSphere storage support

Out of tree cloud providers no longer provide native storage support. Instead, a
Container Storage Interface (CSI) driver is required. The vSphere CSI driver is
located [here](https://github.com/kubernetes-sigs/vsphere-csi-driver).
