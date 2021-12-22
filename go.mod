module k8s.io/cloud-provider-vsphere

go 1.17

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/mock v1.5.0
	github.com/golang/protobuf v1.5.2
	github.com/google/uuid v1.2.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/vmware-tanzu/vm-operator-api v0.1.4-0.20201118171008-5ca641b0e126
	github.com/vmware/govmomi v0.22.1
	github.com/vmware/vsphere-automation-sdk-go/lib v0.2.0
	github.com/vmware/vsphere-automation-sdk-go/runtime v0.2.0
	github.com/vmware/vsphere-automation-sdk-go/services/nsxt v0.3.0
	golang.org/x/net v0.0.0-20211015210444-4f30a5c0130f
	golang.org/x/tools v0.1.8 // indirect
	google.golang.org/grpc v1.39.0
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1 // indirect
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/cloud-provider v0.22.1
	k8s.io/code-generator v0.22.1
	k8s.io/component-base v0.22.1
	k8s.io/klog/v2 v2.9.0
	sigs.k8s.io/cluster-api v0.4.5 // indirect
	sigs.k8s.io/cluster-api/test v0.4.5
	sigs.k8s.io/controller-runtime v0.9.7
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/onsi/ginkgo => github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega => github.com/onsi/gomega v1.11.0
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.5
)
