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

// Package containing different util functions for excluded prefix collector and prefix sources
package utils

import (
	"context"
	"github.com/ghodss/yaml"
	"k8s.io/client-go/kubernetes"
	"os"
)

type clientSetKeyType string

// ClientSet key in context map
const ClientSetKey clientSetKeyType = "clientsetKey"

// Struct containing prefixes list
type Prefixes struct {
	PrefixesList []string `json:"Prefixes"`
}

// Returns ClientSet from context ctx
func FromContext(ctx context.Context) kubernetes.Interface {
	return ctx.Value(ClientSetKey).(kubernetes.Interface)
}

// Converts list of prefixes to yaml file
func PrefixesToYaml(prefixesList []string) ([]byte, error) {
	source := Prefixes{prefixesList}

	bytes, err := yaml.Marshal(source)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// Converts yaml file to Prefixes
func YamlToPrefixes(bytes []byte) (Prefixes, error) {
	destination := Prefixes{}
	err := yaml.Unmarshal(bytes, &destination)
	if err != nil {
		return Prefixes{}, err
	}

	return destination, nil
}

// Sends empty struct to notifyChan
func Notify(notifyChan chan<- struct{}) {
	notifyChan <- struct{}{}
}

// Creates directory path if it doesn't exist
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
