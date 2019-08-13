# In-Tree vs Out-of-Tree

In the early development stages of Kubernetes, implementing cloud providers natively (in-tree) was the most viable solution. Today, with many infrastructure providers supporting Kubernetes, new cloud providers are required to be out-of-tree in order to grow the project sustainably. For the existing in-tree cloud providers, there's an effort to extract/migrate clusters to use out-of-tree cloud providers. See [this KEP](https://github.com/kubernetes/enhancements/blob/master/keps/sig-cloud-provider/20190125-removing-in-tree-providers.md) for more details.

So what exactly is the difference between in-tree and out-of-tree? With in-tree, you simply deploy Kubernetes, and you can immediately begin to provision storage by simply setting the appropriate StorageClass that matches your underlying storage infrastructure. There is nothing additional to install. When it comes to placement decisions, Zones and Regions are already implemented so you can also start consuming the feature immediately.

Out-of-tree, on the other hand, means that there is no driver or provider installed in Kubernetes. Thus, in order to consume underlying infrastructure resources, or indeed understanding what the underlying infrastructure offers, you will need to install a CSI driver and a CPI driver before you are able to leverage the infrastructure.

Thus, the out-of-tree approach elongates the time to get Kubernetes up and running. However, because the CSI and CPI remove a bunch of tasks from core Kubernetes and put the onus of cloud, platform and storage providers, life cycle management of Kubernetes has been radically improved.
