# NSX-T Load Balancer Controller

*Kubernetes load balancer support using NSX-T for the `vSphere` cloud controller
manager.*

This package enriches the cloud provider interface by implementing the load
balancing API of the cloud controller for an NSX-T environment.

**To activate the load balancer support, the environment variable
`ENABLE_ALPHA_NSXT_LB` must be set**.
Since this is an alpha feature, this implementation is a work in progress and the underlying implementation details can change.

If this feature gate is enabled the load balancer service must be configured
properly, also.

The basic assumption is that all nodes are bound to a logical tier1 router.
Here the load balancer service is attached to. Because there may be only
one such service here, the configuration of the service must be done
during the creation of the service. Here the selection of the (t-shirt) size is
required (S, M, L or XL).
For every service port a dedicated virtual server is managed which is
connected to this load balancer service.

## Features

The load balancer controller part of the vsphere cloud controller manager
is optional. If no `loadBalancer` or `loadBalancerClass` section is given
in the controller configuration loadbalancing support is disabled.
If enabled the follwing feature are supported:

### Tagging

All generated NSX-T elements are tagged with the app name of the controller,
the cluster name and the information from the service. Using this tagging it is
able to handle recovery of lost or dangling elements and garbage collection of
unused elements previously generated by the controller, even if the kubernetes
service object is already (accidentally) gone.

### Load Balancer Classes

This load balancer controller supports the usage of multiple load balancer
classes. Classes are preconfigured in the configuration file of the cloud
controller manager. There may be an arbitrary set of such classes in a dedicated
setup. Every class may use another `ipPool` configured in NSX-T.
This supports the creation of load balancers in different visibility realms,
for example an `*internet facing* or a *private* load balancer. The IPPools must
be preconfigured in NSX-T.
Additionally dedicated TCP and/or UDP profiles can be selected differing from
the default ones.

The class used to create a Kubernetes load balancer can then be selected on
the level of the Kubernetes service object.
To select a dedicated load balancer class different from the default one, the
Kubernetes service object must be annotated with the annotation:

```yaml
loadbalancer.vmware.io/class: <class name>
```

If no such annotation is given the default class will be used. This gives
the administrator of the cluster a chance to restrict the usage of the
NSXT-T resources for cluster users. They can determine which elements should
be used for a dedicated purpose. The cluster user just needs to know and select
the purpose by annotating the appropriate load balancer class.

### Health Checks

For TCP load balancers a health check will be generated.

## Configuration File

The controller manager requires dedicated entries in the cloud controller's
configuration file:

```yaml
loadBalancer:
  ipPoolName: pool1
  lbServiceId: 4711
  size: SMALL
  tcpAppProfileName: default-tcp-lb-app-profile
  udpAppProfileName: default-udp-lb-app-profile
  tags:
    tag1: value1
    tag2: value 2

loadBalancerClass:
  public:
    ipPoolName: poolPublic
  private":
    ipPoolName: poolPrivate

nsxt:
  user: admin
  password": secret
  host: nsxt-server
  insecureFlag: false
```

If the `loadBalancer` section or at least one `loadBalancerClass` section is
given, the load balancer support of the vSphere cloud controller manager is
enabled, otherwise it is disabled.

Only one of `ipPoolId` or `ipPoolName` may be given.
As the `lbServiceId` is given the controller is running in the *unmanaged*
mode.

The ``tcpAppProfileName`` and `udpAppProfileName` are used on creating
virtual servers. Alternatively `tcpAppProfilePath` and `udpAppProfilePath`
can be specified.

The `tags` field allows to specify additional tags which will be added
to all generated elements in NSX-T. The value must be a JSON object containing
the tags and string values.
The tag scope `owner` can be used to overwrite the owner name using the
controller's app name by default.

The `loadBalancer` section defines an implicit default load balancer class. This
load balancer class is used if the service does not specify a dedicated
load balancer class via annotation. Its values are also used as defaults
for all explicitly specified load balancer classes.

Additionaly classes may be configured by the `loadBalancerClass`
subsections.

### Managing Modes

There are two different modes the load balancer support can be used with:

- the *unmanaged* mode is used if the configuration specifies a load balancer
  service id. Here only the virtual servers are managed for the specified
  loadbalancer service.
  
- the *managed* mode manages the load balancer service, also. Here the tier1
  gateway must be specified, which is used for the segments the cluster
  nodes are connected to. The NSX-T load balancer service is only created if it
  is required. This saves resources if no kubernetes service of type
  `loadBalancer` is actually used.

Exactly one of the properties `lbServiceId` or `tier1GatewayPath` must be
specified if the load balancer support for the vSphere cloud controller
manager is enabled.

If the load balancer service should be managed by the controller (*managed* mode),
the `tier1GatewayPath` must be set (`lbServiceId` must not be set in this case):

```yaml
loadBalancer:
  ipPoolName: pool1
  tier1GatewayPath: /infra/tier-1s/12345
  size: SMALL
  tcpAppProfileName: default-tcp-lb-app-profile
  udpAppProfileName: default-udp-lb-app-profile
