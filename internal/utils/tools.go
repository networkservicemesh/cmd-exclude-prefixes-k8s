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

// Package utils contains different util functions for excluded prefix collector and prefix sources
package utils

import (
	"context"
	"os"

	"github.com/ghodss/yaml"
	"k8s.io/client-go/kubernetes"
)

type clientSetKeyType string

// ClientSetKey is ClientSet key in context map
const ClientSetKey clientSetKeyType = "clientsetKey"

// Prefixes is struct containing prefixes list
type Prefixes struct {
	PrefixesList []string `json:"Prefixes"`
}

// FromContext returns ClientSet from context ctx
func FromContext(ctx context.Context) kubernetes.Interface {
	return ctx.Value(ClientSetKey).(kubernetes.Interface)
}

// PrefixesToYaml converts list of prefixes to yaml file
func PrefixesToYaml(prefixesList []string) ([]byte, error) {
	source := Prefixes{prefixesList}

	bytes, err := yaml.Marshal(source)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// YamlToPrefixes converts yaml file to Prefixes
func YamlToPrefixes(bytes []byte) (Prefixes, error) {
	destination := Prefixes{}
	err := yaml.Unmarshal(bytes, &destination)
	if err != nil {
		return Prefixes{}, err
	}

	return destination, nil
}

// Notify sends empty struct to notifyChan
func Notify(notifyChan chan<- struct{}) {
	notifyChan <- struct{}{}
}

// CreateDirIfNotExists creates directory named path if it doesn't exist
func CreateDirIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err == nil {
			return nil
		}
		if err := os.MkdirAll(path, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}
