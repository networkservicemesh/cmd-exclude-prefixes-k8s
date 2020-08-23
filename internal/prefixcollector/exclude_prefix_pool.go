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
	"github.com/sirupsen/logrus"
	"net"
	"sync"
)

type ExcludePrefixPool struct {
	lock     sync.RWMutex
	prefixes []string
}

func NewExcludePrefixPool(prefixes ...string) (*ExcludePrefixPool, error) {
	for _, prefix := range prefixes {
		_, _, err := net.ParseCIDR(prefix)
		if err != nil {
			return nil, err
		}
	}
	return &ExcludePrefixPool{
		prefixes: prefixes,
	}, nil
}

func (impl *ExcludePrefixPool) Add(prefixesToAdd []string) error {
	impl.lock.Lock()
	newPrefixes := make([]string, 0, len(impl.prefixes)+len(prefixesToAdd))
	prefixesToRemove := make(map[string]struct{})

	for _, prefixToAdd := range prefixesToAdd {
		intersected := false
		_, prefixToAddSubnet, err := net.ParseCIDR(prefixToAdd)
		if err != nil {
			logrus.Errorf("Wrong CIDR: %v", prefixesToAdd)
			return err
		}
		for _, prefix := range impl.prefixes {
			_, prefixSubnet, _ := net.ParseCIDR(prefix)
			if intersect, firstIsWider := intersect(prefixSubnet, prefixToAddSubnet); intersect == true {
				intersected = true
				if !firstIsWider {
					newPrefixes = append(newPrefixes, prefixToAdd)
					prefixesToRemove[prefix] = struct{}{}
				}
			}
		}
		if !intersected {
			newPrefixes = append(newPrefixes, prefixToAdd)
		}
	}

	for _, prefix := range impl.prefixes {
		if _, ok := prefixesToRemove[prefix]; ok {
			continue
		}

		newPrefixes = append(newPrefixes, prefix)
	}

	impl.prefixes = newPrefixes
	impl.lock.Unlock()

	return nil
}

func (impl *ExcludePrefixPool) GetPrefixes() []string {
	impl.lock.Lock()
	copyArray := make([]string, len(impl.prefixes))
	copy(copyArray, impl.prefixes)
	defer impl.lock.Unlock()
	return copyArray
}

func intersect(first, second *net.IPNet) (bool, bool) {
	f, _ := first.Mask.Size()
	s, _ := second.Mask.Size()
	firstIsBigger := false

	var widerRange, narrowerRange *net.IPNet
	if f < s {
		widerRange, narrowerRange = first, second
		firstIsBigger = true
	} else {
		widerRange, narrowerRange = second, first
	}

	return widerRange.Contains(narrowerRange.IP), firstIsBigger
}
