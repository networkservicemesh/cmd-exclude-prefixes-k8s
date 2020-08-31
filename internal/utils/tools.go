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
	"net"

	"github.com/ghodss/yaml"
)

// Prefixes is struct containing prefixes list
type Prefixes struct {
	PrefixesList []string `json:"Prefixes"`
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

// GetValidatedPrefixes returns list of validated via CIDR notation parsing prefixes
func GetValidatedPrefixes(prefixes []string) []string {
	validatedPrefixes := make([]string, 0, len(prefixes))
	for _, prefix := range prefixes {
		_, _, err := net.ParseCIDR(prefix)
		if err == nil {
			validatedPrefixes = append(validatedPrefixes, prefix)
		}
	}

	return validatedPrefixes
}
