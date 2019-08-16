# VMware vSphere Storage Concepts

vSphere has a proven Software Defined Storage (SDS) platform that integrates with block, file and hyper converged offerings such as [VMware vSAN](https://storagehub.vmware.com/t/vmware-vsan/). These storage offerings can be exposed as VMFS, NFS, vVOL or vSAN datastores.

vSphere has enterprise grade features like [Storage Policy Based Management (SPBM)](https://www.youtube.com/watch?v=e0wkMPDvKPQ) which enables customers to define performance, availability and redundancy levels requested by their business critical applications and ensure compliance with these SLAs. Sphere provides HA and redundancy at a compute and data level, out of the box for all workloads.

A vSphere datastore is an abstraction which hides storage details (such as LUNs) and provides a uniform interface for storing persistent data. Datastores enable simplified storage management and data services for storage presented to vSphere. Depending on the backend storage used, the datastores can be of the type vSAN, VMFS, NFS & vVOL. Volumes (VMDKs) provisioned on top of vSAN, VMFS, NFS and vVOL are all presented as block (ReadWriteOnce) volumes to K8s pods.

* vSAN is a hyper-converged infrastructure storage which provides excellent performance as well as reliability. vSAN is a simple storage system with management features like policy driven administration.
* VMFS (Virtual Machine File System) is a cluster file system that allows virtualization to scale beyond a single node for multiple VMware ESX servers. VMFS increases resource utilization by providing shared access to pool of storage.
* NFS (Network File System) is a distributed file protocol to access storage over network like local storage. vSphere supports NFS as backend to store virtual machines files.
* vVOL (Virtual Volumes) - Virtual Volumes datastore represents a storage container in vCenter Server and vSphere Web Client.

Both in-tree and out-of-tree solutions from VMware allows Kubernetes Pods to use enterprise grade persistent storage.