...
```

### Configuraton Option Reference

The load balancer configuration uses the sections `nsxt`, `loadBalancer` and
the subsections `loadBalancerClass`

#### Section NSX-T

The section NSX-T specifies the access to the NSX-T environment used to
provision the load balancers.
The following attributes are supported:

|Attribute|Meaning|
|---------|-------|
|`host`|NSXT-T host|
|`insecureFlag`|to be set to true if NSX-T uses locally signed cert without specifying a ca|
|`caFile`|certificate authority for the server certificate for locally signed certificates |
|`user`|user name (either password, access token or certificate based authentification must be specified)|
|`password`|password in clear text for password based authentification|
|`vmcAccessToken`|access token for token based authentification|
|`vmcAuthHost`|verification host for token based authentification|
|`clientAuthCertFile`|client certificate for the certificate based authorization|
|`clientAuthKeyFile`|private key for the client certificate|

#### Section loadBalancer

The load balancer section contains general settings and default settings for
the load balancer classes. The following attributes are supported:

|Attribute|Meaning|
|---------|-------|
|`size`|Size of load balancer service (`SMALL`,`MEDIUM`,`LARGE`,`XLARGE`)|
|`lbServiceId`|service id of the load balancer service to use (for unmanaged mode)|
|`tier1GatewayPath`|policy path for the tier1 gateway|
|`snatDisabled`|Set to true if want to preserve client IP (for inline mode)|
|`tags`|JSON map with name/value pairs used for creating additional tags for the generated NSX-T elements|

If the tag key `owner` is given it overwrites the default owner
(application name of the cloud controller manager). The owner is used together
with the cluster name (specified with the option `--cluster-name`) to identify
dangling elements in the infrastructure originating from this controller manager.
If the cluster name option is not given, there will be no automated cleanup of
dangling elements.

Additionally the attributes of a `loadBalancerClass` can be specified here. These
values are used as defaults for configured load balancer classes.
If no explicit default load balancer class (with name `default`) is configured,
these settings are used for the default load balancer class.

The default load balancer class settings are always used if the kubernetes
service object does not explicitly specify a load balancer class by using the
annotation `loadbalancer.vmware.io/class`.

#### Subsections loadBalancerClass

The name of the subsection is used as name for the load balancer class to configure.
A load balancer class configuration uses the following attributes:

|Attribute|Meaning|
|---------|-------|
|`ipPoolName`| name of the ip pool used for the virtual servers (either `ipPoolName` or `ipPoolID` must be specified)|
|`ipPoolID`| id of the ip pool |
|`tcpAppProfileName`| name of application profile used for TCP connections (either `tcpAppProfileName` or `tcpAppProfileID` must be specified)|
|`tcpAppProfileID`| id of application profile used for TCP connections|
|`udpAppProfileName`| name of application profile used for UDP connections (either `udpAppProfileName` or `udpAppProfileID` must be specified)|
|`udpAppProfileID`| id of application profile used for UDP connections|

If a name/id pair is missing completely it will be defaulted by the settings from the `loadBalancer` section.
If there no value is specified, also, the configuration is invalid.
If a name is specified instead of an id there *MUST* not be multiple such elements with the same name, even this
is possible in NSX-T.
