# In-Tree vs Out-of-Tree

Originally, Kubernetes implemented cloud provider-specific functionalities natively within the main Kubernetes tree, or as in-tree modules. However, with many infrastructure providers supporting Kubernetes, the old method is no longer advised. New providers that support Kubernetes must follow the out-of-tree model. For the existing in-tree cloud providers, Kubernetes offers a program of migration to the out-of-tree architecture and removal of all cloud provider specific code from the repository. See the [Removing In-Tree Cloud Provider Code](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20190125-removing-in-tree-providers.md) proposal for more details.

So what exactly is the difference between in-tree and out-of-tree? With in-tree, you simply deploy Kubernetes, and you can immediately begin to provision storage by simply setting the appropriate StorageClass that matches your underlying storage infrastructure. There is nothing additional to install. When it comes to placement decisions, Zones and Regions are already implemented so you can also start consuming the feature immediately.

Out-of-tree, on the other hand, means that there is no driver or provider installed in Kubernetes. Thus, in order to consume underlying infrastructure resources, or indeed understanding what the underlying infrastructure offers, you will need to install a CSI driver and a CPI driver before you are able to leverage the infrastructure.

Thus, the out-of-tree approach elongates the time to get Kubernetes up and running. However, because the CSI and CPI remove a bunch of tasks from core Kubernetes and put the onus of cloud, platform and storage providers, life cycle management of Kubernetes has been radically improved.
