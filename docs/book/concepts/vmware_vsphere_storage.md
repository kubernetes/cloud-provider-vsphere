# VMware vSphere Storage Concepts

vSphere provides a software-defined storage platform that integrates with block, file, and hyper-converged offerings such as [VMware vSAN](https://storagehub.vmware.com/t/vmware-vsan/). These storage offerings can be exposed as VMFS, NFS, vVols, or vSAN datastores.

vSphere has enterprise grade features, such as [Storage Policy Based Management (SPBM)](https://www.youtube.com/watch?v=e0wkMPDvKPQ), that enable users to define performance, availability, and redundancy levels requested by their business critical applications and ensure compliance with these requirements. vSphere provides high availability and redundancy at a compute and data level for all workloads.

A vSphere datastore is an abstraction that hides storage details, such as LUNs, and provides a uniform interface for storing persistent data. Datastores enable simplified storage management and data services for storage presented to vSphere. Depending on the backend storage, the datastores can be of one of the following types: vSAN, VMFS, NFS, and vVols. Volumes, or VMDKs, provisioned on top of the datastore are presented as block, or ReadWriteOnce, volumes to K8s pods.

* vSAN is a software-defined enterprise storage solution that supports hyper-converged infrastructure (HCI) systems. vSAN aggregates local or direct-attached storage devices to create a single storage pool shared across all hosts in a vSAN cluster.
* VMFS (Virtual Machine File System) is a cluster file system that allows virtualization to scale beyond a single node for multiple ESXi servers. VMFS increases resource utilization by providing multiple virtual machines with shared access to a pool of storage.
* NFS (Network File System) is a distributed file protocol that ESXi hosts use to communicate with NAS storage over TCP/IP. ESXi hosts can mount an NFS datastore and use it to store and boot virtual machines.
* vVols (Virtual Volumes) is an integration and management framework that virtualizes SAN and NAS arrays. It enables a more efficient operational model that is optimized for virtualized environments and centered on the application instead of the infrastructure.

Both in-tree and out-of-tree solutions from VMware allow Kubernetes Pods to use enterprise grade persistent storage.
