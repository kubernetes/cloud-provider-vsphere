# Deploying `cloud-provider-vsphere` with RBAC

This document is designed to quickly get you up and running with the `cloud-provider-vsphere` external Cloud Controller Manager (CCM).

## Deployment Overview

Steps that will covered in deploying `cloud-provider-vpshere`:

1. Set all your Kubernetes nodes to run using an external cloud controller manager.
2. Configure your vsphere.conf file file and create a `configmap` you settings.
3. Create the RBAC roles for `cloud-provider-vsphere`.
4. Create the RBAC role bindings for `cloud-provider-vsphere`.
5. Deploy `cloud-provider-vsphere` using either the Pod or DaemonSet YAML.

## Deploying `cloud-provider-vsphere`

#### 1. Set the Kubernetes cluster to use an external Cloud Controller Manager

This is already covered in the Kubernetes documentation. Please take a look at configuration guidelines located [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager).


#### 2. Creating a `configmap` of your vSphere configuration

An example [vsphere.conf](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere.conf) is located in the `cloud-provider-vsphere` repo in the [manifests/controller-manager](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/manifests/controller-manager) directory.

Example vsphere.conf contents:

```
[Global]
# properties in this section will be used for all specified vCenters unless overriden in VirtualCenter section.

user = "vCenter username for cloud provider"
password = "password"        
port = "443" #Optional
insecure-flag = "1" #set to 1 if the vCenter uses a self-signed cert
datacenters = "list of datacenters where Kubernetes node VMs are present"

[VirtualCenter "1.2.3.4"]
# Override specific properties for this Virtual Center.
        user = "vCenter username for cloud provider"
        password = "password"
        # port, insecure-flag, datacenters will be used from Global section.

[VirtualCenter "1.2.3.5"]
# Override specific properties for this Virtual Center.
        port = "448"
        insecure-flag = "0"
        # user, password, datacenters will be used from Global section.

[Network]
        public-network = "Network Name to be used"
```

Configure your vsphere.conf file and create a `configmap` of your settings using the following command:

```bash
[k8suser@k8master ~]$ kubectl create configmap cloud-config --from-file=vsphere.conf --namespace=kube-system
```

#### 3. Create the RBAC roles

You can find the RBAC roles required by the provider in [cloud-controller-manager-roles.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/cloud-controller-manager-roles.yaml).

To apply them to your Kubernetes cluster, run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f cloud-controller-manager-roles.yaml
```

#### 4. Create the RBAC role bindings

You can find the RBAC role bindings required by the provider in [cloud-controller-manager-role-bindings.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/cloud-controller-manager-role-bindings.yaml).

To apply them to your Kubernetes cluster, run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f cloud-controller-manager-role-bindings.yaml
```

#### 5. Deploy `cloud-provider-vsphere` CCM

You have two options for deploying `cloud-provider-vsphere`. It can be deployed either as a simple Pod or in a DaemonSet. There really isn't much difference between the two other than the DaemonSet will do leader election and the Pod will just assume to be the leader.

**IMPORTANT NOTE:** Deploy either as a Pod or in a DaemonSet, but *DO NOT* deploy both.

##### Deploy `cloud-provider-vsphere` as a Pod

The YAML to deploy `cloud-provider-vsphere` as a Pod can be found in [vsphere-cloud-controller-manager-pod.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere-cloud-controller-manager-pod.yaml).

The run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f vsphere-cloud-controller-manager-pod.yaml
```

##### Deploy `cloud-provider-vsphere` as a DaemonSet

The YAML to deploy `cloud-provider-vsphere` as a DaemonSet can be found in [vsphere-cloud-controller-manager-ds.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere-cloud-controller-manager-ds.yaml).

The run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f vsphere-cloud-controller-manager-ds.yaml
```

## Wrapping Up

That's it! Pretty straightforward. Questions, comments, concerns... please stop by the #sig-vmware channel at [kubernetes.slack.com](https://kubernetes.slack.com).
