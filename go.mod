module cmd-exclude-prefixes-k8s

go 1.16

require (
	github.com/fsnotify/fsnotify v1.4.9
	github.com/ghodss/yaml v1.0.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/networkservicemesh/sdk v0.5.1-0.20220321161040-57746e0ee944
	github.com/networkservicemesh/sdk-k8s v0.0.0-20220321161717-5570632d309d
	github.com/onsi/gomega v1.16.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/stretchr/testify v1.7.0
	go.uber.org/goleak v1.1.12
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	sigs.k8s.io/cluster-api v0.3.10
)
