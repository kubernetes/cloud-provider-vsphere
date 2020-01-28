# Introduction

This guide assumes you have an existing Kubernetes cluster, set up with either Kubeadm, or manually and covers only the enabling and troubleshooting of the vSphere CSI driver.

## Infrastructure prerequisites

This section will cover the prerequisites that need to be in place before attempting the deployment.

### vSphere requirements

vSphere 6.7U3 (or later) is a prerequisite for using CSI and CPI at the time of writing. This may change going forward, and the documentation will be updated to reflect any changes in this support statement. If you are on a vSphere version that is below 6.7 U3, you can either upgrade vSphere to 6.7U3 or follow one of the tutorials for earlier vSphere versions. Here is the tutorial on deploying Kubernetes with kubeadm, using the VCP - [Deploying Kubernetes using kubeadm with the vSphere Cloud Provider (in-tree)](./k8s-vcp-on-vsphere-with-kubeadm.md).

### Firewall requirements

Providing the K8s master node(s) access to the vCenter management interface will be sufficient, given the CPI and CSI pods are deployed on the master node(s). Should these components be deployed on worker nodes or otherwise - those nodes will also need access to the vCenter management interface.

If you want to use topology-aware volume provisioning and the late binding feature using `zone`/`region`, the node needs to discover its topology by connecting to the vCenter, for this every node should be able to communicate to the vCenter. You can disable this optional feature if you want to open only the master node to the vCenter management interface.

### Virtual Machine Hardware requirements

Virtual Machine Hardware must be `version 15` or higher. For Virtual Machine CPU and Memory requirements, size adequately based on workload requirements.
VMware also recommend that virtual machines use the VMware Paravirtual SCSI controller for Primary Disk on the Node VM. This should be the default, but it is always good practice to check.

Finally, the `disk.EnableUUID` parameter must be set for each node VMs. This step is necessary so that the VMDK always presents a consistent UUID to the VM, thus allowing the disk to be mounted properly.
It is recommended to not take snapshots of CNS node VMs to avoid errors and unpredictable behavior.

#### disk.EnableUUID=1

The following govc commands will set the disk.EnableUUID=1 on all nodes.

```sh
export GOVC_INSECURE=1
export GOVC_URL='https://<VC_IP>'
export GOVC_USERNAME=VC_Admin_User
export GOVC_PASSWORD=VC_Admin_Passwd
```

Check the connection to vCenter:

```sh
$ govc ls
/datacenter/vm
/datacenter/network
/datacenter/host
/datacenter/datastore
```

To retrieve all Node VMs, use the following command:

```sh
$ govc ls /<datacenter-name>/vm
/datacenter/vm/k8s-node3
/datacenter/vm/k8s-node4
/datacenter/vm/k8s-node1
/datacenter/vm/k8s-node2
/datacenter/vm/k8s-master
```

To use govc to enable Disk UUID, use the following command:

```sh
govc vm.change -vm '/datacenter/vm/k8s-node1' -e="disk.enableUUID=1"
govc vm.change -vm '/datacenter/vm/k8s-node2' -e="disk.enableUUID=1"
govc vm.change -vm '/datacenter/vm/k8s-node3' -e="disk.enableUUID=1"
govc vm.change -vm '/datacenter/vm/k8s-node4' -e="disk.enableUUID=1"
govc vm.change -vm '/datacenter/vm/k8s-master' -e="disk.enableUUID=1"
```

Further information on disk.enableUUID can be found in [VMware Knowledgebase Article 52815](https://kb.vmware.com/s/article/52815).

#### Upgrade Virtual Machine Hardware

VM Hardware should be at version 15 or higher.

```bash
govc vm.upgrade -version=15 -vm '/datacenter/vm/k8s-node1'
govc vm.upgrade -version=15 -vm '/datacenter/vm/k8s-node2'
govc vm.upgrade -version=15 -vm '/datacenter/vm/k8s-node3'
govc vm.upgrade -version=15 -vm '/datacenter/vm/k8s-node4'
govc vm.upgrade -version=15 -vm '/datacenter/vm/k8s-master'
```

Check the VM Hardware version after running the above command:

```bash
$ govc vm.option.info '/datacenter/vm/k8s-node1' | grep HwVersion
HwVersion:           15
```

## Kubernetes changes

### Node-level changes

On each K8s node, set the `kubelet`’s `cloud-provider` flag to `external` on all nodes. This flag needs to be set in the service configuration file (usually `/etc/systemd/system/kubelet.service`) but this depends on how you installed Kubernetes or the distribution you are using.

E.g:

```sh
--cloud-provider=external
```

Restart the `kubelet` service on each node.

```sh
systemctl daemon-reload
systemctl restart kubelet.service
```

### Kubernetes manifest changes

Set taints on all nodes to allow them to be initialised by the vSphere Cloud Provider Interface, this allows them to have their `providerID` populated, which creates the link between the CSI and the VM in vCenter.

On worker nodes set this taint:

```sh
kubectl taint nodes <your k8s node name> node.cloudprovider.kubernetes.io/uninitialized=true:NoSchedule
```

On master nodes set this taint:

```sh
kubectl taint nodes <your k8s master node name> node-role.kubernetes.io/master=:NoSchedule
```

### Install the vSphere Cloud Provider Interface

Please refer to this guide for details on installing the CPI – <https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/tutorials/kubernetes-on-vsphere-with-kubeadm.md#install-the-vsphere-cloud-provider-interface>

**Note: Taints needs to be set on the nodes BEFORE the installation of the CPI.**

### Install the vSphere CSI Driver

Please refer to this guide for details on installing the CSI Driver - <https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/tutorials/kubernetes-on-vsphere-with-kubeadm.md#install-the-vsphere-csi-driver>
