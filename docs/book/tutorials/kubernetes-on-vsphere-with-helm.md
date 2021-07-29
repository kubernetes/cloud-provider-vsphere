# Deploying vSphere CPI using Helm

The purpose of this guide is to provide the reader with step by step instructions on how to deploy the vSphere CPI on vSphere infrastructure using [Helm](https://github.com/helm/helm). The reader will learn how to install and configure Helm as well as learn about basic Helm operations. At the end of this tutorial you will have a fully configured vSphere CPI running on a vSphere environment.

## Prerequisites

Review the comprehensive guide locate at [Deploying a Kubernetes Cluster on vSphere with CSI and CPI](./kubernetes-on-vsphere-with-kubeadm.md). The prerequisites found in that guide also apply when deploying the CPI via Helm. This guide also assumes that you have a Kubernetes cluster up and running. If you need assistance in setting up a Kubernetes cluster, please refer to the [Deploying a Kubernetes Cluster on vSphere with CSI and CPI](./kubernetes-on-vsphere-with-kubeadm.md) guide for cluster setup instructions.

### Helm requirements

[Helm charts](https://github.com/helm/charts) has been fully deprecated since Nov 13th 2020.

The [Helm Chart for vSphere CPI](https://github.com/helm/charts/tree/master/stable/vsphere-cpi) has been moved to [this repo](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/charts/vsphere-cpi). It has been tested and verified working using Helm v.2.16.X and v3.0.0+. It is highly recommended that Helm v3.0.0+ be used when deploying the Helm Chart for vSphere CPI. At any point should you have additional questions regarding Helm, please visit this website for the official [Helm documentation](https://helm.sh/docs/).

## Setting up Helm

Before you begin, all outlined steps below are carried out on the master node only.

### Option 1: From the Binary Releases

Download the [latest release](https://github.com/helm/helm/releases) of Helm appropriate for your platform. At the time of writing this Helm quickstart guide, the latest release is [v3.6.0](https://github.com/helm/helm/releases/tag/v3.6.0). It is highly recommended using Helm v3.0.0+ when deploying the Helm Chart for vSphere CPI.

After the download is complete, unpack and install Helm v3.0.0+.

```bash
# tar -zxvf helm-v3.6.0-linux-amd64.tgz
# sudo mv linux-amd64/helm /usr/local/bin/helm
```

### Option 2: From Script

```bash
curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/master/scripts/get-helm-3
chmod 700 get_helm.sh
./get_helm.sh
```

See [Install Helm](https://helm.sh/docs/intro/install/) for more options.

Verify that Helm installed successfully by running the following command:

```bash
helm help
```

### Check that all nodes are tainted

Before continuing, make sure all nodes are tainted with `node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule`. When the kubelet is started with "external" cloud provider, this taint is set on a node to mark it as unusable. After a controller from the cloud provider initializes this node, the kubelet removes this taint.

```bash
# kubectl describe nodes | egrep "Taints:|Name:"
Name:               k8s-master
Taints:             node-role.kubernetes.io/master:NoSchedule
Name:               k8s-node1
Taints:             node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule
Name:               k8s-node2
Taints:             node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule
Name:               k8s-node3
Taints:             node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule
Name:               k8s-node4
Taints:             node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule
```

## Installing the Chart using Helm 3.0+

To add the Helm Stable Charts for cloud-provider-vsphere, you can run the following command:

```bash
helm repo add vsphere-cpi https://kubernetes.github.io/cloud-provider-vsphere
helm repo update
```

See [help repo](https://helm.sh/docs/helm/helm_repo/) for command documentation.

## Deploying vSphere CPI for Simple Configurations

If your vSphere environment contains only a single vCenter Server where the default `vsphere.conf` is acceptable, this section should be sufficient for your deployment needs. You can deploy the Helm Chart for vSphere CPI using the follwing single Helm command:

```bash
# helm upgrade --install vsphere-cpi vsphere-cpi/vsphere-cpi --namespace kube-system --set config.enabled=true --set config.vcenter=<vCenter IP> --set config.username=<vCenter Username> --set config.password=<vCenter Password> --set config.datacenter=<vCenter Datacenter>
```

Here we use the '--set' flag to override values in a chart and pass configuration from the command line.

The following is a description of the fields used in the vsphere.conf configmap:

* `config.enabled` should be set to true to enable the functionality to create the configMap and secret
* `config.vcenter` the IP address or FQDN for your vCenter Server should be specified here
* `config.username` holds the username to be used for your vCenter Server
* `config.password` holds the password to be used for your vCenter Server
* `config.datacenter` should be the list of all comma separated datacenters where kubernetes node VMs are present.

## Deploying vSphere CPI for Advanced Configurations

If your vSphere environment contains multiple vCenter Servers or the default parameters contained within the `vsphere.conf` must be changed, you can deploy the Helm Chart using the procedure below.

1. Create a CPI configMap
2. Create a CPI secret
3. Deploy vSphere CPI using Helm

### Create a CPI configMap

This cloud-config configmap file, passed to the CPI on initialization, contains details about the vSphere configuration. This file, which here we have called `vsphere.conf` has been populated with some sample values. Obviously, you will need to modify this file to reflect your own vSphere configuration.

```bash
# tee /etc/kubernetes/vsphere.conf >/dev/null <<EOF
[Global]
port = "443"
insecure-flag = "true"
secret-name = "cpi-global-secret"
secret-namespace = "kube-system"

[VirtualCenter "1.1.1.1"]
datacenters = "finance"

[VirtualCenter "192.168.0.1"]
datacenters = "hr"

[VirtualCenter "10.0.0.1"]
datacenters = "engineering"
secret-name = "cpi-engineering-secret"
secret-namespace = "kube-system"

[Labels]
region = "k8s-region"
zone = "k8s-zone"

EOF
```

Here is a description of the fields used in the vsphere.conf configmap.

* `insecure-flag` should be set to true to use self-signed certificate for login
* `VirtualCenter` section is defined to hold property of vcenter. IP address and FQDN should be specified here.
* `secret-name` holds the credential(s) for a single or list of vCenter Servers.
* `secret-namespace` is set to the namespace where the secret has been created.
* `port` is the vCenter Server Port. The default is 443 if not specified.
* `datacenters` should be the list of all comma separated datacenters where kubernetes node VMs are present.

Create the configmap by running the following command:

```bash
# cd /etc/kubernetes
```

```bash
# kubectl create configmap cloud-config --from-file=vsphere.conf --namespace=kube-system
```

Verify that the configmap has been successfully created in the kube-system namespace.

```bash
# kubectl get configmap cloud-config --namespace=kube-system
NAME              DATA     AGE
cloud-config      1        82s
```

### Create a CPI secret

The CPI supports storing vCenter credentials either in:

* a shared global secret containing all vCenter credentials, or
* a secret dedicated for a particular vCenter configuration which takes precedence over anything that might be configured within the global secret

In the example `vsphere.conf` above, there are two configured [Kubernetes secret](https://kubernetes.io/docs/concepts/configuration/secret/#using-secrets). The vCenter at `10.0.0.1` contains credentials in the secret named `cpi-engineering-secret` in the namespace `kube-system` and the vCenter at `1.1.1.1` and `192.168.0.1` contains credentials in the secret named `cpi-global-secret` in the namespace `kube-system` defined in the `[Global]` section.

An example [Secrets YAML](https://raw.githubusercontent.com/kubernetes/cloud-provider-vsphere/master/manifests/controller-manager/vccm-secret.yaml) can be used for reference when creating your own `secrets`. If the example secret YAML is used, update the secret name to use a `<unique secret name>`, the vCenter IP address in the keys of `stringData`, and the `username` and `password` for each key.

The secret for the vCenter at `1.1.1.1` might look like the following:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cpi-engineering-secret
  namespace: kube-system
stringData:
  10.0.0.1.username: "administrator@vsphere.local"
  10.0.0.1.password: "password"
```

Then to create the secret, run the following command replacing the name of the YAML file with the one you have used:

```bash
# kubectl create -f cpi-engineering-secret.yaml
```

Verify that the credential secret is successfully created in the kube-system namespace.

```bash
# kubectl get secret cpi-engineering-secret --namespace=kube-system
NAME                    TYPE     DATA   AGE
cpi-engineering-secret   Opaque   1      43s
```

If you have multiple vCenters as in the example vsphere.conf above, your Kubernetes Secret YAML could look like the following to storage the vCenter credentials for vCenters at `1.1.1.1` and `192.168.0.1`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cpi-global-secret
  namespace: kube-system
stringData:
  1.1.1.1.username: "administrator@vsphere.local"
  1.1.1.1.password: "password"
  192.168.0.1.username: "administrator@vsphere.local"
  192.168.0.1.password: "password"
```

### Deploy vSphere CPI using Helm

Once the `configMap` and `secret` have been created, deploy the Helm Chart for vSphere CPI by running the following command:

```bash
# helm install vsphere-cpi vsphere-cpi/vsphere-cpi
```

### Verify that the CPI has been successfully deployed

You can verify the vSphere CPI deployed succesfully by listing the Helm Charts currently deployed.

```bash
# helm list
```

Next verify **vsphere-cloud-controller-manager** is running and all other system pods are up and running (note that the coredns pods were not running previously - they should be running now as the taints have been removed by installing the CPI):

```bash
# kubectl get pods -n kube-system
NAME                                     READY   STATUS    RESTARTS   AGE
coredns-fb8b8dccf-bq7qq                  1/1     Running   0          71m
coredns-fb8b8dccf-r47q2                  1/1     Running   0          71m
etcd-k8s-master                          1/1     Running   0          69m
kube-apiserver-k8s-master                1/1     Running   0          70m
kube-controller-manager-k8s-master       1/1     Running   0          69m
kube-flannel-ds-amd64-7kmk9              1/1     Running   0          38m
kube-flannel-ds-amd64-dtvbg              1/1     Running   0          63m
kube-flannel-ds-amd64-hq57c              1/1     Running   0          30m
kube-flannel-ds-amd64-j7g4s              1/1     Running   0          22m
kube-flannel-ds-amd64-q4zsn              1/1     Running   0          21m
kube-proxy-6jcng                         1/1     Running   0          30m
kube-proxy-bh8kh                         1/1     Running   0          21m
kube-proxy-rb9xp                         1/1     Running   0          22m
kube-proxy-srhpj                         1/1     Running   0          71m
kube-proxy-vh4lg                         1/1     Running   0          38m
kube-scheduler-k8s-master                1/1     Running   0          70m
vsphere-cloud-controller-manager-549hb   1/1     Running   0          25s
```

### Check that all nodes are untainted

Verify `node.cloudprovider.kubernetes.io/uninitialized` taint is removed from all nodes.

```bash
# kubectl describe nodes | egrep "Taints:|Name:"
Name:               k8s-master
Taints:             node-role.kubernetes.io/master:NoSchedule
Name:               k8s-node1
Taints:             <none>
Name:               k8s-node2
Taints:             <none>
Name:               k8s-node3
Taints:             <none>
Name:               k8s-node4
Taints:             <none>
```

Note: If you happen to make an error with the `vsphere.conf`, simply delete the Helm deployment of vSphere CPI (describe in the next section below), the configMap, and the secret, then make any necessary edits to the configMap `vSphere.conf` file and/or secret, and reapply the steps above.

You may now remove the `vsphere.conf` file created at `/etc/kubernetes/`.

## Uninstall the Helm Chart for vSphere CPI

To uninstall/delete the vsphere-cpi deployment:

```bash
# Helm 3
$ helm uninstall [RELEASE_NAME]
```

You can delete the `configMap` and `secret` for the vSphere CPI if they are no longer needed.
