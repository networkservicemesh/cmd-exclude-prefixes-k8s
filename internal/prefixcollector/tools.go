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
	"github.com/ghodss/yaml"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"
)

type Prefixes struct {
	PrefixesList []string `json:"Prefixes"`
}

func GetExcludePrefixChan(ctx context.Context, options ...func(context.Context) ([]string, error)) <-chan []string {
	ch := make(chan []string)
	set := map[string]bool{}

	go func() {
		for {
			var prefixes []string

			for _, o := range options {
				if newPrefixes, err := o(ctx); err == nil {
					prefixes = append(prefixes, newPrefixes...)
				}
			}

			prefixes = deleteDuplicate(prefixes)
			if hasChanges(prefixes, set) {
				set = make(map[string]bool)
				for _, v := range prefixes {
					set[v] = true
				}
				ch <- prefixes
			}
			<-time.After(time.Second)
		}
	}()

	return ch
}

func GetNotifyChannel(sources ...ExcludePrefixSource) <-chan struct{} {
	channels := make([]<-chan struct{}, len(sources))
	for _, v := range sources {
		channels = append(channels, v.GetNotifyChannel())
	}
	return mergeNotifyChannels(channels...)
}

func mergeNotifyChannels(channels ...<-chan struct{}) <-chan struct{} {
	out := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(channels))
	output := func(c <-chan struct{}) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}

	for _, c := range channels {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func deleteDuplicate(prefixes []string) []string {
	encountered := map[string]bool{}
	result := []string{}

	for index := range prefixes {
		if encountered[prefixes[index]] {
			continue
		}
		encountered[prefixes[index]] = true
		result = append(result, prefixes[index])
	}
	return result
}

func hasChanges(new []string, old map[string]bool) bool {
	if len(new) != len(old) {
		return true
	}
	for _, v := range new {
		if old[v] == false {
			return true
		}
	}
	return false
}

func FromContext(ctx context.Context) *kubernetes.Clientset {
	return ctx.Value(ClientsetKey).(*kubernetes.Clientset)
}

func PrefixesToYaml(prefixesList []string) ([]byte, error) {
	source := Prefixes{prefixesList}

	bytes, err := yaml.Marshal(source)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

func YamlToPrefixes(bytes []byte) (Prefixes, error) {
	destination := Prefixes{}
	err := yaml.Unmarshal(bytes, &destination)
	if err != nil {
		return Prefixes{}, err
	}

	return destination, nil
}
