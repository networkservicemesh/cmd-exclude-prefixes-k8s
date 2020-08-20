package prefix_sources

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"context"
	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"time"
)

type ConfigMapPrefixSource struct {
	configMapName      string
	configMapNameSpace string
	configMapInterface v1.ConfigMapInterface
	prefixChan         chan []string
	errorChan          chan error
}

func NewConfigMapPrefixSource(context context.Context, name, namespace string) (*ConfigMapPrefixSource, error) {
	clientSet := prefixcollector.FromContext(context)
	configMapInterface := clientSet.CoreV1().ConfigMaps(namespace)

	// check if config map with such name exists
	_, err := configMapInterface.Get(context, name, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get ConfigMap '%s/%s': %v", namespace, name, err)
		return nil, err
	}
	cmps := ConfigMapPrefixSource{
		name,
		namespace,
		configMapInterface,
		make(chan []string, 1),
		make(chan error),
	}

	go cmps.watchConfigMap(context)

	return &cmps, nil
}

func (cmps *ConfigMapPrefixSource) ResultChan() <-chan []string {
	return cmps.prefixChan
}

func (cmps *ConfigMapPrefixSource) ErrorChan() <-chan error {
	return cmps.errorChan
}

func (cmps *ConfigMapPrefixSource) watchConfigMap(context context.Context) {
	//prefixesList := make([]string, 5)
	for {
		cm, err := cmps.configMapInterface.Get(context, cmps.configMapName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMap '%s/%s': %v", cmps.configMapNameSpace, cmps.configMapName, err)
			cmps.errorChan <- err
			return
		}

		bytes := []byte(cm.Data[prefixpool.PrefixesFile])
		prefixes, err := prefixcollector.YamlToPrefixes(bytes)
		if err != nil {
			logrus.Errorf("Can not create unmarshal prefixes, err: %v", err.Error())
			cmps.errorChan <- err
			return
		}
		// if prefixes.PrefixesList != prefixesList
		cmps.prefixChan <- prefixes.PrefixesList
		<-time.After(time.Second * 10)
	}
}
