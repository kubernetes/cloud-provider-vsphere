# Known Issues

## VMTools Nice/Device Filtering

A number of [CNI](https://github.com/containernetworking/cni) implementations (such Calico, Antrea, and etc) introduce networking artifacts that interfere with the normal operation of vSphere's internal reporting for network/device interfaces. To address this issue, an `exclude-nics` filter for VMTools needs to be applied in order to prevent these artifacts from getting reported to vSphere and causing problems with network/device associations to vNICs on virtual machines.

The recommended `exclude-nics` filter is as follows for `/etc/vmware-tools/tools.conf`:

```bash
[guestinfo]
primary-nics=eth0
exclude-nics=antrea-*,cali*,cilium*,lxc*,ovs-system,br*,flannel*,veth*,docker*,virbr*,vxlan_sys_*,genev_sys_*,gre_sys_*,stt_sys_*,????????-??????
```

Each filter represents known CNI network/device interfaces. Most filters are straight foward, such as `docker*` for devices based on docker. Some filters, such as `????????-??????`, aren't so straight-forward as that filter identifies Antrea devices which get created per POD.

Restart VMTools for the changes to take effect.

```bash
/etc/vmware-tools/services.sh start
/etc/vmware-tools/services.sh stop
/etc/vmware-tools/services.sh restart
```
