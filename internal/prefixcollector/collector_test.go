// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prefixcollector_test

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"context"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"os"
	"path/filepath"
	"time"

	"cmd-exclude-prefixes-k8s/internal/utils"
	"io/ioutil"
	"testing"
)

type dummyPrefixSource struct {
	prefixes []string
}

const (
	configMapPath     = "./testfiles/configMap.yaml"
	kubeConfigMapPath = "./testfiles/kubeAdmConfigMap.yaml"
	testFilePath      = "testFile.yaml"
	configMapName     = "test"
)

func (d *dummyPrefixSource) Start(notifyChan chan<- struct{}) {
	go func() { utils.Notify(notifyChan) }()
}

func (d *dummyPrefixSource) GetPrefixes() []string {
	return d.prefixes
}

func newDummyPrefixSource(prefixes []string) *dummyPrefixSource {
	return &dummyPrefixSource{prefixes}
}

func TestCollectorWithDummySources(t *testing.T) {
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
				"127.0.3.1/18",
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
	expectedResult := []string{
		"10.244.0.0/16",
		"10.96.0.0/12",
	}

	ctx := createConfigMap(t, prefixcollector.KubeNamespace, kubeConfigMapPath)

	configMapSource := prefixcollector.
		NewKubeAdmPrefixSource(ctx)
	options := prefixcollector.WithSources([]prefixcollector.ExcludePrefixSource{configMapSource})
	testCollector(t, expectedResult, options, prefixcollector.WithFilePath(testFilePath))
}

func TestConfigMapSource(t *testing.T) {
	expectedResult := []string{
		"168.0.0.0/10",
		"1.0.0.0/11",
	}

	ctx := createConfigMap(t, prefixcollector.DefaultConfigMapNamespace, configMapPath)

	configMapSource := prefixcollector.
		NewConfigMapPrefixSource(ctx, configMapName, prefixcollector.DefaultConfigMapNamespace)
	options := prefixcollector.WithSources([]prefixcollector.ExcludePrefixSource{configMapSource})
	testCollector(t, expectedResult, options, prefixcollector.WithFilePath(testFilePath))
}

func createConfigMap(t *testing.T, namespace, configPath string) context.Context {
	ctx := context.Background()
	clientSet := fake.NewSimpleClientset()
	configMap := getConfigMap(t, configPath)

	ctx = context.WithValue(ctx, utils.ClientSetKey, clientSet)
	_, _ = clientSet.CoreV1().
		ConfigMaps(namespace).
		Create(ctx, configMap, metav1.CreateOptions{})

	return ctx
}

func testCollector(t *testing.T, expectedResult []string, options ...prefixcollector.ExcludePrefixCollectorOption) {
	collector := prefixcollector.NewExcludePrefixCollector(context.Background(), options...)
	collector.Start()
	defer func() { _ = os.Remove(testFilePath) }()
	<-time.After(time.Second * 2)
	bytes, err := ioutil.ReadFile(testFilePath)
	if err != nil {
		t.Fatal("Error reading test file: ", err)
	}

	prefixes, err := utils.YamlToPrefixes(bytes)
	if err != nil {
		t.Fatal("Error transforming yaml to prefixes: ", err)
	}

	require.ElementsMatch(t, expectedResult, prefixes.PrefixesList)
}

func getConfigMap(t *testing.T, filePath string) *v1.ConfigMap {
	destination := v1.ConfigMap{}
	bytes, err := ioutil.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatal("Error reading user config map: ", err)
	}
	if err = yaml.Unmarshal(bytes, &destination); err != nil {
		t.Fatal("Error decoding user config map: ", err)
	}

	return &destination
}
