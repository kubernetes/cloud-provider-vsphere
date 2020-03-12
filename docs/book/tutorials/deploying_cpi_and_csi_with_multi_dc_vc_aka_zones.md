# Deploying the vSphere CPI and CSI in a Multi-vCenter OR Multi-Datacenter Environment using Zones

This document is designed to quickly get you up and running in a vSphere configuration that consists of multiple vCenter or a multiple Datacenter environment via using zones.

Note: These steps need to be done at initial Kubernetes cluster deployment. It is not possible to add zone support after the Kubernetes cluster has been deployed.

## Prerequisites

This document assumes that you have read and understood the setup documentation for both the vSphere Cloud Provider Interface (also known as the vSphere Cloud Controller Manager - CCM) and  vSphere Container Storage Interface (CSI) driver. This guide will go over the additional zone-based configuration needed to support a multi-vCenter or multi-Datacenter environment by using the previous documentation as a base. If you need to revisit the base CPI and CSI documentation, you can find the documentation links below:

[Deploying Kubernetes Cluster on vSphere with CPI and CSI](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/tutorials/kubernetes-on-vsphere-with-kubeadm.md)

## Why Do We Need to Use Zones in a Multi-vCenter or Multi-Datacenter Environment

There exist 2 significant issues when deploying Kubernetes workloads or pods in a mutli-vCenter or single vCenter with multiple Datacenters. They are:

1. Datastore objects, specifically names and even morefs (Managed Object References), are not unique across vCenters instances
2. Datastore objects, specifically names, are not unique within a single vCenter since objects of the same name can exist in different Datacenters

![Which datastore?](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/whichdatastore.png)

There needs to be a mechanism in place to allow end-users to continue to use the human readable "friendly" names for objects like datastores and datastore clusters and still be able to target workloads to use resources from them. This is where the concept of zones or zoning comes in. Zones allow you to partition datacenters and compute clusters so that the end-user can target workloads to specific locations in your vSphere environment.

## Understanding Optimal Zone Configurations

This section outlines some optimal configurations for Kubernetes zones in your vSphere environment/configuration. The implementation for zone support in the CPI and CSI driver are quite flexible but there are some configurations that can take advantage of features in vSphere and thus providing certain benefits. Here are a couple of common deployment scenarios for zones. If you cannot roll out or deploy zones in some of these suggested configurations, it might be worth consulting someone with familiarity with how zones are implemented.

### Zones Per Cluster

An ideal configuration is creating a zone per cluster. It follows that datastore and datastore clusters access be tied to the compute nodes within a given cluster. The main reason for this is to take advantage of the High Availability (HA) that clusters offer as well as features like vMotion and etc. Example diagrams or configurations appear below.

![Cluster-based Zones](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/clusterbased.png)

### Zones Per Datacenter

Zones per datacenter can work as well, but there are some very important design considerations when doing this. If this deployment strategy is taken, it is important to understand that all compute nodes in that zone aka datacenter have access to provision VMDKs from a given shared datastore. The reason for this is CSI driver uses zones in order to target Kubenetes pods or workloads when provisioning external storage. Example diagrams or configurations appear below.

![Datacenter-based Zones](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/datacenterbased.png)

## Pitfalls: Zones as they Relate to Storage

Here is a great example of a mistake you don't want to make. In a multi-Datacenter or multi-vCenter environment, you need to use to zones in an effective way especially when it comes to the use of persistent storage. The picture below shows an example of how zones can be incorrectly used.

We have two clusters in `Datacenter 1`. If we deploy a pod to `Zone Engineering` what cluster will the pod land on? If you don't care, then this topology will work, but if you want to run a stateful pod with some storage to be provisioned, then placement really does matter. The `StorageClass` explicitly calls out from what datastore you want to provision a storage, or in our case a VMDK, out of. So placement in that case is very important and the zone configuration here is **insufficient** to handle that deployment scenario.

![Pitfalls of Zones](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/pitfalls.png)

## Wrap-Up Zone Considerations

Some important takeaways for implementing zones:

1. Zones allow you to target Kubernetes workloads to a specific group of vSphere infrastructure. This is handled by the CPI.
2. Zones also define persistent storage boundaries. In other words, all compute nodes within a given zone must have access to shared storage if persistent storage (aka an FCD) is to be provisioned for stateful applications/pods/workloads.

