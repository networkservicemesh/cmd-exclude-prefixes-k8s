package prefixcollector_test

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"context"
	"github.com/ghodss/yaml"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"time"

	"cmd-exclude-prefixes-k8s/internal/utils"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

type dummyPrefixSource struct {
	prefixes []string
}

const (
	testFilePath  = "testFile.yaml"
	configMapName = "test"
)

func (d *dummyPrefixSource) Start(chan<- struct{}) {
	panic("Implement me")
}

func (d *dummyPrefixSource) GetPrefixes() []string {
	return d.prefixes
}

func newDummyPrefixSource(prefixes []string) *dummyPrefixSource {
	return &dummyPrefixSource{prefixes}
}

func TestCollector(t *testing.T) {
	sources := []prefixcollector.ExcludePrefixSource{
		newDummyPrefixSource(
			[]string{
				"127.0.0.1/16",
				"127.0.2.1/16",
				"168.92.0.1/24",
			},
		),
		newDummyPrefixSource(
			[]string{
				"127.0.3.1/16",
				"134.56.0.1/8",
				"168.92.0.1/16",
			},
		),
	}
	sourcesOption := prefixcollector.WithSources(sources)

	expectedResult := []string{
		"127.0.0.0/16",
		"168.92.0.0/16",
		"134.0.0.0/8",
	}

	testCollector(t, expectedResult, sourcesOption, prefixcollector.WithFilePath(testFilePath))
}

func TestKubeAdmConfigSource(t *testing.T) {
	panic("implement me")
}

func TestConfigMapSource(t *testing.T) {
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	configMap := getConfigMap(t)

	ctx = context.WithValue(ctx, utils.ClientSetKey, clientSet)
	_, _ = clientSet.CoreV1().
		ConfigMaps(prefixcollector.DefaultConfigMapNamespace).
		Create(ctx, configMap, metav1.CreateOptions{})

	configMapSource := prefixcollector.
		NewConfigMapPrefixSource(ctx, configMapName, prefixcollector.DefaultConfigMapNamespace)
	options := prefixcollector.WithSources([]prefixcollector.ExcludePrefixSource{configMapSource})
	testCollector(t, []string{"168.0.0.0/10", "1.0.0.0/11"}, options, prefixcollector.WithFilePath(testFilePath))
}

func testCollector(t *testing.T, expectedResult []string, options ...prefixcollector.ExcludePrefixCollectorOption) {
	collector := prefixcollector.NewExcludePrefixCollector(context.Background(), options...)
	collector.Start()
	<-time.After(time.Second)
	bytes, err := ioutil.ReadFile(testFilePath)
	if err != nil {
		t.Fatal("Error reading test file: ", err)
	}

	prefixes, err := utils.YamlToPrefixes(bytes)
	if err != nil {
		t.Fatal("Error transforming yaml to prefixes: ", err)
	}

	assert.ElementsMatch(t, expectedResult, prefixes.PrefixesList)
	_ = os.Remove(testFilePath)
}

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
