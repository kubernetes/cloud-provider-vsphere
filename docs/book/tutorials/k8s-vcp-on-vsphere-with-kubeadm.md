# Running a Kubernetes Cluster on vSphere with kubeadm

## Prerequisites

### OS and VMs

It is assumed that you have Ubuntu 18.04 LTS VMs set up as a template and cloned from to act as base images for your K8s cluster, if you would like guidance on how to do this, please [see here](https://blah.cloud/kubernetes/creating-an-ubuntu-18-04-lts-cloud-image-for-cloning-on-vmware/)

In the setup guide below we set up a single master and multiple worker nodes.

It is also assumed you have SSH access to all nodes in order to run the commands on in the following guide.

### Tools

We are using macOS here, so the `brew` package manager is used to install and manage the tools, if you are using Linux or Windows, use the appropriate install guide for each tool, according to your OS.

For each tool listed below is the `brew` install command and the link to the install instructions for other OSes.

* brew
  * [https://brew.sh](https://brew.sh)
* govc - `brew tap govmomi/tap/govc && brew install govmomi/tap/govc`
  * [https://github.com/vmware/govmomi/tree/master/govc](https://github.com/vmware/govmomi/tree/master/govc)
* kubectl - `brew install kubernetes-cli`
  * [https://kubernetes.io/docs/tasks/tools/install-kubectl/](https://kubernetes.io/docs/tasks/tools/install-kubectl/)
* tmux (optional) - `brew install tmux`
  * [https://github.com/tmux/tmux](https://github.com/tmux/tmux)

## Setting up VMs with K8s components

### On all nodes (part a)

Install the container runtime (in our case Docker)

```sh
# Install Docker CE
# Update the apt package index
sudo apt update

## Install packages to allow apt to use a repository over HTTPS
sudo apt install ca-certificates software-properties-common apt-transport-https curl -y

## Add Docker’s official GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -

## Add docker apt repository.
sudo add-apt-repository \
"deb [arch=amd64] https://download.docker.com/linux/ubuntu \
$(lsb_release -cs) \
stable"

# Install docker ce (latest supported for K8s 1.14 is Docker 18.09)
sudo apt update && sudo apt install docker-ce=5:18.09.6~3-0~ubuntu-bionic -y

# Setup daemon parameters, like log rotation and cgroups
sudo tee /etc/docker/daemon.json >/dev/null <<EOF
{
  "exec-opts": ["native.cgroupdriver=systemd"],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m"
  },
  "storage-driver": "overlay2"
}
EOF

sudo mkdir -p /etc/systemd/system/docker.service.d

# Restart docker.
sudo systemctl daemon-reload
sudo systemctl restart docker
```

Install the K8s components

```sh
# Add the K8s repo to apt
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -
echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee /etc/apt/sources.list.d/kubernetes.list >/dev/null

# Install kubelet, kubectl and kubeadm for cluster spinup
sudo apt update
sudo apt install kubelet kubeadm kubectl -y

# Hold K8s packages at their installed version so as not to upgrade unexpectedly on an apt upgrade
sudo apt-mark hold kubelet kubeadm kubectl
```

We will be using [`flannel`](https://github.com/coreos/flannel) for pod networking in this example, so the below needs to be run on all nodes to pass [bridged IPv4 traffic to iptables chains](https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/#tabs-pod-install-4):

```sh
sudo sysctl net.bridge.bridge-nf-call-iptables=1
```

## Enabling the VMware vSphere Cloud Provider

### On the master node(s) (part a)

#### Create your `vsphere.conf` file with vCenter details

For reference, the vCenter configuration looks like below (you can correlate the values in the UI to the values in the `vsphere.conf` file below):

![vCenter](/docs/images/vCenter.png)

Edit the below command to fill in your vCenter details before running.

If you don't have a folder created with your kubernetes node VMs added we can do that quickly with `govc` (note, change `vSAN-DC` to your Datacenter name in vCenter):

*N.B: The below command assumes your K8s VMs are named in the pattern `k8s-*`, if they have a different naming schema, please adjust the command below appropriately.*

```sh
govc folder.create /vSAN-DC/vm/k8s
govc object.mv /vSAN-DC/vm/k8s-\* /vSAN-DC/vm/k8s
```

Details on [syntax for the below vsphere.conf file can be found here](https://vmware.github.io/vsphere-storage-for-kubernetes/documentation/existing.html). It is important to note, whatever `VM folder` you specify below needs to be pre-created in your vCenter, in my case the folder is called `k8s`.

```sh
sudo tee /etc/kubernetes/vsphere.conf >/dev/null <<EOF
[Global]
user = "administrator@vsphere.local"
password = "Admin!23"
port = "443"
insecure-flag = "1"

[VirtualCenter "10.198.17.154"]
datacenters = "vSAN-DC"

[Workspace]
server = "10.198.17.154"
datacenter = "vSAN-DC"
default-datastore = "vsanDatastore"
resourcepool-path = "vSAN-Cluster/Resources"
folder = "k8s"

[Disk]
scsicontrollertype = pvscsi

[Network]
public-network = "VM Network"
EOF
```

Activate the vSphere Cloud Provider in the `kubeadm init` config file. Additionally, as we are deploying `flannel` as our overlay network for pods and it requires the below subnet CIDR in order for the overlay to work (do not change this if you intend to use `flannel`).

```yaml
sudo tee /etc/kubernetes/kubeadminitmaster.yaml >/dev/null <<EOF
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
bootstrapTokens:
       - groups:
         - system:bootstrappers:kubeadm:default-node-token
         token: y7yaev.9dvwxx6ny4ef8vlq
         ttl: 0s
         usages:
         - signing
         - authentication
nodeRegistration:
  kubeletExtraArgs:
    cloud-provider: "vsphere"
    cloud-config: "/etc/kubernetes/vsphere.conf"
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
kubernetesVersion: v1.14.2
apiServer:
  extraArgs:
    cloud-provider: "vsphere"
    cloud-config: "/etc/kubernetes/vsphere.conf"
  extraVolumes:
  - name: cloud
    hostPath: "/etc/kubernetes/vsphere.conf"
    mountPath: "/etc/kubernetes/vsphere.conf"
controllerManager:
  extraArgs:
    cloud-provider: "vsphere"
    cloud-config: "/etc/kubernetes/vsphere.conf"
  extraVolumes:
  - name: cloud
    hostPath: "/etc/kubernetes/vsphere.conf"
    mountPath: "/etc/kubernetes/vsphere.conf"
networking:
  podSubnet: "10.244.0.0/16"
EOF
```

Restart the kubelet daemon to reload the configuration

```sh
sudo systemctl daemon-reload
sudo systemctl restart kubelet
```

## Initialising the cluster with kubeadm

### On all nodes (part b)

Firstly, verify that connectivity to the required `gcr.io` registries is working by pulling the containers required by `kubeadm`

```sh
$ sudo kubeadm config images pull
[config/images] Pulled k8s.gcr.io/kube-apiserver:v1.14.2
[config/images] Pulled k8s.gcr.io/kube-controller-manager:v1.14.2
[config/images] Pulled k8s.gcr.io/kube-scheduler:v1.14.2
[config/images] Pulled k8s.gcr.io/kube-proxy:v1.14.2
[config/images] Pulled k8s.gcr.io/pause:3.1
[config/images] Pulled k8s.gcr.io/etcd:3.3.10
[config/images] Pulled k8s.gcr.io/coredns:1.3.1
```

### On the master node(s) (part b)

Initialise `kubeadm` with the config file from above which includes our vSphere Cloud Provider and Flannel CIDR configurations.

```sh
$ sudo kubeadm init --config /etc/kubernetes/kubeadminitmaster.yaml
[init] Using Kubernetes version: v1.14.2
[preflight] Running pre-flight checks
[preflight] Pulling images required for setting up a Kubernetes cluster
[preflight] This might take a minute or two, depending on the speed of your internet connection
[preflight] You can also perform this action in beforehand using 'kubeadm config images pull'
[kubelet-start] Writing kubelet environment file with flags to file "/var/lib/kubelet/kubeadm-flags.env"
[kubelet-start] Writing kubelet configuration to file "/var/lib/kubelet/config.yaml"
[kubelet-start] Activating the kubelet service
[certs] Using certificateDir folder "/etc/kubernetes/pki"
[certs] Generating "ca" certificate and key
[certs] Generating "apiserver" certificate and key
[certs] apiserver serving cert is signed for DNS names [k8s-master kubernetes kubernetes.default kubernetes.default.svc kubernetes.default.svc.cluster.local] and IPs [10.96.0.1 10.198.26.169]
[certs] Generating "apiserver-kubelet-client" certificate and key
[certs] Generating "etcd/ca" certificate and key
[certs] Generating "etcd/peer" certificate and key
[certs] etcd/peer serving cert is signed for DNS names [k8s-master localhost] and IPs [10.198.26.169 127.0.0.1 ::1]
[certs] Generating "etcd/healthcheck-client" certificate and key
[certs] Generating "etcd/server" certificate and key
[certs] etcd/server serving cert is signed for DNS names [k8s-master localhost] and IPs [10.198.26.169 127.0.0.1 ::1]
[certs] Generating "apiserver-etcd-client" certificate and key
[certs] Generating "front-proxy-ca" certificate and key
[certs] Generating "front-proxy-client" certificate and key
[certs] Generating "sa" key and public key
[kubeconfig] Using kubeconfig folder "/etc/kubernetes"
[kubeconfig] Writing "admin.conf" kubeconfig file
[kubeconfig] Writing "kubelet.conf" kubeconfig file
[kubeconfig] Writing "controller-manager.conf" kubeconfig file
[kubeconfig] Writing "scheduler.conf" kubeconfig file
[control-plane] Using manifest folder "/etc/kubernetes/manifests"
[control-plane] Creating static Pod manifest for "kube-apiserver"
[controlplane] Adding extra host path mount "cloud" to "kube-apiserver"
[controlplane] Adding extra host path mount "cloud" to "kube-controller-manager"
[control-plane] Creating static Pod manifest for "kube-controller-manager"
[controlplane] Adding extra host path mount "cloud" to "kube-apiserver"
[controlplane] Adding extra host path mount "cloud" to "kube-controller-manager"
[control-plane] Creating static Pod manifest for "kube-scheduler"
[controlplane] Adding extra host path mount "cloud" to "kube-apiserver"
[controlplane] Adding extra host path mount "cloud" to "kube-controller-manager"
[etcd] Creating static Pod manifest for local etcd in "/etc/kubernetes/manifests"
[wait-control-plane] Waiting for the kubelet to boot up the control plane as static Pods from directory "/etc/kubernetes/manifests". This can take up to 4m0s
[apiclient] All control plane components are healthy after 17.503719 seconds
[upload-config] storing the configuration used in ConfigMap "kubeadm-config" in the "kube-system" Namespace
[kubelet] Creating a ConfigMap "kubelet-config-1.14" in namespace kube-system with the configuration for the kubelets in the cluster
[upload-certs] Skipping phase. Please see --experimental-upload-certs
[mark-control-plane] Marking the node k8s-master as control-plane by adding the label "node-role.kubernetes.io/master=''"
[mark-control-plane] Marking the node k8s-master as control-plane by adding the taints [node-role.kubernetes.io/master:NoSchedule]
[bootstrap-token] Using token: y7yaev.9dvwxx6ny4ef8vlq
[bootstrap-token] Configuring bootstrap tokens, cluster-info ConfigMap, RBAC Roles
[bootstrap-token] configured RBAC rules to allow Node Bootstrap tokens to post CSRs in order for nodes to get long term certificate credentials
[bootstrap-token] configured RBAC rules to allow the csrapprover controller automatically approve CSRs from a Node Bootstrap Token
[bootstrap-token] configured RBAC rules to allow certificate rotation for all node client certificates in the cluster
[bootstrap-token] creating the "cluster-info" ConfigMap in the "kube-public" namespace
[addons] Applied essential addon: CoreDNS
[addons] Applied essential addon: kube-proxy

Your Kubernetes control-plane has initialized successfully!

To start using your cluster, you need to run the following as a regular user:

  mkdir -p $HOME/.kube
  sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
  sudo chown $(id -u):$(id -g) $HOME/.kube/config

You should now deploy a pod network to the cluster.
Run "kubectl apply -f [podnetwork].yaml" with one of the options listed at:
  https://kubernetes.io/docs/concepts/cluster-administration/addons/

Then you can join any number of worker nodes by running the following on each as root:

kubeadm join 10.198.26.169:6443 --token y7yaev.9dvwxx6ny4ef8vlq \
    --discovery-token-ca-cert-hash sha256:fd7bcdb33570d477d3c7b1c9220a7f98661d3435c8d2380a0701849f8b5be099
```

A lot of text will output as it spins up the cluster components, if all is successful, we can start using the cluster now by importing the `kubeconfig`.

```sh
mkdir -p $HOME/.kube
sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config
sudo chown $(id -u):$(id -g) $HOME/.kube/config
```

You can also use it on external systems by copying the output from the below command into your local computer's `~/.kube/config` file:

```sh
sudo cat /etc/kubernetes/admin.conf
```

Let's deploy our `flannel` pod overlay networking so the pods can communicate with each other.

```sh
kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/62e44c867a2846fefb68bd5f178daf4da3095ccb/Documentation/kube-flannel.yml
```

Check to make sure the pods are all in the status `Running`:

```sh
$ kubectl get pods --all-namespaces
NAMESPACE     NAME                                 READY   STATUS    RESTARTS   AGE
kube-system   coredns-86c58d9df4-fqbdm             1/1     Running   0          2m19s
kube-system   coredns-86c58d9df4-zhpj6             1/1     Running   0          2m19s
kube-system   etcd-k8s-master                      1/1     Running   0          2m37s
kube-system   kube-apiserver-k8s-master            1/1     Running   0          68s
kube-system   kube-controller-manager-k8s-master   1/1     Running   0          2m36s
kube-system   kube-flannel-ds-amd64-8cst6          1/1     Running   0          26s
kube-system   kube-proxy-6grkv                     1/1     Running   0          2m19s
kube-system   kube-scheduler-k8s-master            1/1     Running   0          2m36s
```

Export the master node config used to point the workers being joined to the master:

```sh
kubectl -n kube-public get configmap cluster-info -o jsonpath='{.data.kubeconfig}' > discovery.yaml
```

### On your laptop

Copy the `discovery.yaml` to your local machine with `scp`.

```sh
scp ubuntu@10.198.17.177:~/discovery.yaml discovery.yaml
```

Then upload it to the worker nodes.

```sh
scp discovery.yaml ubuntu@10.198.17.189:~/discovery.yaml
scp discovery.yaml ubuntu@10.198.17.190:~/discovery.yaml
scp discovery.yaml ubuntu@10.198.17.191:~/discovery.yaml
```

### On all the worker nodes

To check and make sure the `discovery.yaml` file was copied correctly, do a quick `cat`.

```sh
cat ~/discovery.yaml
```

Then create the worker node `kubeadm` config yaml file (notice it's using our `discovery.yaml` as the input for master discovery) and the `token` is the same as we put in the master `kubeadminitmaster.yaml` configuration above and we specify the `cloud-provider` as `vsphere` for the workers:

```yaml
sudo tee /etc/kubernetes/kubeadminitworker.yaml >/dev/null <<EOF
apiVersion: kubeadm.k8s.io/v1beta1
caCertPath: /etc/kubernetes/pki/ca.crt
discovery:
  file:
    kubeConfigPath: discovery.yaml
  timeout: 5m0s
  tlsBootstrapToken: y7yaev.9dvwxx6ny4ef8vlq
kind: JoinConfiguration
nodeRegistration:
  criSocket: /var/run/dockershim.sock
  kubeletExtraArgs:
    cloud-provider: vsphere
EOF
```

And now we should be able to join our workers to the cluster.

```sh
$ sudo kubeadm join --config /etc/kubernetes/kubeadminitworker.yaml
[preflight] Running pre-flight checks
[preflight] Reading configuration from the cluster...
[preflight] FYI: You can look at this config file with 'kubectl -n kube-system get cm kubeadm-config -oyaml'
[kubelet-start] Downloading configuration for the kubelet from the "kubelet-config-1.14" ConfigMap in the kube-system namespace
[kubelet-start] Writing kubelet configuration to file "/var/lib/kubelet/config.yaml"
[kubelet-start] Writing kubelet environment file with flags to file "/var/lib/kubelet/kubeadm-flags.env"
[kubelet-start] Activating the kubelet service
[kubelet-start] Waiting for the kubelet to perform the TLS Bootstrap...

This node has joined the cluster:
* Certificate signing request was sent to apiserver and a response was received.
* The Kubelet was informed of the new secure connection details.

Run 'kubectl get nodes' on the control-plane to see this node join the cluster.
```

## Verify setup

Now, as the output says above, back on the master check that all nodes have joined the cluster

```sh
ubuntu@k8s-master:~$ kubectl get nodes -o wide
NAME          STATUS   ROLES    AGE   VERSION   INTERNAL-IP     EXTERNAL-IP     OS-IMAGE             KERNEL-VERSION      CONTAINER-RUNTIME
k8s-master    Ready    master   22h   v1.14.2   10.198.25.157   10.198.25.157   Ubuntu 18.04.2 LTS   4.15.0-51-generic   docker://18.9.6
k8s-worker1   Ready    <none>   22h   v1.14.2   10.198.25.158   10.198.25.158   Ubuntu 18.04.2 LTS   4.15.0-51-generic   docker://18.9.6
k8s-worker2   Ready    <none>   22h   v1.14.2   10.198.25.172   10.198.25.172   Ubuntu 18.04.2 LTS   4.15.0-51-generic   docker://18.9.6
k8s-worker3   Ready    <none>   22h   v1.14.2   10.198.25.173   10.198.25.173   Ubuntu 18.04.2 LTS   4.15.0-51-generic   docker://18.9.6
```

Verify the `providerID` is set on all the nodes for the VCP to operate correctly:

```sh
ubuntu@k8s-master:~$ kubectl describe nodes | grep "ProviderID"
ProviderID:                  vsphere://420f0d85-cf4a-c7a7-e52d-18e9b4b71dec
ProviderID:                  vsphere://420fc2b2-64ab-a477-f7b1-37d4e6747abf
ProviderID:                  vsphere://420f2d75-37bd-8b56-4e2f-421cbcbbb0b2
ProviderID:                  vsphere://420f7ec3-2dbd-601e-240b-4ee6d8945210
```

You now have a fully up and running k8s cluster with the vSphere Cloud Provider installed, please try creating a StorageClass using an SPBM policy as documented [here](https://vmware.github.io/vsphere-storage-for-kubernetes/documentation/policy-based-mgmt.html#create-a-storageclass) and deploy some applications!