## Deployment Overview

Steps that will be covered in order to setup zones for the vSphere CPI, vSphere CSI driver, and vSphere environment/configuration:

1. Enabling Zones the `vsphere.conf` file
2. Creating Zones in your vSphere Environment via Tags
3. Updating your `StorageClass` when using Persistent Storage
4. Example: Deploying a Kubernetes pod to a Specific Zone using Persistent Storage

## Deploying Zones using the CPI and CSI driver

### 1. Enabling Zones the `vsphere.conf` file

> ***Note:*** The CSI and CPI drivers have their own vsphere.conf files. The following modifications need to be made in both configurations.

The zones implementation depends on 2 sets of vSphere tags to be used on objects, such as datacenters or clusters. The first is a `region` tag and the second is a `zone` tag. vSphere tags are very simply put key/value pairs that can be assigned to objects and instead of using fixed keys to denote a `region` or a `zone`, we give the end-user the ability to come up with their own keys for a `region` and `zone` in the form of vSphere Tag Catagory. It just allows for a level of indirection in case you already have regions and zones setup in your configuration. Once a key/label or vSphere Tag Category is selected for each, create a `labels:` section in the `vsphere.conf` then assign tag names for both `region` and `zone`.

In the example `vsphere.conf` below, `k8s-region` and `k8s-zone` was selected:

```bash
# Global properties in this section will be used for all specified vCenters unless overriden in VirtualCenter section.
global:
  user: YourVCenterUser
  password: YourVCenterPass
  port: "443"
  # set insecure-flag to true if the vCenter uses a self-signed cert
  insecure-flag: true
  # settings for using k8s secret
  secret-name: cpi-secret
  secret-namespace: kube-system

# VirtualCenter section
virtualcenter:
  tenant1:
    user: YourVCenterUser
    password: YourVCenterPass
    server: 10.0.0.1
    datacenters:
      - mydc1
  tenant2:
    server: 127.0.0.1
    port: 448
    insecure-flag: false
    datacenters:
      - myotherdc1
      - myotherdc2

# labels for regions and zones
labels:
  region: k8s-region
  zone: k8s-zone
```

### 2. Creating Zones in your vSphere Environment via Tags

 The `region` tag is just a construct that allows one to make a grouping for a specific set of resources. It could be used to indicate something like a geographic location like a country or perhaps a specific datacenter. This label is an arbitrary grouping that you decide on. The `zone` tag is another construct that allows you to further subdivide resources within a `region`. As an example, using the countries as a `region`, the `zone` could indicate a specific datacenter out of a list in that `region`. In the second example of using a datacenter as a `region`, you might use a `zone` to indicate a specific rack within the datacenter or even just a cluster within that datacenter. Then all hosts and subsequently all VMs acting as Kubernetes worker nodes under that tagged datacenter or cluster inherit the tags of those parent objects. How one chooses to group regions and zones is completely based on how you want to identify a specific group of resources.

