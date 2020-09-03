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

// YamlToPrefixes converts yaml file to slice of prefixes
func YamlToPrefixes(bytes []byte) ([]string, error) {
	destination := struct {
		Prefixes []string
	}{}
	err := yaml.Unmarshal(bytes, &destination)
	if err != nil {
		return destination.Prefixes, err
	}

	return destination.Prefixes, nil
}

// UnorderedSlicesEquals returns true if specified slice x is equal to specified slice y
// ignoring the order of the elements
func UnorderedSlicesEquals(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	diff := make(map[string]int, len(x))
	for _, xValue := range x {
		diff[xValue]++
	}
	for _, yValue := range y {
		if _, ok := diff[yValue]; !ok {
			return false
		}
		diff[yValue]--
		if diff[yValue] == 0 {
			delete(diff, yValue)
		}
	}
	return len(diff) == 0
}

// GetValidatedPrefixes returns list of validated via CIDR notation parsing prefixes
func GetValidatedPrefixes(prefixes []string) ([]string, error) {
	var validatedPrefixes []string
	for _, prefix := range prefixes {
		_, _, err := net.ParseCIDR(prefix)
		if err != nil {
			return nil, err
		}
		validatedPrefixes = append(validatedPrefixes, prefix)
	}

	return validatedPrefixes, nil
}
