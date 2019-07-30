# Kubernetes vSphere Cloud Provider

This is the official documentation for the [Kubernetes vSphere Cloud Provider](https://github.com/kubernetes/cloud-provider-vsphere/).

## Introduction

This is the official documentation for the Kubernetes vSphere cloud provider integration. This document covers key concepts,
features, known issues, installation requirements and steps for Kubernetes clusters running on vSphere. Before reading this
document, it's worth noting that a Kubernetes cluster can run on vSphere without the cloud provider integration enabled, however,
your Kubernetes clusters will not have features that require integration with the underlying infrastructure/cloud provider.

## Summary

* [Kubernetes Concepts](kubernetes_concepts.md)
* [Components and Tools](components_and_tools.md)
* [Cluster Architecture](cluster_architecture.md)
* [vSphere Integrations](vsphere_integrations.md)
* [Installation / Operations](installation_and_operations.md)
* [Cloud Config Spec](cloud_config.md)

## Tutorials

* [Running a Kubernetes cluster on vSphere with kubeadm](./tutorials/kubernetes-on-vsphere-with-kubeadm.md)
* [Deploying the out-of-tree vSphere Cloud Provider](./tutorials/deploying_cloud_provider_vsphere_with_rbac.md)
* [Deploying CCM and CSI with Zones Topology](./tutorials/deploying_ccm_and_csi_with_multi_dc_vc_aka_zones.md)
