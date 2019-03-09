#!/bin/sh

CLUSTER_NAME="${1}"

KUBECONFIG="$(kind get kubeconfig-path --name "${CLUSTER_NAME}")"
export KUBECONFIG

printf "waiting for vcsim..."
while ! kubectl -n kube-system get pods | \
        grep -q 'vcsim-0[[:space:]]\{0,\}1/1[[:space:]]\{0,\}Running'; do
  sleep 1
  printf "."
done
echo "ok"

for name in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do \
  ip4=$(docker exec "${name}" ip route get dev eth0 1 | awk '{print $NF;exit}')
  mac="$(docker exec "${name}" ip a | grep -F "${ip4}" -B 1 | head -n 1 | awk '{print $2}')"
  host_name="$(docker exec "${name}" hostname -s)"
  host_fqdn="$(docker exec "${name}" hostname -f)"
  serial="$(docker exec "${name}" cat /sys/class/dmi/id/product_serial)"
  serial="$(echo "${serial}" | tr '[:upper:]' '[:lower:]' | cut -c8- | tr -d ' -')"
  serial="$(echo "${serial}" | sed 's/^\([[:alnum:]]\{1,8\}\)\([[:alnum:]]\{1,4\}\)\([[:alnum:]]\{1,4\}\)\([[:alnum:]]\{1,4\}\)\([[:alnum:]]\{1,12\}\)$/\1-\2-\3-\4-\5/')"
  uuid="$(docker exec "${name}" cat /sys/class/dmi/id/product_uuid)"
  uuid="$(echo "${uuid}" | tr '[:upper:]' '[:lower:]')"
  printf 'creating vcsim vm:\n  name=%s\n  fqdn=%s\n  ipv4=%s\n   mac=%s\n  uuid=%s\n  suid=%s\n' \
    "${host_name}" "${host_fqdn}" "${ip4}" "${mac}" "${uuid}" "${serial}"
  kubectl -n kube-system exec vcsim-0 -- govc vm.create \
    -net.address "${mac}" "${host_name}"
  kubectl -n kube-system exec vcsim-0 -- govc vm.change \
    -vm "${host_name}" \
    -e "SET.config.uuid=${serial}" \
    -e "SET.summary.config.uuid=${serial}" \
    -e "SET.config.instanceUuid=${uuid}" \
    -e "SET.summary.config.instanceUuid=${uuid}" \
    -e "SET.guest.hostName=${host_fqdn}" \
    -e "SET.summary.guest.hostName=${host_fqdn}" \
    -e "SET.guest.ipAddress=${ip4}" \
    -e "SET.summary.guest.ipAddress=${ip4}"
done
