# Deploying the vSphere CPI using k3s

This document is designed to show you how to integrate k3s with cloud provider vsphere, which means that your k3s cluster will be able to talk to cloud provider API in order to make requests and configurations. In this tutorial, we will be running k3s with cloud provider vsphere.

When running with a cloud-controller-manager, it is expected to pass the node provider ID to a CCM as `<provider>://<id>`, in our case, `vsphere://1234567`. However, k3s passes it as `k3s://<hostname>`, which makes vsphere CCM not be able to find the node.

We only support `vsphere` as the provider name that is used for constructing **providerID** for both [vsphere](https://github.com/kubernetes/cloud-provider-vsphere/blob/v1.22.9/pkg/cloudprovider/vsphere/cloud.go#L51) and [vsphere-paravirtual](https://github.com/kubernetes/cloud-provider-vsphere/blob/v1.22.9/pkg/cloudprovider/vsphereparavirtual/cloud.go#L42).

## How to integrate k3s with cloud provider vsphere

### Preparation

We need to pass the following parameters when using k3s:

1. disable-cloud-controller

2. no-deploy servicelb

3. kubelet-arg="cloud-provider=external"

4. kubelet-arg="provider-id=vsphere://[master_node_id]"

First, k3s default cloud provider will add `k3s://` as provider and add node name as id.
`disable-cloud-controller` disables default k3s cloud controller and run your own.

Second, `no-deploy servicelb` asks k3s to not deploy servicelb because it
would mess up IP addresses - servicelb would overwrite Ingress IPs with nodes IPs.
We want to have cloud provider vsphere LoadBalancer IPs for service type LoadBalancer instead.

Third, we need to instruct kubelet that we will be using external cloud-provider
by setting `KUBELET_EXTRA_ARGS=--cloud-provider=external`.
Restart kubelet and we will be using external cloud-provider.

Last one, that is a requirement from vsphere CCM to pass this parameter to kubelet. This overwrites the node provider id by passing `--provider-id` flag to name with `vsphere://` format in kubelet in your node.

Example command:

```shell
curl -sfL https://get.k3s.io | sh -s — server \
  --disable-cloud-controller \
  --no-deploy servicelb \
  --kubelet-arg="cloud-provider=external" \
  --kubelet-arg="provider-id=vsphere://$master_node_id"
```

So far we configured only k3s master node, for any worker nodes you only need to install k3s with following parameters:

```shell
curl -sfL https://get.k3s.io | K3S_TOKEN=${token} sh -s - agent \
  --server https://${master_node_ip}:6443 \
  --kubelet-arg="cloud-provider=external" \
  --kubelet-arg="provider-id=vsphere://$worker_id"
```

### Install CCM

Now after k3s server starts we need to install the CCM itself. Simply apply the yaml manifest that matches the CCM version you are using, e.g. for v1.22.9:

```shell
kubectl apply -f releases/v1.22/
```

That’s it!
