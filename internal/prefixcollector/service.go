package prefixcollector

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

type contextKeyType string

const (
	ClientsetKey  contextKeyType = "clientsetKey"
	KubeNamespace                = "kube-system"
	KubeName                     = "kubeadm-config"
)

func StartServer(filePath string, notifyCh <-chan []string) error {
	go func() {
		for {
			select {
			case prefixes, ok := <-notifyCh:
				if ok {
					logrus.Infof("Excluded prefixes changed: %v", prefixes)
					// there are unsaved prefixes, save them
					updateExcludedPrefixesConfigmap(filePath, prefixes)
				}
			case <-time.After(time.Second):
			}
		}
	}()

	return nil
}

func updateExcludedPrefixesConfigmap(filePath string, prefixes []string) {
	data, err := PrefixesToYaml(prefixes)
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
	}

	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}

}

func FromEnv() func(context context.Context) ([]string, error) {
	return func(context context.Context) ([]string, error) {
		excludedPrefixesEnv, ok := os.LookupEnv(ExcludedPrefixesEnv)
		if !ok {
			return []string{}, nil
		}
		logrus.Infof("Getting excludedPrefixes from ENV: %v", excludedPrefixesEnv)
		return strings.Split(excludedPrefixesEnv, ","), nil
	}
}

func FromConfigMap(name, namespace string) func(context context.Context) ([]string, error) {
	return func(context context.Context) ([]string, error) {

		clientset := FromContext(context)

		configMaps := clientset.CoreV1().ConfigMaps(namespace)

		cm, err := configMaps.Get(context, name, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMap '%s/%s': %v", namespace, name, err)
			return nil, err
		}

		bytes := []byte(cm.Data[prefixpool.PrefixesFile])
		prefixes, err := YamlToPrefixes(bytes)
		if err != nil {
			logrus.Errorf("Can not create unmarshal prefixes, err: %v", err.Error())
			return nil, err
		}

		return prefixes.PrefixesList, nil
	}
}

func FromKubernetes() func(context context.Context) ([]string, error) {
	var prefixes []string
	var once sync.Once
	var mutex sync.Mutex
	var result []string

	return func(context context.Context) ([]string, error) {
		clientSet := FromContext(context)

		// checks if kubeadm-config exists
		prefixes, err := getExcludedPrefixesFromKubernetesConfigFile(context)
		if err != nil {
			return nil, err
		}
		//_, err := clientSet.CoreV1().
		//	ConfigMaps(KubeNamespace).
		//	Get(context, KubeName, metav1.GetOptions{})
		//if err == nil {
		//	prefixes, err = getExcludedPrefixesFromKubernetesConfigFile(context)
		//	return prefixes, err
		//}

		// monitoring goroutine
		once.Do(func() {
			ch := monitorSubnets(clientSet)

			go func() {
				for {
					if context.Err() != nil {
						return
					}

					prefixes = <-ch
					mutex.Lock()
					result = prefixes
					mutex.Unlock()
				}
			}()
		})

		mutex.Lock()
		defer mutex.Unlock()
		return result, nil
	}
}
