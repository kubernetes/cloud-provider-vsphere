# vSphere Cloud-Controler-Manager Helm Chart

[vSphere Cloud Provider Interface](https://github.com/kubernetes/cloud-provider-vsphere) handles cloud specific functionality for VMware vSphere infrastructure running on Kubernetes.

## Introduction

This chart deploys all components required to run the external vSphere CPI as described on it's [GitHub page](https://github.com/kubernetes/cloud-provider-vsphere).

## Prerequisites

- Has been tested on Kubernetes 1.22.X+
- Assumes your Kubernetes cluster has been configured to use the external cloud provider. Please take a look at configuration guidelines located in the [Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/#running-cloud-controller-manager).

## Installing the Chart using Helm 3.0+

[The Github project of Helm chart repositories](https://github.com/helm/charts) is now an archive and no longer under active development since Nov 13, 2020. For more information, see the [Helm Charts Deprecation and Archive Notice](https://github.com/helm/charts#%EF%B8%8F-deprecation-and-archive-notice), and [Update](https://helm.sh/blog/charts-repo-deprecation/).

To add and update the Helm Charts for cloud-provider-vsphere, you can run the following command:

```bash
helm repo add vsphere-cpi https://kubernetes.github.io/cloud-provider-vsphere
helm repo update
```

See [help repo](https://helm.sh/docs/helm/helm_repo/) for command documentation.

Then to install the chart and by providing vCenter information/credentials, run the following command:

```bash
helm upgrade --install vsphere-cpi vsphere-cpi/vsphere-cpi --namespace kube-system --set config.enabled=true --set config.vcenter=<vCenter IP> --set config.username=<vCenter Username> --set config.password=<vCenter Password> --set config.datacenter=<vCenter Datacenter>
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example:

```bash
helm install vsphere-cpi -f values.yaml vsphere-cpi/vsphere-cpi
```

> **Tip**: You can use the default [values.yaml](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/charts/vsphere-cpi/values.yaml) as a guide

See [helm install](https://helm.sh/docs/helm/helm_install/) for command documentation.

> **Tip**: List all releases using `helm list --all`
> It may take a few minutes. Confirm the pods are up:
> `kubectl get pods --namespace $NAMESPACE`
> `helm list --namespace $NAMESPACE`

If you want to provide your own `vsphere.conf` and Kubernetes secret `vsphere-cpi` (for example, to handle multple datacenters/vCenters or for using zones), you can learn more about the `vsphere.conf` and `vsphere-cpi` secret by reading the following [documentation](https://cloud-provider-vsphere.sigs.k8s.io/tutorials/kubernetes-on-vsphere-with-kubeadm.html) and then running the following command:

```bash
helm install vsphere-cpi vsphere-cpi/vsphere-cpi --namespace kube-system
```

## Installing the Chart using Helm 2.X

To install this chart with the release name `vsphere-cpi` and by providing a vCenter information/credentials, run the following command:

```bash
helm install vsphere-cpi/vsphere-cpi --name vsphere-cpi --namespace kube-system --set config.enabled=true --set config.vcenter=<vCenter IP> --set config.username=<vCenter Username> --set config.password=<vCenter Password> --set config.datacenter=<vCenter Datacenter>
```

If you provide your own `vsphere.conf` and Kubernetes secret `vsphere-cpi`, then deploy the chart running the following command:

```bash
helm install vsphere-cpi/vsphere-cpi --name vsphere-cpi --namespace kube-system
```

## Uninstalling the Chart

Note: `helm delete` command has been renamed to `helm uninstall`.

To uninstall/delete the `vsphere-cpi` deployment:

```bash
# Helm 2
$ helm delete vsphere-cpi --namespace kube-system

# Helm 3
$ helm uninstall [RELEASE_NAME]
```

The command removes all the Kubernetes components associated with the chart and deletes the release.

See [helm uninstall](https://helm.sh/docs/helm/helm_uninstall/) for command documentation.

> **Tip**: To permanently remove the release using Helm v2.X, run `helm delete --purge vsphere-cpi --namespace kube-system`

## Configuration

The following table lists the configurable parameters of the vSphere CPI chart and their default values.

|             Parameter                    |            Description              |                  Default               |
|------------------------------------------|-------------------------------------|----------------------------------------|
| `podSecurityPolicy.enabled`              | Enable pod sec policy (k8s > 1.17)  |  true                                  |
| `podSecurityPolicy.annotations`          | Annotations for pd sec policy       |  nil                                   |
| `securityContext.enabled`                | Enable sec context for container    |  false                                 |
| `securityContext.runAsUser`              | RunAsUser. Default is `nobody` in   |  1001                                  |
|                                          |    distroless image                 |                                        |
| `securityContext.fsGroup`                | FsGroup. Default is `nobody` in     |  1001                                  |
|                                          |    distroless image                 |                                        |
| `config.enabled`                         | Create a simple single VC config    |  false                                 |
| `config.name`                            | Name of the created VC configmap    |  false                                 |
| `config.vcenter`                         | FQDN or IP of vCenter               |  vcenter.local                         |
| `config.username`                        | vCenter username                    |  user                                  |
| `config.password`                        | vCenter password                    |  pass                                  |
| `config.datacenter`                      | Datacenters within the vCenter      |  dc                                    |
| `config.secret.create`                   | Create secret for VC config         |  true                                  |
| `config.secret.name`                     | Name of the created VC secret       |  vsphere-cloud-secret                  |
| `rbac.create`                            | Create roles and role bindings      |  true                                  |
| `serviceAccount.create`                  | Create the service account          |  true                                  |
| `serviceAccount.name`                    | Name of the created service account |  cloud-controller-manager              |
| `daemonset.annotations`                  | Annotations for CPI pod             |  nil                                   |
| `daemonset.image`                        | Image for vSphere CPI               |  gcr.io/cloud-provider-vsphere/        |
|                                          |                                     |       vsphere-cloud-controller-manager |
| `daemonset.tag`                          | Tag for vSphere CPI                 |  latest                                |
| `daemonset.pullPolicy`                   | CPI image pullPolicy                |  IfNotPresent                          |
| `daemonset.dnsPolicy`                    | CPI dnsPolicy                       |  ClusterFirst                          |
| `daemonset.cmdline.logging`              | Logging level                       |  2                                     |
| `daemonset.cmdline.cloudConfig.dir`      | vSphere conf directory              |  /etc/cloud                            |
| `daemonset.cmdline.cloudConfig.file`     | vSphere conf filename               |  vsphere.conf                          |
| `daemonset.replicaCount`                 | Node resources                      | `[]`                                   |
| `daemonset.resources`                    | Node resources                      | `[]`                                   |
| `daemonset.podAnnotations`               | Annotations for CPI pod             |  nil                                   |
| `daemonset.podLabels`                    | Labels for CPI pod                  |  nil                                   |
| `daemonset.nodeSelector`                 | User-defined node selectors         |  nil                                   |
| `daemonset.tolerations`                  | User-defined tolerations            |  nil                                   |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` using Helm v3.X. For example,

```bash
$ helm install vsphere-cpi \
    stable/vsphere-cpi \
    --set daemonset.pullPolicy=Always
```

Alternatively, a YAML file that specifies the values for the parameters can be provided while installing the chart.

### Image tags

vSphere CPI offers a multitude of [tags](https://github.com/kubernetes/cloud-provider-vsphere/releases) for the various components used in this chart.

## Developer Releasing Guide

`values.yaml` files for the charts can be found in the `charts/vsphere-cpi` repo.

```bash
# Add and update helm repos
helm repo add vsphere-cpi https://kubernetes.github.io/cloud-provider-vsphere
helm repo update

# Package CPI Chart
VERSION=1.28.0
cd charts
helm package vsphere-cpi --version $VERSION --app-version $VERSION

# Debug by installing local helm manifest
helm upgrade --install vsphere-cpi vsphere-cpi --namespace kube-system --debug

# Update repo index
cd ..
helm repo index . --url https://kubernetes.github.io/cloud-provider-vsphere

# Need to modify the path to the github release path

# Push to master and gh-pages
git add .
git commit -m "...."
git push
git checkout gh-pages
git merge master
git push
```
