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
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/suite"
	"go.uber.org/goleak"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	configMapPath       = "./testfiles/configMap.yaml"
	kubeConfigMapPath   = "./testfiles/kubeAdmConfigMap.yaml"
	nsmConfigMapPath    = "./testfiles/nsmConfigMap.yaml"
	excludedPrefixesKey = "excluded_prefixes.yaml"
	configMapNamespace  = "default"
	userConfigMapName   = "test"
	nsmConfigMapName    = "nsm-config"
)

type dummyPrefixSource struct {
	prefixes []string
}

func (d *dummyPrefixSource) Prefixes() []string {
	return d.prefixes
}

func newDummyPrefixSource(prefixes []string) *dummyPrefixSource {
	return &dummyPrefixSource{prefixes}
}

type ExcludedPrefixesSuite struct {
	suite.Suite
	clientSet kubernetes.Interface
}

func (eps *ExcludedPrefixesSuite) SetupSuite() {
	eps.clientSet = fake.NewSimpleClientset()
	eps.createConfigMap(configMapNamespace, nsmConfigMapPath)
}

func (eps *ExcludedPrefixesSuite) TestCollectorWithDummySources() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	cond := sync.NewCond(&sync.Mutex{})
	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()

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

	eps.testCollector(ctx, cond, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestConfigMapSource() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"168.0.0.0/10",
		"1.0.0.0/11",
	}
	cond := sync.NewCond(&sync.Mutex{})

	configMap, ctx := eps.createConfigMap(configMapNamespace, configMapPath)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sources := []prefixcollector.ExcludePrefixSource{
		prefixcollector.NewConfigMapPrefixSource(
			ctx,
			cond,
			userConfigMapName,
			configMapNamespace,
		),
	}

	_, err := eps.clientSet.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		eps.T().Fatalf("Error updating config map %v/%v: %v", configMap.Namespace, configMap.Name, err)
	}

	eps.testCollector(ctx, cond, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestKubeAdmConfigSource() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"10.244.0.0/16",
		"10.96.0.0/12",
	}

	cond := sync.NewCond(&sync.Mutex{})
	configMap, ctx := eps.createConfigMap(prefixcollector.KubeNamespace, kubeConfigMapPath)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sources := []prefixcollector.ExcludePrefixSource{
		prefixcollector.NewKubeAdmPrefixSource(ctx, cond),
	}

	_, err := eps.clientSet.CoreV1().ConfigMaps(configMap.Namespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		eps.T().Fatalf("Error updating config map %v/%v: %v", configMap.Namespace, configMap.Name, err)
	}

	eps.testCollector(ctx, cond, expectedResult, sources)
}

func TestExcludedPrefixesSuite(t *testing.T) {
	suite.Run(t, &ExcludedPrefixesSuite{})
}

func (eps *ExcludedPrefixesSuite) createConfigMap(namespace, configPath string) (*v1.ConfigMap, context.Context) {
	ctx := context.Background()
	configMap := getConfigMap(eps.T(), configPath)

	ctx = prefixcollector.WithKubernetesInterface(ctx, eps.clientSet)
	configMap, err := eps.clientSet.CoreV1().
		ConfigMaps(namespace).
		Create(ctx, configMap, metav1.CreateOptions{})

	if err != nil {
		eps.T().Fatalf("Error creating config map: %v", err)
	}

	return configMap, ctx
}

func (eps *ExcludedPrefixesSuite) testCollector(ctx context.Context, cond *sync.Cond,
	expectedResult []string, sources []prefixcollector.ExcludePrefixSource) {
	collector := prefixcollector.NewExcludePrefixCollector(
		cond,
		nsmConfigMapName,
		configMapNamespace,
		sources...,
	)

	errCh := eps.watchConfigMap(ctx)

	go collector.Serve(ctx)

	if err := <-errCh; err != nil {
		eps.T().Fatal("Error watching config map: ", err)
	}

	configMap, err := eps.clientSet.CoreV1().ConfigMaps(configMapNamespace).Get(ctx, nsmConfigMapName, metav1.GetOptions{})
	if err != nil {
		eps.T().Fatal("Error getting nsm config map: ", err)
	}

	prefixes, err := utils.YamlToPrefixes([]byte(configMap.Data[excludedPrefixesKey]))
	if err != nil {
		eps.T().Fatal("Error transforming yaml to prefixes: ", err)
	}

	eps.Require().ElementsMatch(expectedResult, prefixes)
}

func (eps *ExcludedPrefixesSuite) watchConfigMap(ctx context.Context) <-chan error {
	watcher, err := eps.clientSet.CoreV1().ConfigMaps(configMapNamespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		eps.T().Fatalf("Error watching configmap: %v", err)
	}

	errorCh := make(chan error)
	go func() {
		for {
			select {
			case event := <-watcher.ResultChan():
				configMap := event.Object.(*v1.ConfigMap)
				if event.Type == watch.Error {
					errorCh <- errors.New("error watching configmap")
					return
				}

				if configMap.Name == nsmConfigMapName && (event.Type == watch.Added || event.Type == watch.Modified) {
					close(errorCh)
					return
				}
			case <-ctx.Done():
				errorCh <- errors.New("context canceled")
				return
			}
		}
	}()

	return errorCh
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
