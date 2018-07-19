# Cloud provider for vSphere

## Cloud controller manager for vSphere

This repository provides tools and scripts for building and testing `Kubernetes cloud-controller-manager` for vSphere. The project is under development and should not be used in production.

The vSphere cloud provider code locates at [Kubernetes repository directory](https://github.com/kubernetes/kubernetes/tree/master/pkg/cloudprovider/providers/vsphere). If you want to create issues or pull requests for cloud provider, please go to [Kubernetes repository](https://github.com/kubernetes/kubernetes).

There is an ongoing work for refactoring cloud providers out of the upstream repository. For more details, please check [this KEP](https://github.com/kubernetes/community/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md).

## Contributing

Please see [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on how to contribute.

### NOTE
Currently this repository is used for building and testing cloud-controller-manager for vSphere, it references vSphere cloud provider implementation code as vendor dir. After handoff, the vSphere cloud provider implementation will be moved here.