## Installation / Operations

### Component Flags

#### In-Tree vSphere Cloud Provider

To enable the vSphere cloud provider for in-tree deployments, set these additional flags on each component:

* kube-apiserver: --cloud-provider=vsphere --cloud-config=<path-to-cloud-config-file> --enable-admission-plugins=PersistentVolumeLabel,<other admission plugins>
* kube-controller-manager: --cloud-provider=vsphere --cloud-config=<path-to-cloud-config-file>
* kubelet: --cloud-provider=vsphere --cloud-config=<path-to-cloud-config-file>

#### Out-of-Tree vSphere Cloud Provider

To enable the vSphere cloud provider for out-of-tree deployments, set these additional flags on each component:
* kube-apiserver: --cloud-provider=external --enable-admission-plugins=PersistentVolumeLabel,<other admission plugins>
* kube-controller-manager: --cloud-provider=external
* kubelet: --cloud-provider=external
* cloud-controller-manager: --cloud-provider=vsphere --cloud-config=<path-to-cloud-config-file>

See the [Deploying the out-of-tree vSphere Cloud Provider](tutorials/deploying_cloud_provider_vsphere_with_rbac.md) tutorial for more details on how to install the
vSphere cloud controller manager on your clusters.

### Cloud Config

As mentioned above in [Component Flags](#component-flags), some components in your Kubernetes cluster will expect a cloud config file. This file is required by
the cloud provider integration before it can properly connect to vCenter. At a high level, the cloud config file holds the following information:
* The server URL and credentials to connect to vCenter
* The Virtual Data Center your cluster should be running on
* The Default Datastore used for dynamic volume provisioning
* The SCSI controller type
* The VM network for your Kubernetes cluster
* The zones/regions topology tags to use for your VMs

Go to the [Cloud Config Spec](cloud_config.md) page for more details on how to configure the cloud config file.
