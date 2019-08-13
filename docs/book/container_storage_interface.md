# CSI - Container Storage Interface

The goal of CSI is to establish a standardized mechanism for Container Orchestration Systems (COs) to expose arbitrary storage systems to their containerized workloads. The CSI specification emerged from cooperation between community members from various Container Orchestration Systems (COs)–including Kubernetes, Mesos, Docker, and Cloud Foundry. The specification is developed, independent of Kubernetes, and maintained [here](https://github.com/container-storage-interface/spec/blob/master/spec.md).

## Why do we need it?

Historically, Kubernetes volume plugins were “in-tree”, meaning they’re linked, compiled, built, and shipped with the core kubernetes binaries. Adding support for a new storage system to Kubernetes (a volume plugin) required checking code into the core Kubernetes repository. But aligning with the Kubernetes release process was very painful for many plugin developers. CSI, the Container Storage Interface, makes installing new volume plugins as easy as deploying a pod. It also enables third-party storage providers to develop solutions without the need to add to the core Kubernetes codebase.

CSI enables storage plugins to be developed out-of-tree, containerized, deployed via standard Kubernetes primitives, and consumed through the Kubernetes storage primitives users know and love (PersistentVolumeClaims, PersistentVolumes, StorageClasses).

## How do I get it?

TBD

## Which versions of Kubernetes/vSphere support it?

TBD

## How do I install it?

See the [Deploying csi-vsphere docs](https://github.com/kubernetes-sigs/vsphere-csi-driver/blob/master/docs/deploying_csi_vsphere_with_rbac.md) for install steps. Note that the
vSphere CSI driver depends on the vSphere CPI to be running.

## How do I use/consume it?

If the vSphere CSI storage plugin is already deployed on your cluster, you can use it through the familiar Kubernetes storage primitives such as PersistentVolumeClaims, PersistentVolumes, and StorageClasses.
