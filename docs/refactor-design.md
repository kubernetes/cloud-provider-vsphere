# vSphere Cloud Provider refactor design

As outlined in [KEP0002](https://github.com/kubernetes/community/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md), 
we want to remove any cloud provider specific logic from the kubernetes/kubernetes repo. We want to restructure the code to make it easy for any cloud provider to extend the kubernetes core in a consistent manner for their cloud.

## Current Status

Currently the in-tree vSphere cloud provider and the `vsphere_volume` volume driver are intertwined and the volume driver calls into the cloud provider code to perform the heavy lifting of volume management, also, the in-tree cloud provider assumes to be run on every kubernetes node and has specific code paths for master and worker nodes ([reference](https://github.com/kubernetes/kubernetes/blob/master/pkg/cloudprovider/providers/vsphere/vsphere.go#L221-L231)).

to recap:

- Cloud Provider code is run on every kubernetes node (in-tree, compiled into the kubernetes binaries)
- The `vsphere_volume` volume interface in kubernetes relies on cloud provider to perform volume management
- The heavy lifting for volume management is done by cloud provider

## Reimplement current functionalities with the Out-of-tree model

Currently the vSphere in-tree cloud provider implements two controller loops:

1. nodeController
2. volumeController

Given the ongoing effort to move volume management to out-of-tree plugin using the Container Storage Interface (CSI), KEP0002 favors the removal of the volume management code from controller-manager into a separate CSI plugin ([reference](https://github.com/kubernetes/community/blob/master/keps/sig-cloud-provider/0002-cloud-controller-manager.md#volume-management-changes)).

Specifically to vSphere, the decision is to follow the guideline reimplementing the `nodeController` loop in the cloud controller manager and move `volumeController` to a standalone CSI plugin called `vsphere-csi` that will be available in this same repository.

Both the CCM and the new `vsphere-csi` CSI plugin will run as pods on top of Kubernetes, with the least amount of privileges possible.

Testing the new CCM and CSI plugin end-to-end is also in-scope for this refactor, as currently the cloud provider is partially tested against `vcsim` and not against a real live system. Having a CI system that can test E2E and run conformance tests is a must and reporting conformance tests back to Kubernetes' Testgrid is a requirement for Kubernetes 1.12.

## Reimplementation of nodeController

The current `nodeController` implementation of cloud-provider-vsphere assumes that the code is run on both Masters and Workers and also assumes the ability to read local data on every node (like `/sys/class/dmi/id/product_serial`), as CCMs are only meant to be run on master nodes, a different approach has to be taken here, initially dns names will be used to reconciliate Kubernetes nodes with the underlying vSphere platform, until a more robust system will be implemented.

## Secrets sharing between out-of-tree CCM and vsphere-csi

As both CCM and `vsphere-csi` will need to connect to vSphere to perform their duties, credentials will be aligned between the two to provide one single central point of configuration, service configuration will be stored in a Kubernetes config map and presented as `vsphere.conf` inside the pod while credentials and other sensitive data will be stored in a Kubernetes secret object.

## Further developments

- Currently the `vsphere` provider in-tree does not support Availability Zones ([#64021](https://github.com/kubernetes/kubernetes/issues/64021)), this would be a nice-to-have addition for the new CCM and will treat it as a stretch goal for the refactor.

- Find a better way to reconciliate Kubernetes nodes with the underlying vSphere VMs.