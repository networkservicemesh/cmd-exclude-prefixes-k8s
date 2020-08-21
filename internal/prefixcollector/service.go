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
	"context"
	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strings"
)

type contextKeyType string

type PrefixSource interface {
	GetNotifyChannel() <-chan struct{}
	GetPrefixes() []string
}

const (
	ClientsetKey        contextKeyType = "clientsetKey"
	excludedPrefixesEnv                = "EXCLUDED_PREFIXES"
)

type PrefixCollectorService struct {
	prefixes []string
	filePath string
}

// todo pattern option - массив опций
func NewPrefixCollectorService(filePath string) *PrefixCollectorService {
	return &PrefixCollectorService{
		getPrefixesFromEnv(),
		filePath,
	}
}

func GetDefaultPrefixWatchers(context context.Context) ([]<-chan []string, error) {
	kubernetesWatcher, err := NewKubernetesPrefixSource(context)
	if err != nil {
		logrus.Error("Error creating KubernetesPrefixSource")
		return nil, err
	}

	configMapWatcher, err := NewConfigMapPrefixSource(context, "nsm-config-volume", "default")
	if err != nil {
		logrus.Error("Error creating ConfigMapPrefixSource")
		return nil, err
	}

	return []<-chan []string{
		kubernetesWatcher.ResultChan(),
		configMapWatcher.ResultChan(),
	}, nil
}

func getPrefixesFromEnv() []string {
	var envPrefixes []string
	excludedPrefixesEnv, ok := os.LookupEnv(excludedPrefixesEnv)
	if ok {
		envPrefixes = strings.Split(excludedPrefixesEnv, ",")
		prefixes := make([]string, len(envPrefixes))
		copy(prefixes, envPrefixes)
		return prefixes
	}

	return make([]string, 1)
}

func (pcs *PrefixCollectorService) Start(channel <-chan struct{}) {
	go func() {
		for _ := range channel {
			updateExcludedPrefixesConfigmap(pcs.filePath, prefixes)
		}
	}()
}

func updateExcludedPrefixesConfigmap(filePath string, prefixSources []PrefixSource) {
	var prefixPool prefixpool.PrefixPool
	var prefixes []string
	var err error

	for _, v := range prefixSources {
		prefixes, err = prefixPool.ExcludePrefixes(v.GetPrefixes())
		if err != nil {
			logrus.Errorf("", err)
			return
		}
	}

	data, err := PrefixesToYaml(prefixes)
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
	}

	err = ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}
}
