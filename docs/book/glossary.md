# Glossary

## VM

A virtual machine (VM) is a software computer that, like a physical computer, runs an operating system and applications. The VM abstracts the operating system from physical hardware resources, with which the OS typically interacts, and creates virtual representations of these resources.
The virtual hardware resources include CPU, memory, disks, and so on. The VM is a self-contained entity and shares no components with the host OS. 
In the vSphere environment, the host OS is ESXi.

## vSphere

vSphere is a VMware suite of virtualization products and technologies, among which ESXi and vCenter Server are the core components.

## ESXi

ESXi is a VMware hypervisor, or host operating system, that installs on a physical server and enables VMs with different operating systems to run side-by-side. ESXi provides strong separation between VMs and itself, offering security boundaries between the guest operating systems and the host.
ESXi can be used as a standalone entity without vCenter Server. However, you need vCenter Server to use such essential features as vSphere HA, vSphere vMotion, Workload Balance, and vSAN.

## vCenter Server

vCenter Server is VMware server management software that provides a centralized platform for managing vSphere resources and controlling ESXi hosts and VMs. You use vCenter Server as a single console to arrange ESXi hosts into data centers, clusters, or resources pools. vCenter Server provides higher level vSphere feature, including vSphere vMotion, vSAN, vSphere HA, vSphere DRS, vSphere Distributed Switch, and so on. vCenter Server also serves as an integration point for other products in the VMware SDDC stack, third-party solutions, or networking overlay applications, such as NSX.

## govmomi

govmomi is a Go library for interacting with VMware vSphere APIs, a set of interfaces used to manage ESXi and vCenter Server.

## Kubernetes

Kubernetes (K8s) is an open-source system for automating deployment, scaling, and management of containerized applications.

(source: [kubernetes.io](https://kubernetes.io/))

## kube-apiserver

The Kubernetes API server validates and configures data for the api objects which include pods, services, replicationcontrollers, and others.
The API Server services REST operations and provides the frontend to the cluster’s shared state through which all other components interact.

(source: [kubernetes.io](https://kubernetes.io/))

## kube-controller-manager

The Kubernetes controller manager is a daemon that embeds the core control loops shipped with Kubernetes. In applications of robotics and automation,
a control loop is a non-terminating loop that regulates the state of the system. In Kubernetes, a controller is a control loop that watches the
shared state of the cluster through the apiserver and makes changes attempting to move the current state towards the desired state. Examples of
controllers that ship with Kubernetes today are the replication controller, endpoints controller, namespace controller, and serviceaccounts controller.

(source: [kubernetes.io](https://kubernetes.io/))

## kube-scheduler

The Kubernetes scheduler is a policy-rich, topology-aware, workload-specific function that significantly impacts availability, performance, and capacity.
The scheduler needs to take into account individual and collective resource requirements, quality of service requirements, hardware/software/policy
constraints, affinity and anti-affinity specifications, data locality, inter-workload interference, deadlines, and so on. Workload-specific requirements
will be exposed through the API as necessary.

(source: [kubernetes.io](https://kubernetes.io/))

**NOTE**: kube-scheduler never asks the cloud provider for any information pertaining to scheduling. However, it might depend on information on resources
that were placed by other components like the kubelet.

## kubelet

The kubelet is the primary “node agent” that runs on each node. The kubelet works in terms of a PodSpec. A PodSpec is a YAML or JSON object that
describes a pod. The kubelet takes a set of PodSpecs that are provided through various mechanisms (primarily through the apiserver) and ensures
that the containers described in those PodSpecs are running and healthy. The kubelet doesn’t manage containers which were not created by Kubernetes.

(source: [kubernetes.io](https://kubernetes.io/))

## kube-proxy

The Kubernetes network proxy runs on each node. This reflects services as defined in the Kubernetes API on each node and can do simple TCP, UDP,
and SCTP stream forwarding or round robin TCP, UDP, and SCTP forwarding across a set of backends. Service cluster IPs and ports are currently found
through Docker-links-compatible environment variables specifying ports opened by the service proxy. There is an optional addon that provides cluster DNS
for these cluster IPs. The user must create a service with the apiserver API to configure the proxy.

(source: [kubernetes.io](https://kubernetes.io/))

**NOTE**: kube-proxy never asks the cloud provider for any information pertaining to network proxy. However, it might depend on information on resources
that were placed by other components like the kube-controller-manager.

## cloud-controller-manager

The Kubernetes cloud controller manager is a daemon that embeds the cloud specific control loops shipped with Kubernetes. Each cloud provider can run
the cloud controller manager as an addon to their cluster. The cloud-controller-manager defines a specification (Go interface) that must be implemented
by every cloud provider. Various expectations and behaviors of the cluster can be tuned based on the implementation set by the cloud provider.
Cloud providers are also free to run custom controllers as part of the cloud-controller-manager.

(source: [kubernetes.io](https://kubernetes.io/))

## etcd

Consistent and highly-available key value store used as Kubernetes’ backing store for all cluster data.

(source: [kubernetes.io](https://kubernetes.io/))
