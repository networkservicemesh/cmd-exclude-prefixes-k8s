module cmd-exclude-prefixes-k8s

go 1.14

require (
	github.com/edwarnicke/dl v0.0.0-20200425181122-b4e16e4cb9fd // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/networkservicemesh/sdk v0.0.0-20200827102544-4b23de9a2ad4
	github.com/onsi/gomega v1.7.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.1
	k8s.io/kubernetes v1.18.1
)

replace (
	k8s.io/api => k8s.io/api v0.18.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.2-beta.0
	k8s.io/apiserver => k8s.io/apiserver v0.18.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.1
	k8s.io/client-go => k8s.io/client-go v0.18.1
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.1
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.1
	k8s.io/code-generator => k8s.io/code-generator v0.18.2-beta.0
	k8s.io/component-base => k8s.io/component-base v0.18.1
	k8s.io/cri-api => k8s.io/cri-api v0.18.2-beta.0
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.1
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.1
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.1
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.1
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.1
	k8s.io/kubectl => k8s.io/kubectl v0.18.1
	k8s.io/kubelet => k8s.io/kubelet v0.18.1
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.1
	k8s.io/metrics => k8s.io/metrics v0.18.1
	k8s.io/node-api => k8s.io/node-api v0.17.1
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.1
	k8s.io/sample-cli-plugin => k8s.io/sample-cli-plugin v0.18.1
	k8s.io/sample-controller => k8s.io/sample-controller v0.18.1
)
