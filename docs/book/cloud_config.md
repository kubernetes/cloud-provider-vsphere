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
  soap-roundtrip-count = ""
  secret-name = ""
  secret-namespace = ""
  ip-family = "ipv4"

[VirtualCenter "10.0.0.1"]
  user = "viadmin@vmare.local"
  password = "my-secure-password"
  port = "443"
  datacenters = "SDDC-Datacenter"
  soap-roundtrip-count = "1"
  thumbprint = ""
  secret-name = ""
  secret-namespace = ""

[Labels]
  region = k8s-region
  zone = k8s-zone

[Nodes]
  internal-network-subnet-cidr = "192.0.2.0/24"
  external-network-subnet-cidr = "198.51.100.0/24"
  internal-vm-network-name = "Internal K8s Traffic"
  external-vm-network-name = "External/Outbound Traffic"
  exclude-internal-network-subnet-cidr = "192.0.2.0/24,fe80::1/128"
  exclude-external-network-subnet-cidr = "192.1.2.0/24,fe80::2/128"
```

There are 4 sections in the cloud config file, let's break down the fields in each section:

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

  # SOAP round trip counter
  soap-roundtrip-count = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the name of the secret resource
  secret-name = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the namespace of the secret resource
  secret-namespace = ""

  # IP Family enables the ability to support IPv4 or IPv6
  # Supported values are:
  # ipv4 - IPv4 addresses only (Default)
  # ipv6 - IPv6 addresses only
  IPFamily string `gcfg:"ip-family"`
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

  # The CA file to be trusted when connecting to vCenter.
  # If not set, defaults to the thumbprint specified in the Global section
  ca-file = "/etc/kubernetes/vcenter-ca.crt"

  # The vCenter certificate thumbprint, this ensures the correct certificate is used
  # If not set, defaults to the thumbprint specified in the Global section
  thumbprint = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the name of the secret resource
  # If not set, defaults to the thumbprint specified in the Global section
  secret-name = ""

  # You can optionally store vCenter credentials in a Kubernetes secret
  # This field specifies the namespace of the secret resource
  # If not set, defaults to the thumbprint specified in the Global section
  secret-namespace = ""

  # IP Family enables the ability to support IPv4 or IPv6
  # Supported values are:
  # ipv4 - IPv4 addresses only (Default)
  # ipv6 - IPv6 addresses only
  # If not set, defaults to the thumbprint specified in the Global section
  IPFamily string `gcfg:"ip-family"`
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

### Nodes

The Nodes section defines the way that the Node IPs are selected from the
addresses assigned to the Node in kube-api.

Addresses in the optional `exclude-internal-network-subnet-cidr` and optional
`exclude-external-network-subnet-cidr` are removed from consideration for any
match before any selection happens.

If provided, the `internal-network-subnet-cidr` and
`external-network-subnet-cidr` matching will be attempted first. Addresses that
fall within each of the provided CIDRs will be selected.

If provided, and the subnet matching method does not select a matching address,
the `internal-vm-network-name` and `external-vm-network-name` matching will be
attempted. Addresses belonging to networks that match the name in vSphere will
be selected.

If these methods are unsuccessful at selecting an address, or if these other
configurations were not provided, default selection will select the first
address that is not a Localhost address.

```bash
[Nodes]
  # If set, the vSphere cloud provider will select the first address that falls
  # within the provided subnet and assign that value to the Internal network for
  # the node.
  internal-network-subnet-cidr = "192.0.2.0/24"

  # If set, the vSphere cloud provider will select the first address that falls
  # within the provided subnet and assign that value to the External network for
  # the node.
  external-network-subnet-cidr = "198.51.100.0/24"

  # If set, the vSphere cloud provider will select the first address found in
  # the VM network matching the provided name and assign that value to the
  # Internal network for the node.
  internal-vm-network-name = "Internal K8s Traffic"

  # If set, the vSphere cloud provider will select the first address found in
  # the VM network matching the provided name and assign that value to the
  # External network for the node.
  external-vm-network-name = "External/Outbound Traffic"

  # If set, the vSphere cloud provider will never select addresses for the
  # Internal network that fall within the provided subnet ranges. This
  # configuration has the highest precedence. See notes above for details.
  exclude-internal-network-subnet-cidr = "192.0.2.0/24,fe80::1/128"

  # If set, the vSphere cloud provider will never select addresses for the
  # External network that fall within the provided subnet ranges. This
  # configuration has the highest precedence. See notes above for details.
  exclude-external-network-subnet-cidr = "192.1.2.0/24,fe80::2/128"
```

### Storing vCenter Credentials in a Kubernetes Secret

## FAQ

### Do all VMs in a cluster require vCenter credentials?

### What's the preferred way of storing vCenter credentials?
