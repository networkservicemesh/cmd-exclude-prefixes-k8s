package prefixcollector_test

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"github.com/ghodss/yaml"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"testing"
)

func getConfigMap(t *testing.T) *v1.ConfigMap {
	const filePath = "./testfiles/userfile.yaml"
	destination := v1.ConfigMap{}
	bytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		t.Fatal("Error reading user config map: ", err)
	}
	if err = yaml.Unmarshal(bytes, &destination); err != nil {
		t.Fatal("Error decoding user config map: ", err)
	}

	return &destination
}

func TestConstructClientSet(t *testing.T) {
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	configMap := getConfigMap(t)
	ctx = context.WithValue(ctx, utils.ClientSetKey, clientSet)
	_, _ = clientSet.CoreV1().ConfigMaps(prefixcollector.DefaultConfigMapNamespace).Create(ctx, configMap, metav1.CreateOptions{})
	configMapSource := prefixcollector.NewConfigMapPrefixSource(ctx, "test", prefixcollector.DefaultConfigMapNamespace)
	<-configMapSource.GetNotifyChannel()
}
