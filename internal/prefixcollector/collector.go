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
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/ghodss/yaml"

	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
)

const (
	configMapKey = "excluded_prefixes.yaml"
)

// ExcludePrefixSource is source of excluded prefixes
type ExcludePrefixSource interface {
	Prefixes() []string
}

// Notifier is entity used for listeners notification
type Notifier interface {
	Broadcast()
}

// ExcludePrefixCollector is service, collecting excluded prefixes from list of ExcludePrefixSource
// and writing result to outputFilePath in yaml format
type ExcludePrefixCollector struct {
	notify                *sync.Cond
	nsmConfigMapName      string
	nsmConfigMapNamespace string
	sources               []ExcludePrefixSource
	previousPrefixes      []string
}

// NewExcludePrefixCollector creates ExcludePrefixCollector
func NewExcludePrefixCollector(notify *sync.Cond, configMapName, configMapNamespace string,
	sources ...ExcludePrefixSource) *ExcludePrefixCollector {

	return &ExcludePrefixCollector{
		nsmConfigMapName:      configMapName,
		nsmConfigMapNamespace: configMapNamespace,
		notify:                notify,
		sources:               sources,
	}
}

// Serve - begin monitoring sources.
// Updates exclude prefix file after every notification.
func (epc *ExcludePrefixCollector) Serve(ctx context.Context) {
	go func() {
		<-ctx.Done()
		epc.notify.Broadcast()
	}()
	configMapInterface := KubernetesInterface(ctx).CoreV1().ConfigMaps(epc.nsmConfigMapNamespace)
	// check current state of sources
	epc.updateExcludedPrefixesConfigmap(ctx, configMapInterface)

	for ctx.Err() == nil {
		epc.notify.L.Lock()
		epc.notify.Wait()
		epc.notify.L.Unlock()
		epc.updateExcludedPrefixesConfigmap(ctx, configMapInterface)
	}
}

func (epc *ExcludePrefixCollector) updateExcludedPrefixesConfigmap(ctx context.Context, configMapInterface v1.ConfigMapInterface) {
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
	if utils.UnorderedSlicesEquals(newPrefixes, epc.previousPrefixes) {
		return
	}
	epc.previousPrefixes = newPrefixes

	data, err := prefixesToYaml(newPrefixes)
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
		return
	}

	configMap, err := configMapInterface.Get(ctx, epc.nsmConfigMapName, metav1.GetOptions{})
	if err != nil {
		logrus.Fatalf("Failed to get NSM ConfigMap '%s/%s': %v", epc.nsmConfigMapNamespace, epc.nsmConfigMapName, err)
		return
	}
	configMap.Data[configMapKey] = string(data)

	_, err = configMapInterface.Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		logrus.Errorf("Failed to update NSM ConfigMap '%s/%s': %v", epc.nsmConfigMapNamespace, epc.nsmConfigMapName, err)
	}
	logrus.Infof("Excluded prefixes were successfully updated: %v", newPrefixes)
}

// prefixesToYaml converts list of prefixes to yaml file
func prefixesToYaml(prefixesList []string) ([]byte, error) {
	source := struct {
		Prefixes []string
	}{prefixesList}

	bytes, err := yaml.Marshal(source)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}
