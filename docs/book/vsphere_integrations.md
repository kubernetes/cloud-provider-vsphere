## vSphere Integrations

Once the vSphere Cloud Provider is fully functional on your cluster, your cluster will have access to new integration points
with vSphere. Below are the key integrations that are enabled by the vSphere cloud provider.

### Kubernetes Nodes

When a Kubernetes node registers itself with the Kubernetes API server, it will request additional information about itself from the cloud provider.
As of today, the cloud provider will provide a new `Node` object in the cluster with it's node addresses, instance type and zone/region topology. More on each below.

#### Node Addresses

A Kubernetes `Node` has a set of addresses as part of its status field. The Kubernetes API server will use those addresses set on the `Node` object in order to communicate with the
Kubelet running on a given node. For example, when you run `kubectl logs <pod>` against your cluster, the `kube-apiserver` will proxy the log request to the kubelet where the pod is running
using one of the addresses set on the `Node` object. If no addresses is set on the node, the logs request will fail since the `kube-apiserver` will not know how to communicate with the kubelet.

As of today, there are 5 address types: `Hostname`, `InternalIP`, `InternalDNS`, `ExternalIP`, `ExternalDNS. You can read more about each address type in the [Kubernetes docs](https://kubernetes.io/docs/concepts/architecture/nodes/#addresses).
The priority in which each address type should be used is configurable on the `kube-apiserver` with the flag `--kubelet-preferred-address-types`. For example, if the flag is set to
`--kubelet-preferred-address-type=InternalIP,InternalDNS,ExternalIP`, then the `kube-apiserver` will try the `InternalIP`, then the `InternalDNS` and finally the `ExternalIP`, stopping
after the first successful connection to the kubelet. The default preferred address types are `Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP`.

The cloud provider integration comes into play here since the `kubelet` may not necessarily know all of its address types. In this case, the kubelet will request it's node addresses from the
cloud provider. Often it will fetch the addresses of your VM from vCenter directly and apply those addresses to Kubernetes nodes. Often times the `InternalIP` and the `ExternalIP` will be the same
since on-premise VMs are often not given a public IP.

You can see the node addresses set by the vSphere cloud provider with the following:

```
$ kubectl get no k8s-node01 -o yaml
apiVersion: v1
kind: Node
metadata:
  name: k8s-node01
  ...
  ...
spec:
  providerID: vsphere://4230c9af-0daa-4175-60da-1658d1b8234d
status:
  addresses:
  - address: 192.168.3.94
    type: InternalIP
  - address: 192.168.3.94
    type: ExternalIP
  - address: k8s-node01
    type: Hostname
  allocatable:
    cpu: "4"
    memory: 6012708Ki
    pods: "110"
  capacity:
    cpu: "4"
    memory: 6115108Ki
    pods: "110"
```

#### Node Instance Type

A Kubernetes cloud provider has the ability to propagate the "instance type" of a node as a label on the Kubernetes node object during registration. For the case of the out-of-tree vSphere cloud provider,
every node should have a node label like the following:

```bash
$ kubectl get node k8s-node01 -o yaml
apiVersion: v1
kind: Node
metadata:
  name: k8s-node01
  labels:
    kubernetes.io/hostname: k8s-node01
    beta.kubernetes.io/instance-type: vsphere-vm.cpu-4.mem-4gb.os-ubuntu
```

The instance type label follows the format `vsphere-vm.cpu-<num of cpus>.mem-<num GBs of memory>gb.os-<Guest OS shorthand>`.

The instance type label is generally useful and can be used for:
* querying all nodes of a certain size (CPU + MEM)
* a node selector on pods so it can be only scheduled on a specific instance type in your cluster
* node affinity/anti-affinity based on instance type, more on node affinity in the [Kubernetes docs](https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#affinity-and-anti-affinity).

**NOTE**: the in-tree vSphere cloud provider does not set any instance type labels.

#### Node Zones/Regions Topology

Similar to instance type, the cloud provider can also apply zones and region labels to your Kubernetes nodes. The zones and region topology labels are interesting because they originate from use-cases derived from
public cloud providers where VMs are provisioned in physical zones and regions. For the case of vSphere, physical zones and regions may not always apply. However, the vSphere cloud provider allows you to configure
the "zones" and "regions" topology arbitrarily on your clusters. This gives the vSphere admin flexibility to configure zones/region topology based on their use-case. A vSphere admin can enable zones/regions support
by tagging VMs on vSphere with the desired zones/regions. To learn more about how to enable and operate zones on your cluster, see the [Zones Support Tutorial](TODO LINK).

Once you have zones or regions enabled on your cluster, you can verify zones support by looking for the `failure-domain.beta.kubernetes.io/zone` and `failure-domain.beta.kubernetes.io/region` labels on your nodes.
For this specific example, each rack represents a zone and the entire datacenter located in SFO represents a region.
```bash
$ kubectl get node k8s-node01 -o yaml
apiVersion: v1
kind: Node
metadata:
  name: k8s-node01
  labels:
    kubernetes.io/hostname: k8s-node01
    failure-domain.beta.kubernetes.io/zone: rack12
    failure-domain.beta.kubernetes.io/region: dc-sfo
```

### Kubenetes LoadBalancers

### Kubernetes Routes

### Kubernetes Persistent Volumes

The in-tree vSphere cloud provider supports the PersistentVolume type [vsphereVolume](https://kubernetes.io/docs/concepts/storage/volumes/#vspherevolume).
PersistentVolumes of type `vsphereVolume` are used to mount a vSphere VMDK Volume to your Pods. The contents of a volume are preserved when it is unmounted. It supports both VMFS and vSAN datastores.

To learn about Kubernetes storage on vSphere in more detail, go to the [vSphere Storage for Kubernetes docs](https://vmware.github.io/vsphere-storage-for-kubernetes/documentation/).
