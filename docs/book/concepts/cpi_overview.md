# Overview of the Cloud Provider Interface

The Cloud Provider Interface (CPI) project decouples intelligence of underlying cloud infrastructure features from the core Kubernetes project. The out-of-tree CPI provides Kubernetes with details about the infrastructure on which it has been deployed. When a Kubernetes node registers itself with the Kubernetes API server, it requests additional information about itself from the cloud provider. The CPI provides the node object in the Kubernetes cluster with its IP addresses and zone/region topology. When the node understands the topology and hierarchy of the underlying infrastructure, more intelligent application placement decisions can be made. See the [Cloud Provider Interface (CPI)](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/cloud_provider_interface.md) for more details.

The out-of-tree CPI integration connects to vCenter Server and maps information about your infrastructure, such as VMs, disks, and so on, back to the Kubernetes API.  Only the cloud-controller-manager pod is required to have a valid config file and credentials to connect to vCenter Server. The following chapters offer more information on how to configure this provider. For now, assume that the cloud-controller-manager pod has access to the config file and credentials that allow access to vCenter Server. The following simplified diagram illustrates which components in your cluster should be connecting to vCenter Server.

![vSphere Out-of-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/vsphere-out-of-tree-architecture.png "vSphere Out-of-Tree Cloud Provider Architecture")

## Overview of the Container Storage Interface

The [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) is a specification designed to enable persistent storage volume management on Container Orchestrators (COs) such as Kubernetes. The specification allows storage systems to integrate with containerized workloads running on Kubernetes. Using CSI, storage providers, such as VMware, can write and deploy plugins for storage systems in Kubernetes without a need to modify any core Kubernetes code.

CSI allows volume plugins to be installed on Kubernetes clusters as extensions. Once a CSI compatible volume driver is deployed on a Kubernetes cluster, users can use the CSI to provision, attach, mount, and format the volumes exposed by the CSI driver. For vSphere, the CSI driver is csi.vsphere.vmware.com.

## Dependency between CPI and CSI

On Kubernetes, the vSphere CSI driver is used in conjunction with the out-of-tree vSphere CPI. The CPI initializes nodes with labels describing the topology information, such as zone and region. In the case of vSphere, these labels are inherited from vSphere tags, and are applied to the Kubernetes nodes as labels. Pods can then be provisioned using a variety of constraint options, such as node selector, or affinity and anti-affinity rules. The CSI driver can deploy Persistent Volumes (PVs) using the same constraints as Pods. Some of the tutorials explain the relationship between CPI and CSI in a more detailed way.
