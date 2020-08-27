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

// Package prefixcollector contains excluded prefix collector, prefix sources and functions working with them
package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"io/ioutil"
	"os"
	"strings"

	"github.com/networkservicemesh/sdk/pkg/networkservice/common/excludedprefixes"

	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

// ExcludePrefixSource is source of excluded prefixes
type ExcludePrefixSource interface {
	Start(chan<- struct{})
	GetPrefixes() []string
}

// ExcludePrefixCollector is service, collecting excluded prefixes from list of ExcludePrefixSource
// and environment variable "EXCLUDED_PREFIXES"
// and writing result to outputFilePath in yaml format
type ExcludePrefixCollector struct {
	notifyChan          chan struct{}
	baseExcludePrefixes []string
	outputFilePath      string
	sources             []ExcludePrefixSource
}

// ExcludePrefixCollectorOption is functional option for ExcludePrefixCollector
type ExcludePrefixCollectorOption func(*ExcludePrefixCollector)

const (
	// DefaultConfigMapNamespace is default namespace of kubernetes ConfigMap
	DefaultConfigMapNamespace = "default"
	excludedPrefixesEnv       = "EXCLUDED_PREFIXES"
	configMapNamespaceEnv     = "CONFIG_NAMESPACE"
	defaultConfigMapName      = "nsm-config-volume"
)

// WithFilePath returns ExcludePrefixCollectorOption, that set filePath as collector's outputFilePath
func WithFilePath(filePath string) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.outputFilePath = filePath
	}
}

// WithNotifyChan returns ExcludePrefixCollectorOption, that set notifyChan as collector's notifyChan
func WithNotifyChan(notifyChan chan struct{}) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.notifyChan = notifyChan
	}
}

// WithSources returns ExcludePrefixCollectorOption, that set sources as collector's prefix sources
func WithSources(sources []ExcludePrefixSource) ExcludePrefixCollectorOption {
	return func(collector *ExcludePrefixCollector) {
		collector.sources = sources
	}
}

// NewExcludePrefixCollector creates ExcludePrefixCollector
// and applies every ExcludePrefixCollectorOption to it
func NewExcludePrefixCollector(ctx context.Context, options ...ExcludePrefixCollectorOption) *ExcludePrefixCollector {
	collector := &ExcludePrefixCollector{
		baseExcludePrefixes: getPrefixesFromEnv(),
		notifyChan:          make(chan struct{}, 1),
	}

	for _, option := range options {
		option(collector)
	}

	if collector.outputFilePath == "" {
		collector.outputFilePath = excludedprefixes.PrefixesFilePathDefault
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

func getPrefixesFromEnv() []string {
	var envPrefixes []string
	excludedPrefixesEnv, ok := os.LookupEnv(excludedPrefixesEnv)
	if ok {
		envPrefixes = strings.Split(excludedPrefixesEnv, ",")
		if err := utils.ValidatePrefixes(envPrefixes); err == nil {
			return envPrefixes
		}
	}

	return envPrefixes
}

// Start - starts every source, then begin monitoring notifyChan.
// Updates exclude prefix file after every notification.
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

func (epc *ExcludePrefixCollector) updateExcludedPrefixesConfigmap() {
	// error check skipped, because we've already validated baseExcludePrefixes
	excludePrefixPool, _ := prefixpool.New(epc.baseExcludePrefixes...)

	for _, v := range epc.sources {
		sourcePrefixes := v.GetPrefixes()
		if len(sourcePrefixes) == 0 {
			continue
		}

		if err := excludePrefixPool.ReleaseExcludedPrefixes(v.GetPrefixes()); err != nil {
			logrus.Error(err)
			return
		}
	}

	data, err := utils.PrefixesToYaml(excludePrefixPool.GetPrefixes())
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
		return
	}

	err = ioutil.WriteFile(epc.outputFilePath, data, 0600)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}
}
