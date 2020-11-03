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
	"io/ioutil"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	configMapKey          = "excluded_prefixes.yaml"
	outputFilePermissions = 0600
)

// fileWriter - creates file writePrefixesFunc
func fileWriter(filePath string) writePrefixesFunc {
	return func(ctx context.Context, newPrefixes []string) {
		span := spanhelper.FromContext(ctx, "Update excluded prefixes file")
		defer span.Finish()

		data, err := utils.PrefixesToYaml(newPrefixes)
		if err != nil {
			span.Logger().Errorf("Can not create marshal prefixes, err: %v", err.Error())
			return
		}

		err = ioutil.WriteFile(filePath, data, outputFilePermissions)
		if err != nil {
			span.Logger().Fatalf("Unable to write into file: %v", err.Error())
		}
	}
}

// configMapWriter - creates k8s config map writePrefixesFunc
func configMapWriter(configMapName, configMapNamespace string) writePrefixesFunc {
	return func(ctx context.Context, newPrefixes []string) {
		configMapInterface := KubernetesInterface(ctx).
			CoreV1().
			ConfigMaps(configMapNamespace)

		span := spanhelper.FromContext(ctx, "Update excluded prefixes config map")
		defer span.Finish()

		configMap, err := configMapInterface.Get(ctx, configMapName, metav1.GetOptions{})
		if err != nil {
			span.Logger().Fatalf("Failed to get NSM ConfigMap '%s/%s': %v",
				configMapNamespace, configMapName, err)
			return
		}

		if err := updateConfigMap(ctx, newPrefixes, configMap, configMapInterface); err != nil {
			span.Logger().Error(err)
		}
	}
}

// configMapWatchFunc - creates watchPrefixesFunc, that keep track of prefixes k8s config map external changes
func configMapWatchFunc(configMapName, configMapNamespace string) watchPrefixesFunc {
	return func(ctx context.Context, previousPrefixes *utils.SynchronizedPrefixesContainer) {
		configMapInterface := KubernetesInterface(ctx).
			CoreV1().
			ConfigMaps(configMapNamespace)

		span := spanhelper.FromContext(ctx, "Watch NSM config map")
		defer span.Finish()

		watcher, err := configMapInterface.Watch(ctx, metav1.ListOptions{})
		if err != nil {
			span.Logger().Fatalf("Error watching config map: %v", err)
		}

		logEntry := span.Logger().WithFields(logrus.Fields{
			"configmap-namespace": configMapNamespace,
			"configmap-name":      configMapName,
		})

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.ResultChan():
				if !ok {
					return
				}

				configMap, ok := event.Object.(*apiV1.ConfigMap)
				if !ok || configMap.Name != configMapName {
					continue
				}

				switch event.Type {
				case watch.Error:
					logEntry.Errorf("Error during nsm configmap watch: %v", err)
					return
				case watch.Modified:
					prefixes, err := utils.YamlToPrefixes([]byte(configMap.Data[configMapKey]))
					if err != nil || !utils.UnorderedSlicesEquals(prefixes, previousPrefixes.Load()) {
						logEntry.Warn("Nsm configmap excluded prefixes field external change, restoring last state")
						if err := updateConfigMap(ctx, previousPrefixes.Load(), configMap, configMapInterface); err != nil {
							span.Logger().Error(err)
						}
					}
				}
			}
		}
	}
}

func updateConfigMap(ctx context.Context, newPrefixes []string,
	configMap *apiV1.ConfigMap, configMapInterface v1.ConfigMapInterface) error {
	data, err := utils.PrefixesToYaml(newPrefixes)
	if err != nil {
		return errors.Wrapf(err, "Can not create marshal prefixes")
	}
	configMap.Data[configMapKey] = string(data)

	_, err = configMapInterface.Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to update NSM ConfigMap")
	}

	return nil
}
