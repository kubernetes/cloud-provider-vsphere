## Kubernetes Concepts

Before diving into how to operate and install your Kubernetes clusters on vSphere, it's useful to go over some well-known concepts
in Kubernetes.

### Cloud Providers

The cloud provider, from the perspective of Kubernetes, refers to the infrastructure provider for cluster resources such as
nodes, load balancers, routes, etc. Various Kubernetes components are capable of communicating with the underlying cloud provider
via APIs in order to call operations (create, update, delete) against resources required for a cluster. This capabilitiy is what
we refer to as the cloud provider integration.

As of writing this, there are two modes of cloud provider integrations: in-tree and out-of-tree. More details on these two modes below.

### In-Tree Cloud Providers

In-tree cloud providers refers to cloud provider integrations that are directly compiled and built into the core Kubernetes components.
This also means that the integration is also developed within the same source code repository as Kubernetes core. As a result, updates to the cloud

![In-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/in-tree-arch.png "Kubernetes In-Tree Cloud Provider Architecture - from k8s.io/website")

### Out-of-Tree Cloud Providers

Out-of-tree cloud provider refers to integrations that can be developed, built and released independent of Kubernetes core. This requires
adding a new component to the cluster called the cloud-controller-manager. The cloud-controller-manager is responsible for running all the
cloud-specific control loops that were previously run in core components like the kube-controller-manager and the kubelet.

![Out-of-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/out-of-tree-arch.png "Kubernetes Out-of-Tree Cloud Provider Architecture - from k8s.io/website")

### In-Tree vs Out-of-Tree

As of writing this, in-tree cloud providers are only supported for historical reasons. In the early development stages of Kubernetes, implementing
cloud providers natively (in-tree) was the most viable solution. Today, with many infrastructure providers supporting Kubernetes, new cloud providers
are requried to be out-of-tree in order to grow the project sustainably. For the existing in-tree cloud providers, there's an effort to extract/migrate
clusters to use out-of-tree cloud providers, see [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20190125-removing-in-tree-providers.md) for more detais.

For Kubernetes clusters on vSphere, both in-tree and out-of-tree modes of operation are supported. However, the out-of-tree vSphere
cloud provider is strongly recommended as future releases of Kubernetes will remove support for all in-tree cloud providers.
Regardless, this document will cover both the in-tree and out-of-tree vSphere integration for Kubernetes.

