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

type ExcludePrefixSource interface {
	GetNotifyChannel() <-chan struct{}
	GetPrefixes() []string
}

const (
	ClientsetKey        contextKeyType = "clientsetKey"
	excludedPrefixesEnv                = "EXCLUDED_PREFIXES"
)

type PrefixCollectorService struct {
	prefixPool *prefixpool.PrefixPool
	filePath   string
	sources    []ExcludePrefixSource
}

// todo pattern option - массив опций
func NewPrefixCollectorService(context context.Context, filePath string) (*PrefixCollectorService, error) {
	prefixPool, err := prefixpool.NewPrefixPool(getPrefixesFromEnv()...)
	if err != nil {
		return nil, err
	}
	return &PrefixCollectorService{
		prefixPool,
		filePath,
		GetDefaultPrefixSources(context),
	}, nil
}

func GetDefaultPrefixSources(context context.Context) []ExcludePrefixSource {
	// TODO
	sources := make([]ExcludePrefixSource, 0, 5)
	kubernetesWatcher, err := NewKubernetesPrefixSource(context)
	if err == nil {
		sources = append(sources, kubernetesWatcher)
	}

	configMapWatcher, err := NewConfigMapPrefixSource(context, "nsm-config-volume", "default")
	if err == nil {
		sources = append(sources, configMapWatcher)
	}

	kubeAdmPrefixSource, err := NewKubeAdmPrefixSource(context)
	if err == nil {
		sources = append(sources, kubeAdmPrefixSource)
	}

	return sources
}

func getPrefixesFromEnv() []string {
	var envPrefixes []string
	excludedPrefixesEnv, ok := os.LookupEnv(excludedPrefixesEnv)
	if ok {
		return strings.Split(excludedPrefixesEnv, ",")
	}

	return envPrefixes
}

func (pcs *PrefixCollectorService) Start(channel <-chan struct{}) {
	go func() {
		for _ = range channel {
			pcs.updateExcludedPrefixesConfigmap(pcs.filePath)
		}
	}()
}

func (pcs *PrefixCollectorService) updateExcludedPrefixesConfigmap(filePath string) {
	var prefixes []string
	var err error

	for _, v := range pcs.sources {
		tmp := v.GetPrefixes()
		if len(tmp) == 0 {
			logrus.Info("EMPTY")
			continue
		}
		_ = pcs.prefixPool.ReleaseExcludedPrefixes(v.GetPrefixes())
		prefixes, err = pcs.prefixPool.ExcludePrefixes(v.GetPrefixes())
		if err != nil {
			logrus.Errorf("", err)
			//return
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
