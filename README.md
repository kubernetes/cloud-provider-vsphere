# Kubernetes vSphere Cloud Provider

## vSphere Cloud Controller Manager

This repository contains the [Kubernetes cloud-controller-manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) for vSphere.

This project replaces the deprecated in-tree vSphere cloud provider located within the [Kubernetes repository](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/legacy-cloud-providers/vsphere). If you want to create issues or pull requests for the in-tree cloud provider, please go to the [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20180530-cloud-controller-manager.md).

## Quickstart

Get started with Cloud controller manager for vSphere with Kubeadm with this [quickstart](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html).

## Quickstart using Helm

Get started with Cloud controller manager for vSphere using Helm with this [Helm quickstart](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-helm.html).

## Documentation

Documentation on how to install and use the Kubernetes vSphere Cloud Provider is located on the [docs site](https://cloud-provider-vsphere.sigs.k8s.io/).

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

## Container images

Official releases of the vSphere Cloud Controller Manager container image can be found at:

<https://gcr.io/cloud-provider-vsphere/cpi/release/manager>

The very latest builds from the tip of master, which may not be stable, can be found at:

<https://gcr.io/cloud-provider-vsphere/cpi/ci/manager>

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

### vSphere storage support

Out of tree cloud providers no longer provide native storage support. Instead, a
Container Storage Interface (CSI) driver is required. The vSphere CSI driver is
located [here](https://github.com/kubernetes-sigs/vsphere-csi-driver).
