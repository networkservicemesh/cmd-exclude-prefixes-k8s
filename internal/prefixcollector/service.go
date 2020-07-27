package prefixcollector

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/open-policy-agent/opa/plugins/rest"
	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

const (
	configMapName = "nsm-config"
)

type prefixService struct {
	sync.RWMutex
	excludedPrefixes prefixpool.PrefixPool
}

// NewExcludePrefixServer creates an instance of prefixService
func NewExcludePrefixServer(config *rest.Config) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	emptyPrefixPool, err := prefixpool.NewPrefixPool()
	if err != nil {
		return err
	}

	rv := &prefixService{
		excludedPrefixes: emptyPrefixPool,
	}

	if err := rv.monitorExcludedPrefixes(clientset); err != nil {
		return err
	}

	return nil
}

func (ps *prefixService) getExcludedPrefixes() prefixpool.PrefixPool {
	ps.RLock()
	defer ps.RUnlock()

	return ps.excludedPrefixes
}

func (ps *prefixService) monitorExcludedPrefixes(clientset *kubernetes.Clientset) error {
	poolCh, err := getExcludedPrefixesChan(clientset)
	if err != nil {
		return err
	}

	go func() {
		configMaps := clientset.CoreV1().ConfigMaps(common.GetNamespace())
		prefixes := []string{}
		for {
			select {
			case pool, ok := <-poolCh:
				if ok {
					prefixes = pool.GetPrefixes()
					logrus.Infof("Excluded prefixes changed: %v", prefixes)
				}
			case <-time.After(5 * time.Second):
			}
			if len(prefixes) > 0 {
				// there is unsaved prefixes, save them
				if updateExcludedPrefixesConfigmap(configMaps, prefixes) {
					prefixes = []string{}
				}
			}
		}
	}()

	return nil
}

func updateExcludedPrefixesConfigmap(configMaps v1.ConfigMapInterface, prefixes []string) bool {
	cm, err := configMaps.Get(context.TODO(), configMapName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Failed to get ConfigMap '%s/%s': %v", common.GetNamespace(), configMapName, err)
		return false
	}
	cm.Data[prefixpool.PrefixesFile] = buildPrefixesYaml(prefixes)
	_, err = configMaps.Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("Failed to update ConfigMap: %v", err)
		return false
	}
	logrus.Info("Successfully updated excluded prefixes config file")
	return true
}

func buildPrefixesYaml(prefixes []string) string {
	marker := "prefixes:"
	if len(prefixes) == 0 {
		return marker + " []"
	}
	return strings.Join(append([]string{marker}, prefixes...), "\n- ")
}
