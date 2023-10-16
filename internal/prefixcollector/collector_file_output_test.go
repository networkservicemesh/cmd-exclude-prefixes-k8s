// Copyright (c) 2020-2022 Doc.ai and/or its affiliates.
//
// Copyright (c) 2023 Cisco and/or its affiliates.
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
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"go.uber.org/goleak"
)

const (
	prefixesFileName = "excluded_prefixes.yaml"
)

func (eps *ExcludedPrefixesSuite) TestAllSourcesWithFileOutput() {
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
			notifyChan,
		),
		prefixsource.NewKubeAdmPrefixSource(ctx, notifyChan),
		prefixsource.NewConfigMapPrefixSource(
			ctx,
			notifyChan,
			userConfigMapName,
			configMapNamespace,
			userConfigMapKey,
		),
	}

	eps.testCollectorWithFileOutput(ctx, notifyChan, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) TestDummySourceWithSeveralNotifications() {
	defer goleak.VerifyNone(eps.T(), goleak.IgnoreCurrent())
	expectedResult := []string{
		"127.0.0.0/16",
		"168.92.0.0/24",
	}

	notificationCount := 5

	notifyChan := make(chan struct{}, notificationCount)
	ctx, cancel := context.WithCancel(prefixcollector.WithKubernetesInterface(context.Background(), eps.clientSet))
	defer cancel()

	eps.createConfigMap(ctx, prefixsource.KubeNamespace, kubeConfigMapPath)
	eps.createConfigMap(ctx, configMapNamespace, configMapPath)

	source := newDummyPrefixSource(
		[]string{
			"127.0.0.1/16",
			"127.0.2.1/16",
			"168.92.0.1/24",
		},
		notifyChan,
	)

	sources := []prefixcollector.PrefixSource{source}

	for i := 0; i < notificationCount; i++ {
		source.SendNotification()
	}

	eps.testCollectorWithFileOutput(ctx, notifyChan, expectedResult, sources)
}

func (eps *ExcludedPrefixesSuite) testCollectorWithFileOutput(ctx context.Context, notifyChan chan struct{},
	expectedResult []string, sources []prefixcollector.PrefixSource) {
	prefixesFilePath := filepath.Join(os.TempDir(), prefixesFileName)
	_, err := os.Create(filepath.Clean(prefixesFilePath))
	eps.Require().NoError(err)

	collector := prefixcollector.NewExcludePrefixCollector(
		prefixcollector.WithNotifyChan(notifyChan),
		prefixcollector.WithFileOutput(prefixesFilePath),
		prefixcollector.WithSources(sources...),
	)

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	var modified atomic.Int32
	watcher, errCh := eps.watchFile(ctx, prefixesFilePath, &modified)

	go collector.Serve(ctx)

	if watchErr := <-errCh; watchErr != nil {
		eps.T().Fatalf("Error watching file: %v", watchErr)
	}

	if watchErr := watcher.Close(); watchErr != nil {
		eps.T().Errorf("Error closing watcher: %v", watchErr)
	}

	bytes, err := os.ReadFile(filepath.Clean(prefixesFilePath))
	if err != nil {
		eps.T().Fatalf("Error reading file: %v", err)
	}

	prefixes, err := utils.YamlToPrefixes(bytes)
	if err != nil {
		eps.T().Fatalf("Error transforming yaml to prefixes: %v", err)
	}

	eps.Require().LessOrEqual(int(modified.Load()), len(sources)*2)
	eps.Require().ElementsMatch(expectedResult, prefixes)
}

func (eps *ExcludedPrefixesSuite) watchFile(ctx context.Context, prefixesFilePath string,
	modified *atomic.Int32) (watcher *fsnotify.Watcher, errorCh chan error) {
	watcher, err := fsnotify.NewWatcher()
	errorCh = make(chan error)

	if err != nil {
		errorCh <- err
		return watcher, errorCh
	}

	err = watcher.Add(prefixesFilePath)
	if err != nil {
		errorCh <- err
		return watcher, errorCh
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					close(errorCh)
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					modified.Add(1)
				}
			case watcherError, ok := <-watcher.Errors:
				if !ok {
					errorCh <- watcherError
					return
				}
			case <-ctx.Done():
				close(errorCh)
				return
			}
		}
	}()

	return watcher, errorCh
}
