# Enable Dual Stack feature

## Feature stage: Alpha

## Supported version: v1.22.3+

## Description

Since Cloud provider vsphere 1.22.3, we added support for Dual Stack. Now Node could have IP address with IPV4 and IPV6 address. Upstream K8s GA dual stack in 1.23. We will GA our support accordingly.

## Prerequests

* The VM should has IPV6 and IPV4 address

## Steps

We have a feature gate flag `ENABLE_ALPHA_DUAL_STACK` to enable this feature

* Set `ENABLE_ALPHA_DUAL_STACK: true` in Daemonset
* For yaml config, set `ipFamily: ipv6,ipv4` in cloud-config `vsphere.conf`
* For ini config, set `ip-family = ipv6,ipv4` in cloud-config `vsphere.conf`
* Apply the manifest.
