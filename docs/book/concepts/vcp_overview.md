# Overview of the VCP - vSphere Cloud Provider

Project Hatchway was VMware’s first container project. It offered vSphere storage infrastructure choices to container environments, from hyper-converged infrastructure (HCI) powered by VMware vSAN to traditional SAN and NAS storage. There were two distinct parts to the project initially – one focusing on docker container volumes and the other focusing on Kubernetes. Both aimed to provision VMDKs (block volumes) on vSphere storage to provide a persistent storage solution for containerized applications running in a Container Orchestrator on vSphere. The Kubernetes part became known as the vSphere Cloud Provider (VCP) and was included in-tree in Kubernetes distributions since Kubernetes version v1.6.5. This enabled both static and dynamic consumption of vSphere storage from Kubernetes. It was also fully integrated with Storage Policy Based Management, meaning that Persistent Volumes could also inherit and select capabilities of the underlying storage infrastructure, e.g. RAID levels, encryption, deduplication, compression, etc.

The in-tree vSphere cloud provider integration connects to vCenter in order to map information about your infrastructure (VMs, disks, etc) back to the Kubernetes API. For the in-tree case, the kubelet, kube-apiserver, and kube-controller-manager are natively aware of how to connect to vCenter if it is provided with a valid config file and credentials. What the config file should look like and how the credentials are shared will be covered later. For now, assume that every component has access to a config file and credentials which allow access to vCenter. The simplified diagram below illustrates which components in your cluster should be connecting to vCenter.

![vSphere In-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/vsphere-in-tree-architecture.png "vSphere In-Tree Cloud Provider Architecture")