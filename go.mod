module k8s.io/cloud-provider-vsphere

go 1.15

require (
	github.com/golang/mock v1.4.4
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.4.0
	github.com/vmware/govmomi v0.22.1
	github.com/vmware/vsphere-automation-sdk-go/lib v0.2.0
	github.com/vmware/vsphere-automation-sdk-go/runtime v0.2.0
	github.com/vmware/vsphere-automation-sdk-go/services/nsxt v0.3.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/sync v0.0.0-20201020160332-67f06af15bc9 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/grpc v1.27.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.19.7
	k8s.io/apimachinery v0.19.7
	k8s.io/client-go v0.19.7
	k8s.io/cloud-provider v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog/v2 v2.4.0
	k8s.io/kubernetes v1.19.7
)

replace (
	k8s.io/api => k8s.io/kubernetes/staging/src/k8s.io/api v0.0.0-20210113141052-1be54aaa7077
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20210113141052-1be54aaa7077
	k8s.io/apimachinery => k8s.io/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20210113141052-1be54aaa7077
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20210113141052-1be54aaa7077
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20210113141052-1be54aaa7077
	k8s.io/client-go => k8s.io/kubernetes/staging/src/k8s.io/client-go v0.0.0-20210113141052-1be54aaa7077
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20210113141052-1be54aaa7077
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20210113141052-1be54aaa7077
	k8s.io/code-generator => k8s.io/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20210113141052-1be54aaa7077
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20210113141052-1be54aaa7077
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20210113141052-1be54aaa7077
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kubectl => k8s.io/kubernetes/staging/src/k8s.io/kubectl v0.0.0-20210113141052-1be54aaa7077
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20210113141052-1be54aaa7077
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20210113141052-1be54aaa7077
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20210113141052-1be54aaa7077
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20210113141052-1be54aaa7077
)
