# Kubernetes vSphere Cloud Provider

![GitHub release (latest SemVer including pre-releases](https://img.shields.io/github/v/release/kubernetes/cloud-provider-vsphere?include_prereleases)
![contributions welcome](https://img.shields.io/badge/contributions-welcome-brightgreen.svg?style=flat)

![image](/docs/images/vsphere_kubernetes_logo.png)

## vSphere Cloud Controller Manager

This repository contains the [Kubernetes cloud-controller-manager](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) for vSphere.

This project replaces the deprecated in-tree vSphere cloud provider located within the [Kubernetes repository](https://github.com/kubernetes/kubernetes/tree/master/staging/src/k8s.io/legacy-cloud-providers/vsphere). If you want to create issues or pull requests for the in-tree cloud provider, please go to the [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20180530-cloud-controller-manager.md).

## Compatibility with Kubernetes

The vSphere cloud provider is released with a specific semantic version `MAJOR.MINOR.PATCH` that correlates with the Kubernetes upstream version. Compatibility with a new Kubernetes version requires upgrading existing cloud provider components since compatibility is ONLY guaranteed between a specific release and its corresponding Kubernetes version.

In the future, the major and minor versions of releases should be equivalent to the compatible upstream Kubernetes release, and the patch version is used for bug fixes pertaining to specific Kubernetes releases. See [the external cloud provider versioning KEP](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cloud-provider/1771-versioning-policy-for-external-cloud-providers) for more details.

Version matrix:

| Kubernetes Version | vSphere Cloud Provider Release Version | Cloud Provider Branch |
|--------------------|----------------------------------------|-----------------------|
| v1.24.X            | v1.24.X                                | release-1.24          |
| v1.23.X            | v1.23.X                                | release-1.23          |
| v1.22.X            | v1.22.X                                | release-1.22          |
| v1.21.X            | v1.21.X                                | release-1.21          |
| v1.20.X            | v1.20.X                                | release-1.20          |
| v1.19.X            | v1.19.X                                | release-1.19          |
| v1.18.X            | v1.18.X                                | release-1.18          |

Our current support policy is that when a new Kubernetes release comes out, we will bump our k8s dependencies to the new version and cut a new release for CPI, e.g. CPI v1.22.x was released after k8s v1.22 comes out.

The latest CPI version is ![GitHub release (latest SemVer including pre-releases](https://img.shields.io/github/v/release/kubernetes/cloud-provider-vsphere?include_prereleases). The recommended way to upgrade CPI can be found on [this page](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/releases/README.md).

## Quickstart

Get started with Cloud controller manager for vSphere with Kubeadm with this [quickstart](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html).

## Quickstart using Helm

Get started with Cloud controller manager for vSphere using Helm with this [Helm quickstart](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/tutorials/kubernetes-on-vsphere-with-helm.md).

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
