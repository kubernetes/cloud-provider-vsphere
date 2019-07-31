# Cluster Architecture

## Kubernetes using the in-tree vSphere Provider

The in-tree vSphere cloud provider integration is capable of connecting to vCenter in order to map information
about your infrastructure (VMs, disks, etc) back to the Kubernetes API. For the in-tree case, the kubelet,
kube-apiserver, and kube-controller-manager are natively aware of how to connect to vCenter if it is provided with a valid config
file and credentials. What the config file should look like and how the credentials are shared will be covered in
[Installing and Operating the vSphere Cloud Provider](#installing-operating-the-vsphere-cloud-provider). For now, assume that
every component has access to a config file and credentials which allow access to vCenter.

![vSphere In-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/vsphere-in-tree-architecture.png "vSphere In-Tree Cloud Provider Architecture")

**Note**: this diagram only illustrates which components in your cluster should be connecting to vCenter.

## Kubernetes using the out-of-tree-tree vSphere Provider (recommended)

The out-of-tree vSphere cloud provider integration also connects to vCenter and maps information about your infrastructure (VMs,
disks, etc) back to the Kubernetes API. For the out-of-tree case however, the only component that will ever talk to vCenter is
the cloud-controller-manager. Therefore, only the cloud-controller-manager is required to have a valid config file and credentials
in order to connnect to vCenter. Similar to the in-tree case, how to configure these will be covered in [Installing and Operating the vSphere Cloud Provider](#installing-operating-the-vsphere-cloud-provider). For now, assume that the cloud-controller-manager has access
to a confile file and credentials which allow access to vCenter.

![vSphere Out-of-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/vsphere-out-of-tree-architecture.png "vSphere Out-of-Tree Cloud Provider Architecture")

**Note**: this diagram only illustrates which components in your cluster should be connecting to vCenter.
