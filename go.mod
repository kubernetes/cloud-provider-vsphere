module k8s.io/cloud-provider-vsphere

go 1.12

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible // indirect
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/google/btree v1.0.0 // indirect
	github.com/google/uuid v1.1.1
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.8.0
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/spf13/cobra v0.0.3
	github.com/spf13/pflag v1.0.3
	github.com/vmware/govmomi v0.21.0
	github.com/vmware/vsphere-automation-sdk-go/lib v0.1.1
	github.com/vmware/vsphere-automation-sdk-go/runtime v0.1.1
	github.com/vmware/vsphere-automation-sdk-go/services/nsxt v0.1.1
	golang.org/x/lint v0.0.0-20200130185559-910be7a94367 // indirect
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	golang.org/x/tools v0.0.0-20200301222351-066e0c02454c // indirect
	google.golang.org/grpc v1.22.1
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/square/go-jose.v2 v2.3.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	honnef.co/go/tools v0.0.1-2020.1.3 // indirect
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v0.0.0
	k8s.io/cloud-provider v0.0.0
	k8s.io/component-base v0.0.0
	k8s.io/klog v0.3.2
	k8s.io/kube-openapi v0.0.0-20190401085232-94e1e7b7574c // indirect
	k8s.io/kubernetes v1.15.0
	k8s.io/sample-controller v0.0.0-20190731144349-6f8905ae4ee5
)

replace (
	// these replacements are pinned to e8462b5b5dc2 which is the sha associated with the 1.15.0 tag on k/k
	// as you cannot pin them to v1.15.0 directly
	k8s.io/api => k8s.io/kubernetes/staging/src/k8s.io/api v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/apiextensions-apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiextensions-apiserver v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/apimachinery => k8s.io/kubernetes/staging/src/k8s.io/apimachinery v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/apiserver => k8s.io/kubernetes/staging/src/k8s.io/apiserver v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/cli-runtime => k8s.io/kubernetes/staging/src/k8s.io/cli-runtime v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/client-go => k8s.io/kubernetes/staging/src/k8s.io/client-go v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/cloud-provider => k8s.io/kubernetes/staging/src/k8s.io/cloud-provider v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/cluster-bootstrap => k8s.io/kubernetes/staging/src/k8s.io/cluster-bootstrap v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/code-generator => k8s.io/kubernetes/staging/src/k8s.io/code-generator v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/component-base => k8s.io/kubernetes/staging/src/k8s.io/component-base v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/cri-api => k8s.io/kubernetes/staging/src/k8s.io/cri-api v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/csi-translation-lib => k8s.io/kubernetes/staging/src/k8s.io/csi-translation-lib v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/kube-aggregator => k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/kube-controller-manager => k8s.io/kubernetes/staging/src/k8s.io/kube-controller-manager v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/kube-proxy => k8s.io/kubernetes/staging/src/k8s.io/kube-proxy v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/kube-scheduler => k8s.io/kubernetes/staging/src/k8s.io/kube-scheduler v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/kubelet => k8s.io/kubernetes/staging/src/k8s.io/kubelet v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/legacy-cloud-providers => k8s.io/kubernetes/staging/src/k8s.io/legacy-cloud-providers v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/metrics => k8s.io/kubernetes/staging/src/k8s.io/metrics v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/node-api => k8s.io/kubernetes/staging/src/k8s.io/node-api v0.0.0-20190615005809-e8462b5b5dc2
	k8s.io/sample-apiserver => k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver v0.0.0-20190615005809-e8462b5b5dc2
)
