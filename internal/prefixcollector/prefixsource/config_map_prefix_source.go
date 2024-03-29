// Copyright (c) 2020-2021 Doc.ai and/or its affiliates.
//
// Copyright (c) 2022-2024 Cisco and/or its affiliates.
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

// Package prefixsource contains excluded prefix sources
package prefixsource

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"

	"github.com/pkg/errors"
	apiV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

// ConfigMapPrefixSource is Kubernetes ConfigMap excluded prefix source
type ConfigMapPrefixSource struct {
	configMapName      string
	configMapNameSpace string
	configMapKey       string
	configMapInterface v1.ConfigMapInterface
	prefixes           *utils.SynchronizedPrefixesContainer
	ctx                context.Context
	notify             chan<- struct{}
}

// NewConfigMapPrefixSource creates ConfigMapPrefixSource
func NewConfigMapPrefixSource(ctx context.Context, notify chan<- struct{}, name, namespace, configMapKey string) *ConfigMapPrefixSource {
	clientSet := prefixcollector.KubernetesInterface(ctx)
	configMapInterface := clientSet.CoreV1().ConfigMaps(namespace)
	cmps := ConfigMapPrefixSource{
		configMapName:      name,
		configMapNameSpace: namespace,
		configMapKey:       configMapKey,
		configMapInterface: configMapInterface,
		ctx:                ctx,
		notify:             notify,
		prefixes:           utils.NewSynchronizedPrefixesContainer(),
	}

	go func() {
		for cmps.ctx.Err() == nil {
			cmps.watchConfigMap()
		}
	}()
	return &cmps
}

// Prefixes returns prefixes from source
func (cmps *ConfigMapPrefixSource) Prefixes() []string {
	return cmps.prefixes.Load()
}

func (cmps *ConfigMapPrefixSource) watchConfigMap() {
	configMapWatch, err := cmps.configMapInterface.Watch(cmps.ctx, metav1.ListOptions{})
	if err != nil {
		log.FromContext(cmps.ctx).Errorf("Error creating config map watch: %v", err)
		return
	}
	defer configMapWatch.Stop()

	// we should check current state after we create the watcher,
	// or else we could miss an update
	cmps.checkCurrentConfigMap()

	log.FromContext(cmps.ctx).Info("Starting watching configmaps")

	for {
		select {
		case <-cmps.ctx.Done():
			log.FromContext(cmps.ctx).Warn("Configmaps watcher context is canceled")
			return
		case event, ok := <-configMapWatch.ResultChan():
			if !ok {
				log.FromContext(cmps.ctx).Warn("Configmaps watcher is closed")
				return
			}

			log.FromContext(cmps.ctx).Tracef("Config map event received: %v", event)

			if event.Type == watch.Error {
				continue
			}

			configMap, ok := event.Object.(*apiV1.ConfigMap)
			if !ok || configMap.Name != cmps.configMapName {
				continue
			}

			if event.Type == watch.Deleted {
				cmps.prefixes.Store([]string(nil))
				cmps.notify <- struct{}{}
				log.FromContext(cmps.ctx).Info("Prefixes from config map deleted")
				continue
			}

			if err = cmps.setPrefixesFromConfigMap(configMap); err != nil {
				log.FromContext(cmps.ctx).Errorf("Error setting prefixes from config map: %s", configMap.Name)
			}
		}
	}
}

func (cmps *ConfigMapPrefixSource) checkCurrentConfigMap() {
	configMap, err := cmps.configMapInterface.Get(cmps.ctx, cmps.configMapName, metav1.GetOptions{})
	if err != nil {
		log.FromContext(cmps.ctx).Errorf("Error getting config map: %v", err)
		return
	}

	if err = cmps.setPrefixesFromConfigMap(configMap); err != nil {
		log.FromContext(cmps.ctx).Errorf("Error setting prefixes from config map: %s", configMap.Name)
	}
}

func (cmps *ConfigMapPrefixSource) setPrefixesFromConfigMap(configMap *apiV1.ConfigMap) error {
	prefixesField, ok := configMap.Data[cmps.configMapKey]
	if !ok {
		return nil
	}

	prefixes, err := utils.YamlToPrefixes([]byte(prefixesField))
	if err != nil {
		return errors.Errorf("Can not unmarshal prefixes, err: %v", err.Error())
	}
	cmps.prefixes.Store(prefixes)
	cmps.notify <- struct{}{}
	log.FromContext(cmps.ctx).Infof("Prefixes sent from config map source: %v", prefixes)

	return nil
}
