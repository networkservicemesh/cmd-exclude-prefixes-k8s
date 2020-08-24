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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"time"
)

type ConfigMapPrefixSource struct {
	configMapName      string
	configMapNameSpace string
	configMapInterface v1.ConfigMapInterface
	prefixes           utils.SynchronizedPrefixList
	notifyChan         chan struct{}
}

func NewConfigMapPrefixSource(ctx context.Context, name, namespace string) *ConfigMapPrefixSource {
	clientSet := utils.FromContext(ctx)
	configMapInterface := clientSet.CoreV1().ConfigMaps(namespace)
	cmps := ConfigMapPrefixSource{
		name,
		namespace,
		configMapInterface,
		utils.NewSynchronizedPrefixListImpl(),
		make(chan struct{}, 1),
	}

	go cmps.watchConfigMap(ctx)

	return &cmps
}

func (cmps *ConfigMapPrefixSource) GetNotifyChannel() <-chan struct{} {
	return cmps.notifyChan
}

func (cmps *ConfigMapPrefixSource) GetPrefixes() []string {
	return cmps.prefixes.GetList()
}

func (cmps *ConfigMapPrefixSource) watchConfigMap(ctx context.Context) {
	for {
		cm, err := cmps.configMapInterface.Get(ctx, cmps.configMapName, metav1.GetOptions{})
		if err != nil {
			logrus.Errorf("Failed to get ConfigMap '%s/%s': %v", cmps.configMapNameSpace, cmps.configMapName, err)
			return
		}

		bytes := []byte(cm.Data[prefixpool.PrefixesFile])
		prefixes, err := utils.YamlToPrefixes(bytes)
		if err != nil {
			logrus.Errorf("Can not unmarshal prefixes, err: %v", err.Error())
			return
		}
		cmps.prefixes.SetList(prefixes.PrefixesList)
		utils.Notify(cmps.notifyChan)
		<-time.After(time.Second * 10)
	}
}
