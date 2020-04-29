# CPI - Cloud Provider Interface

## Why do we need it?

The Kubernetes Controller Manager (KCM) is a daemon that embeds the core control loops shipped with Kubernetes. The Cloud Provider Interface is responsible for running all the platform specific control loops that were previously run in core Kubernetes components like the KCM and the kubelet, but have been moved out-of-tree to allow cloud and infrastructure providers to implement integrations that can be developed, built and released independent of Kubernetes core.

Since Kubernetes is a declarative system, the purpose of these control loops is to watch the actual state of the system through the API server.  If the actual state is different from the desired/declared state, it initiates operations to rectify the situation by making changes to try to move the current state towards the desired state.

When an application is deployed in Kubernetes, the application definition (the desired end-state of the application) is persisted in etcd via the API server on the K8s master node.

The API server holds both a record of the desired state and another record of the actual state (real world observed state). When these records differ, a controller is responsible for initiating tasks to rectify the difference. This could be something as simple as a request to add a PersistentVolume to a Pod. In this case the desired state is different from the actual state, so the controllers will initiate tasks to make them the same, i.e. whatever is needed to attach the correct PV to the Pod.

The Cloud Provider Interface (CPI) replaces the Kubernetes Controller Manager for only the  cloud specific control loops. The Cloud Provider breaks away some of the functionality of Kubernetes controller manager (KCM) and handles it separately. Note that in many cases, some of these interfaces are not relevant for some CPIs, so you may only see a subset of these interfaces implemented for your cloud provider.

Interfaces (optionally) implemented in the Cloud Provider are as follows:

* **Node control loops**, provide cloud specific info about nodes in your cluster. It does a number of tasks:
  * Initialize a node with cloud specific zone/region labels
  * Initialize a node with cloud specific instance details, for example, type and size
  * Obtain the node’s IP addresses and hostname
  * In case a node becomes unresponsive, check the cloud to see if the node has been deleted from the cloud. If the node has been deleted from the cloud, delete the Kubernetes Node object.
* **Route control loops**, provide cloud specific info about networking. It is responsible for configuring network routes in the infrastructre so that containers on different nodes in the Kubernetes cluster can communicate with each other. At the time of writing, the route controller is only applicable for Google Compute Engine clusters, and is not applicable to vSphere.
* **Service control loops** - responsible for listening to K8s Service type create, update, and delete events. This is also known as a Load Balancer control loop, a cloud specific ingress controller. Based on the current state of the services in Kubernetes, it configures load balancers (such as Amazon ELB , Google LB, or Oracle Cloud Infrastructure LB) to reflect the state of the services in Kubernetes. Additionally, it ensures that service backends for load balancers are up to date. vSphere does not have a native load balancer per-se. However, VMware’s network virtualization product, NSX, can be used to provide such functionality to K8s.
* There is no volume controller now. This responsibility has been taken over by the CSI, the Container Storage Initiative. So for vSphere, when transitioning from in-tree to out-of tree, you need both a CPI and CSI to get the same level of functionalist as the previous vSphere Cloud Provider (VCP), e.g. zones for placement.
* **Custom control loops** - implementations of the CPI can also run custom controllers that enhance the cluster’s capabilities specific to the underlying infrastructure platform. vSphere today does not run any custom controllers but may introduce them in the future.

![Out-of-Tree Cloud Provider Architecture](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/out-of-tree-arch.png "Kubernetes Out-of-Tree Cloud Provider Architecture - from k8s.io/website")

The vSphere Cloud Provider does not implement LoadBalancer, Clusters or Routes - these networking features are not available in native vSphere. The vSphere CPI is only implementing node instances in the vSphere CPI. The credentials to connect to vSphere is managed with a Kubernetes secrets file. Zone support is significant because vSphere has its own concept of zones/fault domains. The CPI maps those vSphere Zone concepts to Kubernetes Zone concepts. vSphere tags are used to identify zones and regions in vSphere datacenter objects. These same tags are mapped to labels in Kubernetes, allowing placement of Nodes and thus Pods and Persistent Volumes in the appropriate Zone or Region.

## CPI Integration Detailed

Once the vSphere Cloud Provider is fully functional on your cluster, your cluster will have access to new integration points with vSphere. Below are the key integrations that are enabled by the vSphere cloud provider.

### Kubernetes Nodes

When a Kubernetes node registers itself with the Kubernetes API server, it will request additional information about itself from the cloud provider. As of today, the cloud provider will provide a new Node object in the cluster with it's node addresses, instance type and zone/region topology.

### Node Addresses

A Kubernetes Node has a set of addresses as part of its status field. The Kubernetes API server will use those addresses in order to communicate with the Kubelet running on a given node. For example, when you run kubectl logs $pod against your cluster, the API server will proxy the log request to the kubelet where the pod is running using one of the addresses set on the Node object. If no addresses are set on the node, the logs request will fail since the kube-apiserver will not know how to communicate with the kubelet.

As of today, there are 5 address types: Hostname, InternalIP, InternalDNS, ExternalIP, ExternalDNS. You can read more about each address type in the Kubernetes docs.

The priority in which each address type should be used is configurable on the API server with the flag --kubelet-preferred-address-types. For example, if the flag is set to --kubelet-preferred-address-type=InternalIP,InternalDNS,ExternalIP, then the kube-apiserver will try the InternalIP, then the InternalDNS and finally the ExternalIP, stopping after the first successful connection to the kubelet. The default preferred address types are Hostname,InternalDNS,InternalIP,ExternalDNS,ExternalIP.

