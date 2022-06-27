# Minimal required privilege for vCenters

## Introduction

This article documents the minimal required permissions required for the vSphere user designated to the vSphere Cloud Provider.

Please refer [vSphere Documentation Center](https://docs.vmware.com/en/VMware-vSphere/6.5/com.vmware.vsphere.security.doc/GUID-18071E9A-EED1-4968-8D51-E0B4F526FDA3.html) to find out
how to create a `Custom Role`, `User`, and `Role Assignment`.

## Environment

* VMware vCenter Server 7.0.0 build-18369597
* vSphere Cloud Controller Manager version v1.18.1_vmware.1

## Required permission

The vSphere Cloud Controller Manager requires the Read permission on the parent entities of the node VMs such as folder, host, datacenter, datastore folder, datastore cluster, etc.
The role `ReadOnly: See details of objects, but not make changes` should be associated with the vSphere user for CPV.
