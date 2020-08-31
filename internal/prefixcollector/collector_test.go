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
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"cmd-exclude-prefixes-k8s/internal/utils"
	"io/ioutil"
	"testing"
)

const (
	configMapPath            = "./testfiles/configMap.yaml"
	kubeConfigMapPath        = "./testfiles/kubeAdmConfigMap.yaml"
	testExcludedPrefixesPath = "./testfiles/excludedPrefixes.yaml"
	configMapName            = "test"
)

type dummyPrefixSource struct {
	prefixes []string
}

func (d *dummyPrefixSource) Start(notifyChan chan<- struct{}) {
	go func() { utils.Notify(notifyChan) }()
}

func (d *dummyPrefixSource) Prefixes() []string {
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

	expectedResult := []string{
		"127.0.0.0/16",
		"168.92.0.0/16",
		"134.0.0.0/8",
	}

	testCollector(t, expectedResult, sources)
}

func TestKubeAdmConfigSource(t *testing.T) {
	expectedResult := []string{
		"10.244.0.0/16",
		"10.96.0.0/12",
	}

	ctx := createConfigMap(t, prefixcollector.KubeNamespace, kubeConfigMapPath)
	sources := []prefixcollector.ExcludePrefixSource{
		prefixcollector.NewKubeAdmPrefixSource(ctx),
	}

	testCollector(t, expectedResult, sources)
}

func TestConfigMapSource(t *testing.T) {
	expectedResult := []string{
		"168.0.0.0/10",
		"1.0.0.0/11",
	}

	ctx := createConfigMap(t, prefixcollector.DefaultConfigMapNamespace, configMapPath)
	sources := []prefixcollector.ExcludePrefixSource{
		prefixcollector.NewConfigMapPrefixSource(ctx, configMapName, prefixcollector.DefaultConfigMapNamespace),
	}

	testCollector(t, expectedResult, sources)
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

func testCollector(t *testing.T, expectedResult []string, sources []prefixcollector.ExcludePrefixSource) {
	options := []prefixcollector.ExcludePrefixCollectorOption{
		prefixcollector.WithSources(sources...),
		prefixcollector.WithFilePath(testExcludedPrefixesPath),
	}

	collector := prefixcollector.NewExcludePrefixCollector(
		context.Background(),
		[]string{},
		"",
		options...,
	)

	collector.Start()

	if err := watchFile(t, len(sources)); err != nil {
		t.Fatal("Error watching file: ", err)
	}

	bytes, err := ioutil.ReadFile(testExcludedPrefixesPath)
	if err != nil {
		t.Fatal("Error reading test file: ", err)
	}

	prefixes, err := utils.YamlToPrefixes(bytes)
	if err != nil {
		t.Fatal("Error transforming yaml to prefixes: ", err)
	}

	require.ElementsMatch(t, expectedResult, prefixes.PrefixesList)
}

func watchFile(t *testing.T, fileUpdatesRequired int) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := watcher.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	}()

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	go func() {
		for i := 0; i < fileUpdatesRequired; {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					i++
				}
			case watcherError, ok := <-watcher.Errors:
				if !ok {
					return
				}
				t.Error("error watching file:", watcherError)
			}
		}
		waitGroup.Done()
	}()

	err = watcher.Add(testExcludedPrefixesPath)
	if err != nil {
		return err
	}

	waitGroup.Wait()
	return nil
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
