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

package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
)

type ExcludePrefixSource interface {
	GetNotifyChannel() <-chan struct{}
	GetPrefixes() []string
}

type ExcludePrefixCollector struct {
	baseExcludePrefixes     []string
	excludePrefixesFilePath string
	sources                 []ExcludePrefixSource
}

type ExcludePrefixCollectorOption func(*ExcludePrefixCollector, context.Context)

const (
	excludedPrefixesEnv       = "EXCLUDED_PREFIXES"
	configMapNamespaceEnv     = "CONFIG_NAMESPACE"
	defaultConfigMapNamespace = "default"
)

func WithConfigMapSource() ExcludePrefixCollectorOption {
	namespace := os.Getenv(configMapNamespaceEnv)
	if namespace == "" {
		namespace = defaultConfigMapNamespace
	}
	return func(collector *ExcludePrefixCollector, ctx context.Context) {
		configMapWatcher := NewConfigMapPrefixSource(ctx, "nsm-config-volume", namespace)
		collector.sources = append(collector.sources, configMapWatcher)
	}
}

func WithKubeadmConfigSource() ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector, ctx context.Context) {
		kubeAdmPrefixSource := NewKubeAdmPrefixSource(ctx)
		collector.sources = append(collector.sources, kubeAdmPrefixSource)
	}
}

func WithKubernetesSource() ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector, ctx context.Context) {
		kubernetesSource := NewKubernetesPrefixSource(ctx)
		collector.sources = append(collector.sources, kubernetesSource)
	}
}

func NewExcludePrefixCollector(filePath string, ctx context.Context,
	options ...ExcludePrefixCollectorOption) *ExcludePrefixCollector {
	collector := &ExcludePrefixCollector{
		getPrefixesFromEnv(),
		filePath,
		make([]ExcludePrefixSource, 0, len(options)),
	}

	for _, option := range options {
		option(collector, ctx)
	}

	return collector
}

func getPrefixesFromEnv() []string {
	var envPrefixes []string
	excludedPrefixesEnv, ok := os.LookupEnv(excludedPrefixesEnv)
	if ok {
		envPrefixes = strings.Split(excludedPrefixesEnv, ",")
		if err := validatePrefixes(envPrefixes); err == nil {
			return envPrefixes
		}
	}

	return envPrefixes
}

func (pcs *ExcludePrefixCollector) Start() {
	notifyChannel := getNotifyChannel(pcs.sources)
	go func() {
		for range notifyChannel {
			pcs.updateExcludedPrefixesConfigmap()
		}
	}()
}

func (pcs *ExcludePrefixCollector) updateExcludedPrefixesConfigmap() {
	excludePrefixPool, _ := NewExcludePrefixPool(pcs.baseExcludePrefixes...)

	for _, v := range pcs.sources {
		sourcePrefixes := v.GetPrefixes()
		if len(sourcePrefixes) == 0 {
			continue
		}

		if err := excludePrefixPool.Add(v.GetPrefixes()); err != nil {
			logrus.Error(err)
			return
		}
	}

	data, err := utils.PrefixesToYaml(excludePrefixPool.GetPrefixes())
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
		return
	}

	err = ioutil.WriteFile(pcs.excludePrefixesFilePath, data, 0644)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}
}

func getNotifyChannel(sources []ExcludePrefixSource) <-chan struct{} {
	channels := make([]<-chan struct{}, len(sources))
	for _, v := range sources {
		channels = append(channels, v.GetNotifyChannel())
	}
	return utils.MergeNotifyChannels(channels...)
}
