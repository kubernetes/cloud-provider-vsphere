## To create configmap from vsphere.conf

kubectl create configmap cloud-config --from-file=vsphere.conf -n kube-system