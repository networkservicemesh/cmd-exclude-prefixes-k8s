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

// Package prefixcollector contains excluded prefix collector and functions working with it
package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"

	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"

	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

const defaultPrefixesFilePath = "/var/lib/networkservicemesh/config/excluded_prefixes.yaml"

// PrefixSource is source of excluded prefixes
type PrefixSource interface {
	Prefixes() []string
}

// writePrefixesFunc is excluded prefixes write func
type writePrefixesFunc func(context.Context, []string)

// watchPrefixesFunc is excluded prefixes resource watch func
type watchPrefixesFunc func(context.Context, *utils.SynchronizedPrefixesContainer)

// Option is ExcludedPrefixCollector option
type Option func(collector *ExcludedPrefixCollector)

// ExcludedPrefixCollector is service, collecting excluded prefixes from list of PrefixSource
// and writing result using provided writePrefixesFunc
type ExcludedPrefixCollector struct {
	notifyChan       <-chan struct{}
	writeFunc        writePrefixesFunc
	watchFunc        watchPrefixesFunc
	sources          []PrefixSource
	previousPrefixes *utils.SynchronizedPrefixesContainer
}

// WithFileOutput is ExcludedPrefixCollector option, which sets file output
func WithFileOutput(outputFilePath string) Option {
	return func(collector *ExcludedPrefixCollector) {
		collector.writeFunc = fileWriter(outputFilePath)
		collector.watchFunc = nil
	}
}

// WithConfigMapOutput is ExcludedPrefixCollector option, which sets configMap output
func WithConfigMapOutput(name, namespace string) Option {
	return func(collector *ExcludedPrefixCollector) {
		collector.writeFunc = configMapWriter(name, namespace)
		collector.watchFunc = configMapWatchFunc(name, namespace)
	}
}

// WithNotifyChan is ExcludedPrefixCollector option, which sets notify chan for collector
func WithNotifyChan(notifyChan <-chan struct{}) Option {
	return func(collector *ExcludedPrefixCollector) {
		collector.notifyChan = notifyChan
	}
}

// WithSources is ExcludedPrefixCollector option, which sets prefix sources
func WithSources(sources ...PrefixSource) Option {
	return func(collector *ExcludedPrefixCollector) {
		collector.sources = sources
	}
}

// NewExcludePrefixCollector creates ExcludedPrefixCollector
func NewExcludePrefixCollector(options ...Option) *ExcludedPrefixCollector {
	collector := &ExcludedPrefixCollector{
		notifyChan:       make(chan struct{}, 1),
		previousPrefixes: utils.NewSynchronizedPrefixesContainer(),
		writeFunc:        fileWriter(defaultPrefixesFilePath),
	}

	for _, option := range options {
		option(collector)
	}

	return collector
}

// Serve - begin monitoring sources.
// Updates exclude prefix file after every notification.
func (epc *ExcludedPrefixCollector) Serve(ctx context.Context) {
	if epc.watchFunc != nil {
		go epc.watchFunc(ctx, epc.previousPrefixes)
	}

	// check current state of sources
	epc.updateExcludedPrefixes(ctx)
	for {
		select {
		case <-epc.notifyChan:
			epc.updateExcludedPrefixes(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (epc *ExcludedPrefixCollector) updateExcludedPrefixes(ctx context.Context) {
	excludePrefixPool, _ := prefixpool.New()

	for _, v := range epc.sources {
		sourcePrefixes := v.Prefixes()
		if len(sourcePrefixes) == 0 {
			continue
		}

		if err := excludePrefixPool.ReleaseExcludedPrefixes(v.Prefixes()); err != nil {
			logrus.Error(err)
			return
		}
	}

	newPrefixes := excludePrefixPool.GetPrefixes()
	if utils.UnorderedSlicesEquals(newPrefixes, epc.previousPrefixes.Load()) {
		return
	}

	span := spanhelper.FromContext(ctx, "Update excluded prefixes")

	epc.previousPrefixes.Store(newPrefixes)
	epc.writeFunc(ctx, newPrefixes)
	span.Logger().Infof("Excluded prefixes were successfully updated: %v", newPrefixes)
}
