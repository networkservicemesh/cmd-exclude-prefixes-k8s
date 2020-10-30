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

	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

// PrefixSource is source of excluded prefixes
type PrefixSource interface {
	Prefixes() []string
}

// PrefixesWriter is excluded prefixes writer
type PrefixesWriter interface {
	Write(context.Context, []string)
	WatchExcludedPrefixes(context.Context, *utils.SynchronizedPrefixesContainer)
}

// ExcludePrefixCollector is service, collecting excluded prefixes from list of PrefixSource
// and writing result to outputFilePath in yaml format
type ExcludePrefixCollector struct {
	notifyChan       <-chan struct{}
	writer           PrefixesWriter
	sources          []PrefixSource
	previousPrefixes *utils.SynchronizedPrefixesContainer
}

// NewExcludePrefixCollector creates ExcludePrefixCollector
func NewExcludePrefixCollector(notifyChan <-chan struct{},
	prefixWriter PrefixesWriter, sources ...PrefixSource) *ExcludePrefixCollector {
	collector := &ExcludePrefixCollector{
		notifyChan:       notifyChan,
		previousPrefixes: utils.NewSynchronizedPrefixesContainer(),
		writer:           prefixWriter,
		sources:          sources,
	}

	return collector
}

// Serve - begin monitoring sources.
// Updates exclude prefix file after every notification.
func (epc *ExcludePrefixCollector) Serve(ctx context.Context) {
	go epc.writer.WatchExcludedPrefixes(ctx, epc.previousPrefixes)

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

func (epc *ExcludePrefixCollector) updateExcludedPrefixes(ctx context.Context) {
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

	epc.previousPrefixes.Store(newPrefixes)
	epc.writer.Write(ctx, newPrefixes)
}
