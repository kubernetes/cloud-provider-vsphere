# Deploying the vSphere CPI using release manifests

This document is designed to show you how to deploy the vSphere CPI using the release manifest YAMLs we provide.

CPI is releasing deployment YAML files per k8s release. You should be able to find the corresponding release manifest YAML under [this repo](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/releases)

Note that YAML files from [manifests/controller-manager repo](https://github.com/kubernetes/cloud-provider-vsphere/tree/master/manifests/controller-manager) is deprecated.

## Example workflow

In this tutorial, we will be installing the latest version of cloud provider vsphere(v1.27.0) freshly. If you have an older version of CPI already installed, the steps to deploy and upgrade CPI stay the same. With our `RollingUpdate` update strategy, after you update a DaemonSet template, old DaemonSet pods will be killed, and new DaemonSet pods will be created automatically.

### Step 1: find the kubernetes major version you are using

For example, the major version of '1.27.x' is '1.27', then run:

```bash
VERSION=1.27
wget https://raw.githubusercontent.com/kubernetes/cloud-provider-vsphere/release-$VERSION/releases/v$VERSION/vsphere-cloud-controller-manager.yaml
```

### Step 2: edit the Secret and ConfigMap inside 'vsphere-cloud-controller-manager.yaml'

In the release yaml files, what we provide is just an example configuration, you will need to update with real values based on your environment.

```bash
...
---
apiVersion: v1
kind: Secret
metadata:
  name: vsphere-cloud-secret
  labels:
    vsphere-cpi-infra: secret
    component: cloud-controller-manager
  namespace: kube-system
  # NOTE: this is just an example configuration, update with real values based on your environment
stringData:
  10.0.0.1.username: "<ENTER_YOUR_VCENTER_USERNAME>"
  10.0.0.1.password: "<ENTER_YOUR_VCENTER_PASSWORD>"
  1.2.3.4.username: "<ENTER_YOUR_VCENTER_USERNAME>"
  1.2.3.4.password: "<ENTER_YOUR_VCENTER_PASSWORD>"

  # NOTE: the following entries show an alternative format.
  # This format is amenable to IPv6 addresses. the server_{id}, username_{id},
  # and password_{id} require common id suffixes per server.
  server_prod:   fd00::1
  username_prod: "<ENTER_YOUR_VCENTER_USERNAME>"
  password_prod: "<ENTER_YOUR_VCENTER_PASSWORD>"
  server_test:   1.2.3.5
  username_test: "<ENTER_YOUR_VCENTER_USERNAME>"
  password_test: "<ENTER_YOUR_VCENTER_PASSWORD>"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: vsphere-cloud-config
  labels:
    vsphere-cpi-infra: config
    component: cloud-controller-manager
  namespace: kube-system
data:
  # NOTE: this is just an example configuration, update with real values based on your environment
  vsphere.conf: |
    # Global properties in this section will be used for all specified vCenters unless overriden in VirtualCenter section.
    global:
      port: 443
      # set insecureFlag to true if the vCenter uses a self-signed cert
      insecureFlag: true
      # settings for using k8s secret
      secretName: vsphere-cloud-secret
      secretNamespace: kube-system

    # vcenter section
    vcenter:
      your-vcenter-name-here:
        server: 10.0.0.1
        user: use-your-vcenter-user-here
        password: use-your-vcenter-password-here
        datacenters:
          - hrwest
          - hreast
      could-be-a-tenant-label:
        server: 1.2.3.4
        datacenters:
          - mytenantdc
        secretName: cpi-engineering-secret
        secretNamespace: kube-system

    # labels for regions and zones
    labels:
      region: k8s-region
      zone: k8s-zone
---
...
```

### Step 3: Now you can apply the release manifest (with updated values in Secret and ConfigMap)

```bash
kubectl apply -f vsphere-cloud-controller-manager.yaml
```

This will start to create Roles, Roles Bindings, Service Account, Service, Secret, ConfigMap and cloud-controller-manager Pod.

### Step 4: Cleanups

```bash
rm vsphere-cloud-controller-manager.yaml
```

For more information, please refer to [this doc](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/cloud_provider_interface.md).
