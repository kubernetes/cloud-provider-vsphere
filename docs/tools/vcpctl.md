# vSphere Cloud Provider CLI

This document is about vSphere Cloud Provider CLI (aka **vcpctl**), a command line interface tool for facilitating cloud controller manager (aka **CCM** ) provisioining, running command against CCM, and controling CCM.

## Provision Overview

Currently, at the first stage, `vcpctl` only support vpshere provisioning for kubernetes at Day 0. This provisioning requires a running vSphere. `vcpctl provision` command will perform a series of prerequisites, including:

1. Create a vSphere solution user, to be used with CCM
2. Create vSphere role with a minimal set of permissions.
3. Checking vms on vSphere for enabling UUID configure for Kubernetes.

### Install

`vcpctl` binary can be built directly from cloud-provider-vsphere repo. 

```bash
go build -o vcpctl ./cmd/vcpctl/main.go
cp ./vcpctl /usr/local/bin/vcpctl
```

### Syntax

Use the following syntax to run `vcpctl` commands from your terminal

```bash
vcpctl provision [flags]
```

`flags`: Specifies optional flags. For example, you can use the `--interactive=false` flags to enable an automation mode without interfaction input from command line. (TODO)

List of flags:

- `host` : Specify vCenter IP, for example: `https://<username>:<password>@<Host IP>/sdk`. Required. `<username>:<password>@` is optional
- `port` : Specify vCenter port
- `user` : Specify vCenter user. Required if host does not contain `<username>:<password>@`
- `password` : Specify vCenter password. Required if host does not contain `<username>:<password>@`
- `insecure` : Don't verify the server's certificate chain. Default is `false`. If you want to enable insecure mode, use `--insecure=true`
- `cert` : Certification for solution user. If you want to provide a certification, pass the file path of the certifcation to cert, like `--cert /path/to/cert.crt`. If no certification is provided, `vcpctl` will create a new one and store in default directory. (TODO)
- `role` : `vcpctl` can create roles during the provision. Role can be either `RegularUser` or `Administrator`. 

Note: vSphere SSO is required to be enabled by default. So SAML token has to be provided as `SSO_LOGIN_TOKEN`.

### Workflow

Run `vcpctl provision --host 'https://<host ip>/sdk' --insecure=true --user <username> --password <password>` in terminal after setting up `SSO_LOGIN_TOKEN`. The `vcpctl` will works as follows:

```bash
Perform cloud provider provisioning...
Create solution user...
Create default role with minimal permissions..
Checking vSphere Config on VMs...
```

1. It creates a solution user base on the certification provided by `--cert`
2. It creates a default role with name of `k8s-vcp-default`, and grants it with minimal permissions.
3. It checks the vm which is used for k8s cluster nodes, enabling uuid attribute.
