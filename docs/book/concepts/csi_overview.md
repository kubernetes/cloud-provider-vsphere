# Overview of the Container Storage Interface

The [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) is a specification designed to enable persistent storage volume management on Container Orchestrators (COs) such as Kubernetes. The specification allows storage systems to integrate with containerized workloads running on Kubernetes. Using CSI, storage providers, such as VMware, can write and deploy plugins for storage systems in Kubernetes without a need to modify any core Kubernetes code.

CSI allows volume plugins to be installed on Kubernetes clusters as extensions. Once a CSI compatible volume driver is deployed on a Kubernetes cluster, users can use the CSI to provision, attach, mount, and format the volumes exposed by the CSI driver. For vSphere, the CSI driver is csi.vsphere.vmware.com.

## Dependency between CPI and CSI

On Kubernetes, the vSphere CSI driver is used in conjunction with the out-of-tree vSphere CPI. The CPI initializes nodes with labels describing the topology information, such as zone and region. In the case of vSphere, these labels are inherited from vSphere tags, and are applied to the Kubernetes nodes as labels. Pods can then be provisioned using a variety of constraint options, such as node selector, or affinity and anti-affinity rules. The CSI driver can deploy Persistent Volumes (PVs) using the same constraints as Pods. Some of the tutorials explain the relationship between CPI and CSI in a more detailed way.
