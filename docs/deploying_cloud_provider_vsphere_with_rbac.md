# Deploying `cloud-provider-vsphere` with RBAC

This document is designed to quickly get you up and running with the `cloud-provider-vsphere` external Cloud Controller Manager (CCM).

## Deployment Overview

Steps that will be covered in deploying `cloud-provider-vpshere`:

1. Set all your Kubernetes nodes to run using an external cloud controller manager.
2. Configure your vsphere.conf file and create a `configmap` of your settings.
3. (Optional, but recommended) Storing vCenter creds in a Kubernetes Secret
4. Create the RBAC roles for `cloud-provider-vsphere`.
5. Create the RBAC role bindings for `cloud-provider-vsphere`.
6. Deploy `cloud-provider-vsphere` using either the Pod or DaemonSet YAML.

## Deploying `cloud-provider-vsphere`

#### 1. Set the Kubernetes cluster to use an external Cloud Controller Manager

This is already covered in the Kubernetes documentation. Please take a look at configuration guidelines located [here](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager).

#### 2. Creating a `configmap` of your vSphere configuration

There are 2 methods for configuring the `cloud-provider-vsphere`:
- Using a Kubernetes Secret
- Within the vsphere.conf

It's highly recommended that you store your vCenter credentials in a Kubernetes secret for added security.

An example [vsphere.conf](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere.conf) is located in the `cloud-provider-vsphere` repo in the [manifests/controller-manager](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/manifests/controller-manager) directory for reference.

##### Method 1: Storing vCenter Credentials in a Kubernetes Secret

Example vsphere.conf contents if the vCenter credentials are going to be stored using a Kubernetes Secret:

```
[Global]
# properties in this section will be used for all specified vCenters unless overriden in VirtualCenter section.

secret-name = "Kubernetes Secret containing creds in the namespace below"
secret-namespace = "Kubernetes namespace for CCM deploy"
service-account = "Kubernetes service account used for CCM deploy" #Default: cloud-controller-manager

port = "443" #Optional
insecure-flag = "1" #set to 1 if the vCenter uses a self-signed cert
datacenters = "list of datacenters where Kubernetes node VMs are present"

[VirtualCenter "1.2.3.4"]
# Override specific properties for this Virtual Center.
        datacenters = "list of datacenters where Kubernetes node VMs are present"
        # port, insecure-flag will be used from Global section.

[VirtualCenter "10.0.0.1"]
# Override specific properties for this Virtual Center.
        port = "448"
        insecure-flag = "0"
        # datacenters will be used from Global section.

[Network]
        public-network = "Network Name to be used"
```

Configure your vsphere.conf file and create a `configmap` of your settings using the following command:

```bash
[k8suser@k8master ~]$ kubectl create configmap cloud-config --from-file=vsphere.conf --namespace=kube-system
```

##### Method 2: Storing vCenter Credentials in the vsphere.conf File

Example vsphere.conf contents if the vCenter credentials are going to be stored within the configuration file:

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

[VirtualCenter "10.0.0.1"]
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

#### 3. (Optional, but recommended) Storing vCenter credentials in a Kubernetes Secret

If you choose to store your vCenter credentials within a Kubernetes Secret (method 1 above), an example [Secrets YAML](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vccm-secret.yaml) is provided for reference. Both the vCenter username and password is base64 encoded within the secret. If you have multiple vCenters (as in the example vsphere.conf file), your Kubernetes Secret YAML will look like the following:

```
apiVersion: v1
kind: Secret
metadata:
  name: vccm
  namespace: kube-system
data:
  1.2.3.4.username: "Replace with output from `echo -n YOUR_VCENTER_USERNAME | base64`"
  1.2.3.4.password: "Replace with output from `echo -n YOUR_VCENTER_PASSWORD | base64`"
  10.0.0.1.username: "Replace with output from `echo -n YOUR_VCENTER_USERNAME | base64`"
  10.0.0.1.password: "Replace with output from `echo -n YOUR_VCENTER_PASSWORD | base64`"
```

Create the secret by running the folowing command:

```bash
[k8suser@k8master ~]$ kubectl create -f vccm-secret.yaml
```

#### 4. Create the RBAC roles

You can find the RBAC roles required by the provider in [cloud-controller-manager-roles.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/cloud-controller-manager-roles.yaml).

To apply them to your Kubernetes cluster, run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f cloud-controller-manager-roles.yaml
```

#### 5. Create the RBAC role bindings

You can find the RBAC role bindings required by the provider in [cloud-controller-manager-role-bindings.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/cloud-controller-manager-role-bindings.yaml).

To apply them to your Kubernetes cluster, run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f cloud-controller-manager-role-bindings.yaml
```

#### 6. Deploy `cloud-provider-vsphere` CCM

You have two options for deploying `cloud-provider-vsphere`. It can be deployed either as a simple Pod or in a DaemonSet. There really isn't much difference between the two other than the DaemonSet will do leader election and the Pod will just assume to be the leader.

**IMPORTANT NOTES:**
- Deploy either as a Pod or in a DaemonSet, but *DO NOT* deploy both.
- The YAML to deploy as a Pod or a DaemonSet assume that your Kubernetes cluster was deployed using [kubeadm](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/). If you deployed your cluster using alternate means, you will need to modify the either of the YAML files in order to provided necessary files or paths based on your deployment.

##### Deploy `cloud-provider-vsphere` as a Pod

The YAML to deploy `cloud-provider-vsphere` as a Pod can be found in [vsphere-cloud-controller-manager-pod.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere-cloud-controller-manager-pod.yaml).

Run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f vsphere-cloud-controller-manager-pod.yaml
```

##### Deploy `cloud-provider-vsphere` as a DaemonSet

The YAML to deploy `cloud-provider-vsphere` as a DaemonSet can be found in [vsphere-cloud-controller-manager-ds.yaml](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere-cloud-controller-manager-ds.yaml).

Run the following command:

```bash
[k8suser@k8master ~]$ kubectl create -f vsphere-cloud-controller-manager-ds.yaml
```

## Wrapping Up

That's it! Pretty straightforward. Questions, comments, concerns... please stop by the #sig-vmware channel at [kubernetes.slack.com](https://kubernetes.slack.com).
