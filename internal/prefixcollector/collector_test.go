package prefixcollector_test

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"context"

	//"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"testing"
)

type dummyPrefixSource struct {
	prefixes []string
}

const testFilePath = "testFile.yaml"

func (d *dummyPrefixSource) Start(chan<- struct{}) {

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

func testCollector(t *testing.T, expectedResult []string, options ...prefixcollector.ExcludePrefixCollectorOption) {
	collector := prefixcollector.NewExcludePrefixCollector(context.Background(), options...)
	collector.UpdateExcludedPrefixesConfigmap()
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
