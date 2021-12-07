# Kubernetes vSphere Cloud Provider

This is documentation for the [Kubernetes vSphere Cloud Provider](https://github.com/kubernetes/cloud-provider-vsphere/).

## Introduction

This documentation provides information about running Kubernetes on vSphere and specifically focuses on the Cloud Provider Interface (CPI), previously called Cloud Control Manager (CCM). This documentation covers key concepts, features, known issues, installation requirements, and offers sample procedures to run Kubernetes clusters on vSphere. Note that you can continue running Kubernetes clusters on vSphere without enabling the cloud provider integration. However, if you do not use the Cloud Provider Interface, your Kubernetes clusters will not have integration with the underlying infrastructure.

## History

To provide persistent storage to containerized applications, Kubernetes introduces the concept of volumes. When a Pod is provisioned, a PersistentVolume (PV) can be connected to the Pod. Initially, the internal Kubernetes provider code was used to provision the necessary storage on the underlying infrastructure, such as vSphere. In the early development stages of Kubernetes, implementing cloud providers natively, or in-tree, was the most viable solution. However, as Kubernetes popularity rose, more providers were added as in-tree. As a result, it became unsustainable to introduce provider enhancements outside of Kubernetes releases, as the only way to upgrade the provider was to also upgrade Kubernetes. Currently, with many infrastructure providers supporting Kubernetes, new cloud providers are required to be out-of-tree, through a plugin, to increase the project sustainably.

The in-tree provider for vSphere is called the vSphere Cloud Provider (VCP). The out-of-tree solution includes two distinct components, the Cloud Provider Interface (CPI) and the Container Storage Interface (CSI). The CPI handles platform specific control loops that were previously implemented by the native Kubernetes Controller Manager. The Kubernetes Controller Manager is a daemon that embeds the core control loops shipped with Kubernetes. In Kubernetes, a controller is a control loop that watches the state of a resource in the cluster through the API server and makes changes attempting to move the current state towards the desired state. Some of these control loops vary depending on the cloud or platform on which Kubernetes is running, so it made sense to move them out of tree. The Container Storage Interface (CSI) is a specification designed to enable persistent storage volume management and is maintained independently of Kubernetes. Again, it was necessary to move this out of tree due to many different storage platforms that are currently supported in Kubernetes.

This document covers both the in-tree and out-of-tree vSphere integrations for Kubernetes. For Kubernetes clusters on vSphere, both in-tree and out-of-tree modes of operation work. However, the out-of-tree vSphere cloud provider is recommended as future releases of Kubernetes will remove support for all in-tree cloud providers. Also, the in-tree VCP only has community support, unless support is provided by a managed Kubernetes offering.

If you are looking for more information about the Container Storage Interface (CSI), please refer to [Kubernetes Container Storage Interface (CSI) Documentation](https://kubernetes-csi.github.io/docs/).

## Summary

* [Concepts](concepts.md)
  * [VMware vSphere Storage Concepts](concepts/vmware_vsphere_storage.md)
  * [In-tree vs Out-of-Tree](concepts/in_tree_vs_out_of_tree.md)
  * [Overview of the VCP](concepts/vcp_overview.md)
  * [Overview of the CPI](concepts/cpi_overview.md)
  * [Overview of the CSI](https://github.com/container-storage-interface/spec/blob/master/spec.md)
* [Glossary](glossary.md)
  * [Cloud Provider Interface (CPI)](cloud_provider_interface.md)
* [Known Issues](known_issues.md)

## Tutorials

### vSphere 6.7U3 tutorials

* [Deploying a new K8s cluster with CPI and CSI on vSphere 6.7U3 with kubeadm](./tutorials/kubernetes-on-vsphere-with-kubeadm.md)
* [Deploying CPI with Zones Topology](./tutorials/deploying_cpi_with_multi_dc_vc_aka_zones.md)

### Earlier tutorials

* [Deploying K8s with vSphere Cloud Provider (in-tree) using kubeadm (deprecated)](./tutorials/k8s-vcp-on-vsphere-with-kubeadm.md)

## Developer Guide

* [Release guide for CPI](./tutorials/make_a_new_cpi_release.md)
