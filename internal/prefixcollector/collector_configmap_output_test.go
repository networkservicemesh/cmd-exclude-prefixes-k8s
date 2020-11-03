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
	"cmd-exclude-prefixes-k8s/internal/prefixcollector/prefixsource"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"errors"
	"io/ioutil"
	"path/filepath"
	"testing"
	"time"

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
	eps.createConfigMap(context.Background(), configMapNamespace, nsmConfigMapPath)
}

func (eps *ExcludedPrefixesSuite) TearDownTest() {
	eps.deleteConfigMap(context.Background(), prefixsource.KubeNamespace, prefixsource.KubeName)
	eps.deleteConfigMap(context.Background(), configMapNamespace, userConfigMapName)
}

func (eps *ExcludedPrefixesSuite) TestCollectorWithDummySources() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	notifyChan := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()

	sources := []prefixcollector.PrefixSource{
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

	eps.testCollectorWithConfigmapOutput(ctx, notifyChan, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestConfigMapSource() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"168.0.0.0/10",
		"1.0.0.0/11",
	}
	notifyChan := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()
	eps.createConfigMap(ctx, configMapNamespace, configMapPath)

	sources := []prefixcollector.PrefixSource{
		prefixsource.NewConfigMapPrefixSource(
			ctx,
			notifyChan,
			userConfigMapName,
			configMapNamespace,
		),
	}

	eps.testCollectorWithConfigmapOutput(ctx, notifyChan, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestKubeAdmConfigSource() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"10.244.0.0/16",
		"10.96.0.0/12",
	}

	notifyChan := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()

	eps.createConfigMap(ctx, prefixsource.KubeNamespace, kubeConfigMapPath)

	sources := []prefixcollector.PrefixSource{
		prefixsource.NewKubeAdmPrefixSource(ctx, notifyChan),
	}

	eps.testCollectorWithConfigmapOutput(ctx, notifyChan, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestAllSources() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"10.244.0.0/16",
		"10.96.0.0/12",
		"127.0.0.0/16",
		"168.92.0.0/24",
		"168.0.0.0/10",
		"1.0.0.0/11",
	}

	notifyChan := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()

	eps.createConfigMap(ctx, prefixsource.KubeNamespace, kubeConfigMapPath)
	eps.createConfigMap(ctx, configMapNamespace, configMapPath)

	sources := []prefixcollector.PrefixSource{
		newDummyPrefixSource(
			[]string{
				"127.0.0.1/16",
				"127.0.2.1/16",
				"168.92.0.1/24",
			},
		),
		prefixsource.NewKubeAdmPrefixSource(ctx, notifyChan),
		prefixsource.NewConfigMapPrefixSource(
			ctx,
			notifyChan,
			userConfigMapName,
			configMapNamespace,
		),
	}

	eps.testCollectorWithConfigmapOutput(ctx, notifyChan, expectedResult, sources)
}

func TestExcludedPrefixesSuite(t *testing.T) {
	suite.Run(t, &ExcludedPrefixesSuite{})
}

func (eps *ExcludedPrefixesSuite) deleteConfigMap(ctx context.Context, namespace, name string) {
	ctx = prefixcollector.WithKubernetesInterface(ctx, eps.clientSet)
	_ = eps.clientSet.CoreV1().
		ConfigMaps(namespace).
		Delete(ctx, name, metav1.DeleteOptions{})
}

func (eps *ExcludedPrefixesSuite) createConfigMap(ctx context.Context, namespace, configPath string) {
	configMap := getConfigMap(eps.T(), configPath)
	_, err := eps.clientSet.CoreV1().
		ConfigMaps(namespace).
		Create(ctx, configMap, metav1.CreateOptions{})

	if err != nil {
		eps.T().Fatalf("Error creating config map: %v", err)
	}
}

func (eps *ExcludedPrefixesSuite) testCollectorWithConfigmapOutput(ctx context.Context, notifyChan chan struct{},
	expectedResult []string, sources []prefixcollector.PrefixSource) {
	collector := prefixcollector.NewExcludePrefixCollector(
		prefixcollector.WithNotifyChan(notifyChan),
		prefixcollector.WithConfigMapOutput(nsmConfigMapName, configMapNamespace),
		prefixcollector.WithSources(sources...),
	)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	errCh := eps.watchConfigMap(ctx, len(sources))

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

func (eps *ExcludedPrefixesSuite) watchConfigMap(ctx context.Context, maxModifyCount int) <-chan error {
	watcher, err := eps.clientSet.CoreV1().ConfigMaps(configMapNamespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		eps.T().Fatalf("Error watching configmap: %v", err)
	}

	errorCh := make(chan error)
	modifyCount := 0
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
					modifyCount++
					print(modifyCount)
					if modifyCount == maxModifyCount {
						close(errorCh)
						return
					}
				}
			case <-ctx.Done():
				close(errorCh)
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
