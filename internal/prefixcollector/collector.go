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
	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
)

type ExcludePrefixSource interface {
	Start(chan<- struct{})
	GetPrefixes() []string
}

type ExcludePrefixCollector struct {
	notifyChan          chan struct{}
	baseExcludePrefixes []string
	outputFilePath      string
	sources             []ExcludePrefixSource
}

type ExcludePrefixCollectorOption func(*ExcludePrefixCollector)

const (
	excludedPrefixesEnv       = "EXCLUDED_PREFIXES"
	configMapNamespaceEnv     = "CONFIG_NAMESPACE"
	DefaultConfigMapNamespace = "default"
	defaultConfigMapName      = "nsm-config-volume"
)

func WithFilePath(filePath string) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.outputFilePath = filePath
	}
}

func WithNotifyChan(notifyChan chan struct{}) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.notifyChan = notifyChan
	}
}

func WithSources(sources []ExcludePrefixSource) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.sources = sources
	}
}

func NewExcludePrefixCollector(ctx context.Context, options ...ExcludePrefixCollectorOption) *ExcludePrefixCollector {
	collector := &ExcludePrefixCollector{
		baseExcludePrefixes: getPrefixesFromEnv(),
		notifyChan:          make(chan struct{}, 1),
	}

	for _, option := range options {
		option(collector)
	}

	if collector.outputFilePath == "" {
		collector.outputFilePath = getDefaultOutputFilePath()
	}

	if collector.sources == nil {
		collector.sources = getDefaultSources(ctx)
	}

	return collector
}

func getDefaultSources(ctx context.Context) []ExcludePrefixSource {
	return []ExcludePrefixSource{
		NewKubeAdmPrefixSource(ctx),
		NewKubernetesPrefixSource(ctx),
		NewConfigMapPrefixSource(ctx, defaultConfigMapName, getDefaultConfigMapNamespace()),
	}
}

func getDefaultConfigMapNamespace() string {
	configMapNamespace := os.Getenv(configMapNamespaceEnv)
	if configMapNamespace == "" {
		configMapNamespace = DefaultConfigMapNamespace
	}
	return configMapNamespace
}

func getDefaultOutputFilePath() string {
	if err := utils.CreateDirIfNotExists(prefixpool.NSMConfigDir); err != nil {
		logrus.Fatalf("Failed to create exclude prefixes directory %v: %v", prefixpool.NSMConfigDir, err)
	}
	return prefixpool.PrefixesFilePathDefault
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

func (epc *ExcludePrefixCollector) Start() {
	for _, source := range epc.sources {
		source.Start(epc.notifyChan)
	}

	go func() {
		for range epc.notifyChan {
			epc.updateExcludedPrefixesConfigmap()
		}
	}()
}

func (epc *ExcludePrefixCollector) GetNotifyChan() chan<- struct{} {
	return epc.notifyChan
}

func (epc *ExcludePrefixCollector) updateExcludedPrefixesConfigmap() {
	// error check skipped, because we've already validated baseExcludePrefixes
	excludePrefixPool, _ := NewExcludePrefixPool(epc.baseExcludePrefixes...)

	for _, v := range epc.sources {
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

	err = ioutil.WriteFile(epc.outputFilePath, data, 0644)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}
}