There are many options for creating vSphere tags. One such method would be to use [govc](https://github.com/vmware/govmomi/tree/master/govc). All the examples below will make use of this method. You could also create tags by accessing the vSphere REST APIs directly or by using the vSphere UI.

> **NOTE**: The example commands below assume that you have exported the GOVC_URL before running said commands:

```bash
[k8suser@k8master ~]$ export GOVC_URL=https://REPLACE_VSPHERE_USERNAME:REPLACE_VSPHERE_PASSWORD@REPLACE_VSPHERE_IP/sdk
```

Using the example above, if it is decided that `k8s-region` and `k8s-zone` are to be used for your Category labels, then you can create those vSphere Categories using `govc` by running the following command:

```bash
[k8suser@k8master ~]$ ./govc tags.category.create -d "Kubernetes region" k8s-region
[k8suser@k8master ~]$ ./govc tags.category.create -d "Kubernetes zone" k8s-zone
```

Say there are 2 `regions` in the US and EU that cover our vSphere environment, we can then create 2 region tags `k8s-region-us` and `k8s-region-eu` using `govc` by running the following command:

```bash
[k8suser@k8master ~]$ ./govc tags.create -d "Kubernetes Region US" -c k8s-region k8s-region-us
[k8suser@k8master ~]$ ./govc tags.create -d "Kubernetes Region EU" -c k8s-region k8s-region-eu
```

Now say our colocations in those regions are fairly small and we have 2 datacenters (dcwest and dceast) in the US and 1 datacenter (dceu) in the EU each with just a small vSphere cluster in each datacenter. Let's each datacenter could represent a particular `zone` in those `regions`. In this example, we could simply create tags for each datacenter, such as `k8s-region-us-west`, `k8s-region-us-east` and `k8s-region-eu-all`, by running the following command:

```bash
[k8suser@k8master ~]$ ./govc tags.create -d "Kubernetes Zone US West" -c k8s-zone k8s-zone-us-west
[k8suser@k8master ~]$ ./govc tags.create -d "Kubernetes Zone US East" -c k8s-zone k8s-zone-us-east
[k8suser@k8master ~]$ ./govc tags.create -d "Kubernetes Zone EU All" -c k8s-zone k8s-region-eu-all
```

Now let's assign the region and zone tags to each of the datacenters in the vSphere environment by running the following command:

```bash
#dcwest
[k8suser@k8master ~]$ ./govc tags.attach k8s-region k8s-region-us /dcwest
[k8suser@k8master ~]$ ./govc tags.attach k8s-zone k8s-zone-us-west /dcwest
#dceast
[k8suser@k8master ~]$ ./govc tags.attach k8s-region k8s-region-us /dceast
[k8suser@k8master ~]$ ./govc tags.attach k8s-zone k8s-zone-us-east /dceast
#dceu
[k8suser@k8master ~]$ ./govc tags.attach k8s-region k8s-region-eu /dceu
[k8suser@k8master ~]$ ./govc tags.attach k8s-zone k8s-region-eu-all /dceu
```

And there you go! All setup with the correct tags.

> **NOTE**: Since the CPI and CSI driver support multiple vCenter Servers, the datacenters in the US and EU could be distinctly different. In that case, the `govc` commands would be identical with the exception of replacing the proper vCenter username, password, and IP address for each command.

### 3. Updating your `StorageClass` when using Persistent Storage

Now that we have set the regions and zones within the vSphere environment, we can now target a specific region/zone to deploy a Kubernetes workload or pod into. If a persistent volume is required for that given Kubernetes pod, we need to update the `StorageClass` with the `region` and `zone` information that the particular datastore is in. This is what the `StorageClass` YAML might look like:

```yaml
kind: StorageClass
apiVersion: storage.k8s.io/v1
metadata:
  name: vsphere-csi
  namespace: kube-system
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.vsphere.vmware.com
parameters:
  datastoreurl: "URL_OF_DATASTORE" # optional parameter
  storagepolicyname: "STORAGE_POLICY" # optional parameter
  fstype: "FILESYSTEM_TYPE" # optional parameter
allowedTopologies:
- matchLabelExpressions:
  - key: failure-domain.beta.kubernetes.io/zone
    values:
    - IF_USING_ZONES_REPLACE_WITH_ZONE_VALUE
  - key: failure-domain.beta.kubernetes.io/region
    values:
    - IF_USING_ZONES_REPLACE_WITH_REGION_VALUE
```

### 4. Example: Deploying a Kubernetes pod to a Specific Zone using Persistent Storage

Now if one wanted to deploy a Kubernetes pod into a specific `region` and `zone`  also using the persistent volume above, the YAML would look something like this:

```yaml
kind: Pod
apiVersion: v1
metadata:
  name: my-csi-app
spec:
  containers:
    - name: my-frontend
      image: busybox
      volumeMounts:
      - mountPath: "/data"
        name: my-csi-volume
      command: [ "sleep", "1000000" ]
  volumes:
    - name: my-csi-volume
      persistentVolumeClaim:
        claimName: vsphere-csi-pvc
```

*IMPORTANT*: Just to re-emphasize topics discussed in this document, the datastore or datastore cluster that the persistent volume is to be provisioned from must be available to the `region` and `zone` and by all hosts within that `region` and `zone` pairing since Kubernetes is what is performing the scheduling of pods.

## Wrapping Up

That's it! Pretty straightforward. Questions, comments, concerns... please stop by the #sig-vmware channel at [kubernetes.slack.com](https://kubernetes.slack.com).
