# Kubernetes vSphere Cloud Provider

This is the official documentation for the [Kubernetes vSphere Cloud Provider](https://github.com/kubernetes/cloud-provider-vsphere/).

## Introduction

This is the official documentation for the running Kubernetes on vSphere, with particular attention paid to the Container Storage Interface and Cloud Provider Interfaces. This document covers key concepts, features, known issues, installation requirements and steps for Kubernetes clusters running on vSphere. Before reading this document, it's worth noting that a Kubernetes cluster can still run on vSphere without the cloud provider integration enabled. However, your Kubernetes clusters will not have integration with the underlying infrastructure without the Cloud Provider Interface (CPI). Note that the term Cloud Control Manager (CCM) is now being referred to as the Cloud Provider Interface (CPI). For the purposes of this document, we will be using the CPI designation to cover the CPI/CCM feature.

## History

Kubernetes, in its early days, introduced the concept of volumes to provide persistent storage to containerized applications. When a Pod was provisioned, a PersistentVolume (PV) could be connected to a Pod. The internal “Provider” code would provision the necessary storage on the underlying infrastructure, e.g. vSphere. In the early development stages of Kubernetes, implementing cloud providers natively (in-tree) was the most viable solution. However as Kubernetes popularity rose, more “providers” were added in-tree. This became unwieldy, making it unsustainable to introduce provider enhancements outside of Kubernetes releases. In other words, the only way to upgrade the provider was to also upgrade Kubernetes. Today, with many infrastructure providers supporting Kubernetes, new cloud providers are required to be out-of-tree (via a plugin) in order to grow the project sustainably.

For vSphere, the in-tree provider is called the vSphere Cloud Provider (VCP). The newer out of tree approach is made up of two distinct components, the Cloud Provider Interface (CPI) and the Container Storage Interface (CSI). The CPI handles platform specific control loops that were previously implemented by the native Kubernetes Controller Manager. The Kubernetes Controller Manager is a daemon that embeds the core control loops shipped with Kubernetes. In Kubernetes, a controller is a control loop that watches the state of a resource in the cluster through the API server and makes changes attempting to move the current state towards the desired state. Some of these control loops vary depending on the cloud or platform on which Kubernetes is running, so it made sense to move them out of tree. The Container Storage Interface (CSI) is a specification designed to enable persistent storage volume management, and is maintained independently of Kubernetes. Again, it was necessary to move this out of tree due to many different storage platforms that are now supported in Kubernetes.

This document will cover both the in-tree and out-of-tree vSphere integrations for Kubernetes. For Kubernetes clusters on vSphere, both in-tree and out-of-tree modes of operation both work. However, the out-of-tree vSphere cloud provider is strongly recommended as future releases of Kubernetes will remove support for all in-tree cloud providers. Note also that the in-tree VCP only has community support, unless support is provided by a managed/package Kubernetes offering.

## Summary

* [Concepts](concepts.md)
  * [VMware vSphere Storage Concepts](concepts/vmware_vsphere_storage.md)
  * [In-tree vs Out-of-Tree](concepts/in_tree_vs_out_of_tree.md)
  * [Overview of the VCP](concepts/vcp_overview.md)
  * [Overview of the CPI](concepts/cpi_overview.md)
  * [Overview of the CSI](concepts/csi_overview.md)
* [Glossary](glossary.md)
  * [Cloud Provider Interface (CPI)](cloud_provider_interface.md)
  * [Container Storage Interface (CSI)](container_storage_interface.md)
  * [Cloud Config Spec](cloud_config.md)

## Tutorials

### vSphere 6.7U3 tutorials

* [Deploying a new K8s cluster with CPI and CSI on vSphere 6.7U3 with kubeadm](./tutorials/kubernetes-on-vsphere-with-kubeadm.md)
* [Deploying CPI and CSI with Zones Topology](./tutorials/deploying_cpi_and_csi_with_multi_dc_vc_aka_zones.md)

### Earlier tutorials

* [Deploying K8s with vSphere Cloud Provider (in-tree) using kubeadm (deprecated)](./tutorials/k8s-vcp-on-vsphere-with-kubeadm.md)
