// Copyright (c) 2020-2021 Doc.ai and/or its affiliates.
//
// Copyright (c) 2022-2023 Cisco and/or its affiliates.
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
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	configMapPath         = "./testfiles/configMap.yaml"
	kubeConfigMapPath     = "./testfiles/kubeAdmConfigMap.yaml"
	kubeConfigMapPathIPv6 = "./testfiles/kubeAdmConfigMapIPv6.yaml"
	nsmConfigMapPath      = "./testfiles/nsmConfigMap.yaml"
	excludedPrefixesKey   = "excluded_prefixes.yaml"
	configMapNamespace    = "default"
	userConfigMapName     = "test"
	userConfigMapKey      = "excluded_prefixes_input.yaml"
	nsmConfigMapName      = "nsm-config"
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

func getConfigMap(t *testing.T, filePath string) *v1.ConfigMap {
	destination := v1.ConfigMap{}
	bytes, err := os.ReadFile(filepath.Clean(filePath))
	if err != nil {
		t.Fatal("Error reading user config map: ", err)
	}
	if err = yaml.Unmarshal(bytes, &destination); err != nil {
		t.Fatal("Error decoding user config map: ", err)
	}

	return &destination
}
