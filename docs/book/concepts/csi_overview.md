# Overview of the CSI - Container Storage Interface

The [Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) is a specification designed to enable persistent storage volume management on Container Orchestrators (COs) such as Kubernetes. This allows storage systems to integrate with containerized workloads running on Kubernetes. Using CSI, third-party storage providers (like VMware) can write and deploy plugins for storage systems in Kubernetes without a need to modify any core Kubernetes code.

CSI allows volume plugins to be deployed (installed) on Kubernetes clusters as extensions. Once a CSI compatible volumea driver is deployed on a Kubernetes cluster, users may use the CSI to provision, attach, mount and format the volumes exposed by the CSI driver. In the case of vSphere, the CSI driver is block.vsphere.csi.vmware.com. We will see an example of how to use this shortly.

## Dependency between CPI and CSI

On Kubernetes, the CSI driver is used in conjunction with the out of tree vSphere CPI. In particular, the CPI will initialize nodes with labels describing the topology information such as Zone/Region. In the case of vSphere, these labels are inherited from vSphere tags, and applied to the Kubernetes nodes as labels. Pods can then be provisioned using a variety of constraint options, such as node selector, or affinity/anti-affinity rules. The CSI driver is able to deploy Persistent Volumes (PVs) using the same constraints as Pods. In some of the tutorials, you will be able to clearly see the relationship between CPI and CSI.
