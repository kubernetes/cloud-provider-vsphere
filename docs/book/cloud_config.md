# vSphere Cloud Config Spec

The vSphere cloud config file is a requirement for the vSphere cloud provider. It should be accessible by various
Kubernetes components that enable the vSphere cloud provider. It is passed off to those components using the
`--cloud-config` flag. The `--cloud-config` flag indicates the path to the cloud config file on the host's filesystem.

At a high level, the cloud config file holds the following information:

* The server URL and credentials to connect to vCenter
* The Virtual Data Center your cluster should be running on
* The Default Datastore used for dynamic volume provisioning
* The SCSI disk controller type
* The VM network for your Kubernetes cluster
* The zones/regions topology tags to use for your VMs

The rest of this page will dive into each section and field of the cloud config file in more detail.

## Cloud Config Spec

Here's the entire cloud config spec using example values:

```bash
[Global]
  datacenters = "SDDC-Datacenter"
  insecure-flag = "1" # set to 1 if the vCenter uses a self-signed cert
  user = "viadmin-global@vmware.local"
  password = "my-secure-global-password"
  server = "10.0.0.1"
  port = "443"
  ca-file = "/etc/kubernetes/vcenter-ca.crt"
  thumbprint = "<certificate thumbprint>"
  datastore = ""
  working-dir = ""
  soap-roundtrip-count = ""
  vm-uuid = ""
  vm-name = ""
  secret-name = ""
  secret-namespace = ""

[VirtualCenter "10.0.0.1"]
  user = "viadmin@vmare.local"
  password = "my-secure-password"
  port = "443"
  datacenters = "SDDC-Datacenter"
  soap-roundtrip-count = "1"
  thumbprint = ""

[Workspace]
  server = "10.0.0.1"
  datacenter = "SDDC-Datacenter"
  default-datastore = "MyDefaultDatastore"
  resourcepool-path = "*/Resources/Compute-ResourcePool/kubernetes-clusters"

[Disk]
  scsicontrollertype = pvscsi

[Network]
  public-network = "sddc-network-01"

[Labels]
  region = k8s-region
  zone = k8s-zone
```

There are 6 sections in the cloud config file, let's break down the fields in each section:

### Global

The `Global` section of the cloud config holds general information about your cluster's environment.
You'll notice that many fields in `Global` overlap with fields from other sections. In most cases,
fields in `Global` become the default if they are not specified in other sections.

```bash
[Global]
  # The name of the Virtual Data Center your cluster is in
  datacenters = "SDDC-Datacenter"

  # Set to 1 if the vCenter uses a self-signed cert, 0 or unset otherwise
  insecure-flag = "1"

  # The user when connecting to vCenter
  user = "viadmin-global@vmware.local"

  # The password for the user above when connecting to vCenter
  password = "my-secure-global-password" # the

  # The vCenter server IP to connect to
  server = "10.0.0.1"

  # The vCenter server port to connect to
  port = "443"

  # The CA file to be trusted when connecting to vCenter. If not set, the node's
  # CA certificates will be used.
  ca-file = "/etc/kubernetes/vcenter-ca.crt"

  # The vCenter certificate thumbprint, this ensures the correct certificate is used
  thumbprint = "<certificate thumbprint>"

  # The default datastore where VMDKs are stored
  # DEPRECATED: see datstore under Workspace instead
  datastore = ""

  # The vSphere path where VMs can be found.
  # DEPRECATED: see working-dir in Workspace
  working-dir = ""

  # SOAP round trip counter
  soap-roundtrip-count = ""

  # The VM instance UUID of a virtual machine. This is used so any processes on the same machine
  # can identify which VM it is running on. If not set, the VM UUID is fetched from the machine's
  # filesystem which requires root access
  vm-uuid = ""

  # VMName is the name of the virtual machine
  # DEPRECATED: since the name of VMs are now automatically discovered
  vm-name = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the name of the secret resource
  secret-name = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the namespace of the secret resource
  secret-namespace = ""
```

### VirtualCenter

The `VirtualCenter` section holds connection information to a vCenter server. Note that the name of the
section should be the server IP of the vCenter server.

```bash
[VirtualCenter "10.0.0.1"] # the server IP is 10.0.0.1
  # The user to connect to this vCenter server
  user = "viadmin@vmare.local"

  # The password to connect to this vCenter server
  password = "my-secure-password"

  # The vCenter port to connect to
  port = "443"

  # The default datacenter to use when connecting to this vCenter server
  # If not set, defaults to the datacenters listed in the Global section
  datacenters = "SDDC-Datacenter"

  # SOAP round trip counter for this vCenter server
  # If not set, defaults to what is set in the Global section
  soap-roundtrip-count = "1"

  # The vCenter certificate thumbprint, this ensures the correct certificate is used
  # If not set, defaults to the thumbprint specified in the Global section
  thumbprint = ""
```

### Workspace

The `Workspace` section only applies to parameters that should be used only when creating Persistent Volumes.
**NOTE**: this section is only available in the in-tree vSphere cloud provider.

```bash
[Workspace]
  # The vCenter server IP to connet to
  server = "10.0.0.1"

  # The data center for the volume
  datacenter = "SDDC-Datacenter"

  # The default data store used for the volume
  default-datastore = "MyDefaultDatastore"

  # The resource pool path to use for your volumes
  resourcepool-path = "*/Resources/Compute-ResourcePool/kubernetes-clusters"
```

### Disk

The `Disk` section exists to define the SCSI controller type, which almost always should be set to `pvscsi`.

```bash
[Disk]
  # Defines the SCSI disk controller type, should almost always be set to pvscsi.
  scsicontrollertype = pvscsi
```

### Network

The Network section specifies the VM network that your cluster is running on. In the event that the kubelet
cannot discover it's own node addresses, it will query for IPs defined in the VM network.

```bash
[Network]
  # The network the VMs in your cluster are joined to
  # Worth noting that the "public" prefix in this field does not mean that the VM network
  # here must be public in any sense.
  public-network = "sddc-network-01"
```

### Labels

The Labels section defines the topology tags applied on VMs in order to apply Kubernetes zones and regions topology.
If set, Kubernetes will apply the labels `failure-domain.beta.kubernetes.io/zone` and `failure-domain.beta.kubernetes.io/region`
on your Nodes and PersistentVolumes based on the value of the tags specified here.

```bash
[Labels]
  # If set, the vSphere cloud provider will check if a VM has the tag with the corresponding value here.
  # If the tag exists, the region topology label `failure-domain.beta.kubernetes.io/region` with the associated value
  # will be applied to Nodes and PVs.
  region = k8s-region

  # If set, the vSphere cloud provider will check if a VM has the tag with the corresponding value here.
  # If the tag exists, the zones topology label `failure-domain.beta.kubernetes.io/zone` with the associated value
  # will be applied to Nodes and PVs.
  zone = k8s-zone
```

### Storing vCenter Credentials in a Kubernetes Secret

## FAQ

### Do all VMs in a cluster require vCenter credentials?

### What's the preferred way of storing vCenter credentials?