The cloud provider integration comes into play here since the kubelet may not necessarily know all of its address types. In this case, the kubelet will request it's node addresses from the cloud provider. Often it will fetch the addresses of your VM from vCenter directly and apply those addresses to Kubernetes nodes. Often times the InternalIP and the ExternalIP will be the same since on-premises VMs are often not given a public IP.

You can see the node addresses set by the vSphere cloud provider with the following:

![Example vSphere Node Addresses](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/cpi_node_addresses_example.png "Example vSphere Node Addresses")

### Node Instance Type

A Kubernetes cloud provider has the ability to propagate the "instance type" of a node as a label on the Kubernetes node object during registration. For the case of the out-of-tree vSphere cloud provider, every node should have a node label similar to the following:

![Example vSphere Node Instance Type](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/cpi_node_instance_type_example.png "Example vSphere Node Instance Type")

The instance type label follows the format vsphere-vm.cpu-[num of cpus].mem-[num GBs of memory]gb.os-[Guest OS shorthand].

The instance type label is generally useful and can be used for:

* Querying all nodes of a certain size (CPU + MEM)
* A node selector on pods so it can be only scheduled on a specific instance type in your cluster
* Node affinity/anti-affinity based on instance type, more on node affinity in the Kubernetes docs.

**NOTE**: the in-tree vSphere cloud provider does not set any instance type labels.

### Node Zones/Regions Topology

Similar to instance type, the cloud provider can also apply zones and region labels to your Kubernetes nodes. The zones and region topology labels are interesting because they originate from use-cases derived from public cloud providers where VMs are provisioned in physical zones and regions. For the case of vSphere, physical zones and regions may not always apply. However, the vSphere cloud provider allows you to configure the "zones" and "regions" topology arbitrarily on your clusters. This gives the vSphere admin flexibility to configure zones/region topology based on their use-case. A vSphere admin can enable zones/regions support by tagging VMs on vSphere with the desired zones/regions. To learn more about how to enable and operate zones on your cluster, see the [Zones Support Tutorial](https://github.com/cormachogan/cloud-provider-vsphere/blob/master/docs/book/tutorials/deploying_cpi_and_csi_with_multi_dc_vc_aka_zones.md).

Once you have zones or regions enabled on your cluster, you can verify zones support by looking for the failure-domain.beta.kubernetes.io/zone and failure-domain.beta.kubernetes.io/region labels on your nodes. For this specific example, each rack represents a zone and the entire datacenter located in SFO represents a region.

![Example vSphere Zones Topology](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/docs/images/cpi_zones_topology_example.png "Example vSphere Zones Topology")

### WaitForFirstConsumer

WaitForFirstConsumer is a volumeBindingMode in the StorageClass which is used for deploying PersistentVolumes (PVs) on the same node that the Pod is scheduled on. This is extremely useful in terms of zones and topologies as it means that the PV creation holds off until the Pod has been scheduled, and the PV is then instantiated on the same node as the Pod. It addresses an issue for storage backends that are not globally accessible from all nodes, and could result in the Pod and PV being instantiated on different nodes. The issue is addressed by delaying the binding and provisioning of the PV, and passing the 'SelectedNode' attribute to the volume create routine once the Pod is scheduled. The result is that the PV is now provisioned using the same constraints as the Pod, for example, node selector, pod affinity/anti-affinity, etc.

Click here to learn more about [WaitForFirstConsumer](https://kubernetes.io/docs/concepts/storage/storage-classes/#volume-binding-mode).

### AllowedTopologies

allowedTopologies was used in the StorageClass to restrict provisioning to specific topologies. It was used to address topologies where storage backends were not available to all nodes. With allowedTopologies, you could control the nodes on which PersistentVolumes were instantiated through the use of zone and region labels discussed earlier.

The use of allowedTopolgies should become less common now that WaitForFirstConsumer is available. However, it is still possible to use allowedTopologies in the StorageClass if there is a desire to control the placement of volumes to certains zones and regions.

Click here to learn more about [Allowed Topologies](https://kubernetes.io/docs/concepts/storage/storage-classes/#allowed-topologies).

## How do I get it?

There are 3 components that make up the CPI. These are the cluster roles (RBAC), role bindings and the service, service account and daemonset for running the CPI.

* [Cloud Controlle Manager Roles](https://raw.githubusercontent.com/kubernetes/cloud-provider-vsphere/master/manifests/controller-manager/cloud-controller-manager-roles.yaml)
* [Cloud Controller Manage Role Bindings](https://raw.githubusercontent.com/kubernetes/cloud-provider-vsphere/master/manifests/controller-manager/cloud-controller-manager-role-bindings.yaml)
* [Cloud Controller Manager DaemonSet](https://github.com/kubernetes/cloud-provider-vsphere/raw/master/manifests/controller-manager/vsphere-cloud-controller-manager-ds.yaml)

## Which versions of Kubernetes/vSphere support it?

CPI is supported for vSphere versions 6.7U3 or later. CPI also requires a Kubernetes version >= v1.11.

## How do I install it?

See the [Deploying cloud-provider-vsphere docs](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/deploying_cloud_provider_vsphere_with_rbac.html) for install steps.
