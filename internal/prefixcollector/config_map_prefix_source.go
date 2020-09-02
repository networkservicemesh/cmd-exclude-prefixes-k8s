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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const configMapPrefixesKey = "excluded_prefixes.yaml"

// ConfigMapPrefixSource is Kubernetes ConfigMap excluded prefix source
type ConfigMapPrefixSource struct {
	configMapName      string
	configMapNameSpace string
	configMapInterface v1.ConfigMapInterface
	prefixes           utils.SynchronizedPrefixesContainer
	ctx                context.Context
	notify             Notifier
}

// NewConfigMapPrefixSource creates ConfigMapPrefixSource
func NewConfigMapPrefixSource(ctx context.Context, notify Notifier, name, namespace string) *ConfigMapPrefixSource {
	clientSet := FromContext(ctx)
	configMapInterface := clientSet.CoreV1().ConfigMaps(namespace)
	cmps := ConfigMapPrefixSource{
		configMapName:      name,
		configMapNameSpace: namespace,
		configMapInterface: configMapInterface,
		ctx:                ctx,
		notify:             notify,
	}

	go cmps.watchConfigMap()
	return &cmps
}

// Prefixes returns prefixes from source
func (cmps *ConfigMapPrefixSource) Prefixes() []string {
	return cmps.prefixes.GetList()
}

func (cmps *ConfigMapPrefixSource) watchConfigMap() {
	cmps.checkCurrentConfigMap()
	configMapWatch, err := cmps.configMapInterface.Watch(cmps.ctx, metav1.ListOptions{})
	if err != nil {
		logrus.Errorf("Error creating config map watch: %v", err)
		return
	}

	for cmps.ctx.Err() == nil {
		select {
		case event, ok := <-configMapWatch.ResultChan():
			if !ok {
				return
			}

			if event.Type == watch.Error {
				continue
			}

			configMap, ok := event.Object.(*apiV1.ConfigMap)
			if !ok || configMap.Name != cmps.configMapName {
				continue
			}

			if event.Type == watch.Deleted {
				cmps.prefixes.SetList([]string{})
				cmps.notify.Broadcast()
				continue
			}

			if err = cmps.setPrefixesFromConfigMap(configMap); err != nil {
				logrus.Error(err)
			}
		case <-cmps.ctx.Done():
		}
	}
}

func (cmps *ConfigMapPrefixSource) checkCurrentConfigMap() {
	configMap, err := cmps.configMapInterface.Get(cmps.ctx, cmps.configMapName, metav1.GetOptions{})
	if err != nil {
		logrus.Errorf("Error getting config map : %v", err)
		return
	}

	if err = cmps.setPrefixesFromConfigMap(configMap); err != nil {
		logrus.Error(err)
	}
}

func (cmps *ConfigMapPrefixSource) setPrefixesFromConfigMap(configMap *apiV1.ConfigMap) error {
	prefixesField, ok := configMap.Data[configMapPrefixesKey]
	if !ok {
		return nil
	}

	prefixes, err := utils.YamlToPrefixes([]byte(prefixesField))
	if err != nil {
		return errors.Errorf("Can not unmarshal prefixes, err: %v", err.Error())
	}
	cmps.prefixes.SetList(prefixes.PrefixesList)
	cmps.notify.Broadcast()
	logrus.Infof("Prefixes sent from config map source: %v", prefixes.PrefixesList)

	return nil
}
