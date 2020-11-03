module cmd-exclude-prefixes-k8s

go 1.15

require (
	github.com/fsnotify/fsnotify v1.4.7
	github.com/ghodss/yaml v1.0.0
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/networkservicemesh/sdk v0.0.0-20200827102544-4b23de9a2ad4
	github.com/networkservicemesh/sdk-k8s v0.0.0-20200928112004-2b9589fc37e8
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	go.uber.org/goleak v1.0.1-0.20200717213025-100c34bdc9d6
	k8s.io/api v0.18.1
	k8s.io/apimachinery v0.18.2
	k8s.io/client-go v0.18.1
	sigs.k8s.io/cluster-api v0.3.10
)
