# Disable node deletion by CPI

The [default behavior]((https://github.com/kubernetes/cloud-provider/blob/e820ef550efff2654f98d08b66e03094ccc0d6d7/controllers/nodelifecycle/node_lifecycle_controller.go#L155)) is that if the vSphere VM is no longer accessible/present according to the vCenter Server, the node VM will be deleted from the cloud provider. Specifically, [InstanceExistsByProviderID](https://github.com/kubernetes/cloud-provider-vsphere/blame/00587b422a0ef2b76e57233bca0e0e3b5380838e/pkg/cloudprovider/vsphere/instances.go#L164) should return `false, nil` when the VM on vSphere no longer exists. This cleans up Kubernetes node objects automatically in the event that a VM is deleted.

In this tutorial, we provide a way to disable deleting the node object of a terminated VM due to certain failure scenarios, e.g. when a VM' is not accessible by vCenter but the corresponding k8s-node is still running(network partition event).

Note that if you disable node deletion, when VMs on vSphere become inaccessible or not found, we will have leftover nodes and may introduce unexpected behaviors. Moreover, this behavior is not consistent with other cloud providers. The `SKIP_NODE_DELETION` flag is just a temporary one-off flag and we will need to re-evaluate if we want to change the current behavior.

## Option 1

Set env variable `SKIP_NODE_DELETION` for vsphere-cloud-controller-manager container:

```bash
    env:
    - name: SKIP_NODE_DELETION
      value: true
```

Example temporary environment variable setting procedure:

1. Add the environment variable with `kubectl set env daemonset vsphere-cloud-controller-manager -n kube-system SKIP_NODE_DELETION=true`.
You can check if the env variable has been applied correctly by running `kubectl describe daemonset vsphere-cloud-controller-manager -n kube-system`.

2. Terminate the running pod(s). The next pod created would pull in that environment variable.

3. Wait for pod to start.

4. View logs with `kubectl logs [POD_NAME] -n kube-system` and confirm everything healthy.

## Option 2

Another option is to manually modify the environment variable via `kubectl edit ds -n kube-system vsphere-cloud-controller-manager`. A new pod will be started and the old one is terminated after you save.

Sample YAML file can be found [here](https://github.com/kubernetes/cloud-provider-vsphere/blob/master/docs/book/tutorials/disable-node-deletion.yaml).

## Option 3

You can run this command in your running pod using `kubectl exec`, but this is only recommended for debugging purposes:

```bash
kubectl exec -it vsphere-cloud-controller-manager export SKIP_NODE_DELETION=true
```

Alternatively, you can get the Pod command line and change the variables in the runtime via `kubectl exec -it vsphere-cloud-controller-manager -- /bin/bash` and then run `export SKIP_NODE_DELETION=true`.
