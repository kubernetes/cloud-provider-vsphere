# E2E test for cloud-provider-vsphere

## Requirements

In order to perform e2e tests against the cloud-provider-vsphere, make sure

* you have administrative access to a vSphere server
* golang 1.16+
* Docker ([download](https://www.docker.com/get-started))

## Environment variables

The first step to running the e2e tests is setting up the required environment variables in the [e2e config file](./config/vsphere-dev.yaml):

| Environment variable       | Description                                                                                           | Example                                                                          |
| -------------------------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| `VSPHERE_SERVER`              | The IP address or FQDN of a vCenter 6.7u3 server   (required)                                      | `my.vcenter.com`                                                                 |
| `VSPHERE_USERNAME`            | The username used to access the vSphere server     (required)                                      | `my-username`                                                                    |
| `VSPHERE_PASSWORD`            | The password used to access the vSphere server      (required)                                     | `my-password`                                                                    |
| `VSPHERE_DATACENTER`          | The unique name or inventory path of the datacenter in which VMs will be created                      | `my-datacenter` or `/my-datacenter`                                              |
| `VSPHERE_FOLDER`              | The unique name or inventory path of the folder in which VMs will be created                          | `my-folder` or `/my-datacenter/vm/my-folder`                                     |
| `VSPHERE_RESOURCE_POOL`       | The unique name or inventory path of the resource pool in which VMs will be created                   | `my-resource-pool` or `/my-datacenter/host/Cluster-1/Resources/my-resource-pool` |
| `VSPHERE_DATASTORE`           | The unique name or inventory path of the datastore in which VMs will be created                       | `my-datastore` or `/my-datacenter/datstore/my-datastore`                         |
| `VSPHERE_NETWORK`             | The unique name or inventory path of the network to which VMs will be connected                       | `my-network` or `/my-datacenter/network/my-network`                              |
| `VSPHERE_HAPROXY_TEMPLATE`    | The unique name or inventory path of the template from which the HAProxy load balancer VMs are cloned | `my-haproxy-template` or `/my-datacenter/vm/my-haproxy-template`                 |
| `VSPHERE_SSH_PRIVATE_KEY`     | The file path of the private key used to ssh into the CAPV VMs                                        | `/home/foo/bar-ssh.key`                                                          |
| `VSPHERE_SSH_AUTHORIZED_KEY`  | The public key that is added to the CAPV VMs                                                          | `ss-rsa ABCDEF...XYZ=`                                                          |

## Running the e2e tests

Checkout the e2e directory `PROJECT_ROOT/test/e2e` and run it with `make`.

Or run `make test-e2e` under the `PROJECT_ROOT`.
