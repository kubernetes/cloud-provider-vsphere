# CSI - Container Storage Interface

The goal of CSI is to establish a standardized mechanism for Container Orchestration Systems (COs) to expose arbitrary storage systems to their containerized workloads. The CSI specification emerged from cooperation between community members from various COs – including; Kubernetes, Mesos, Docker, and Cloud Foundry. The specification is developed, independent of Kubernetes, and maintained [here](https://github.com/container-storage-interface/spec/blob/master/spec.md).

## Why do we need it?

Historically, Kubernetes volume plugins were “in-tree”, meaning they’re linked, compiled, built, and shipped with the core kubernetes binaries. Adding support for a new storage system to Kubernetes (a volume plugin) required checking code into the core Kubernetes repository. But aligning with the Kubernetes release process was very painful for many plugin developers. CSI, the Container Storage Interface, makes installing new volume plugins as easy as deploying a pod. It also enables third-party storage providers to develop solutions without the need to add to the core Kubernetes codebase.

CSI enables storage plugins to be developed out-of-tree, containerized, deployed via standard Kubernetes primitives, and consumed through the Kubernetes storage primitives users know and love (`PersistentVolumeClaim`s, `PersistentVolume`s, `StorageClass`es).

## Which versions of Kubernetes/vSphere support it?

With the GA release of the CSI driver, vSphere `6.7 U3` and above is required, and Kubernetes `v1.14` and above is required.

## How do I install it?

Full instructions on the setup and installation of the vSphere CSI driver and CPI can be found [here](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html).

_Note:_ The vSphere CSI driver requires the vSphere CPI to be installed as well (covered in the same article).

## How do I use/consume it?

If the vSphere CSI storage plugin is already deployed on your cluster, you can use it through the familiar Kubernetes storage primitives such as `PersistentVolumeClaim`s, `PersistentVolume`s, and `StorageClass`es.

## Do you have an example StorageClass?

See below, you should note that it is required to use a [Storage Policy](https://docs.vmware.com/en/VMware-vSphere/6.7/com.vmware.vsphere.storage.doc/GUID-89091D59-D844-46B2-94C2-35A3961D23E7.html) in vSphere - even if you are using VMFS or NFS datastores (use [tag-based](https://blogs.vmware.com/virtualblocks/2018/07/26/using-tag-based-spbm-policies-to-manage-your-storage/) policies).

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: space-efficient
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.vsphere.vmware.com
parameters:
  storagepolicyname: "Space Efficient"
```
