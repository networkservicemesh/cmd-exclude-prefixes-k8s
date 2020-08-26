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
	"math/big"
	"net"
	"sync"
)

type ExcludePrefixPool struct {
	lock     sync.RWMutex
	prefixes []string
}

func NewExcludePrefixPool(prefixes ...string) (*ExcludePrefixPool, error) {
	pool := &ExcludePrefixPool{
		prefixes: make([]string, 0, len(prefixes)),
	}
	if err := pool.Add(prefixes); err != nil {
		return nil, err
	}

	return pool, nil
}

func (impl *ExcludePrefixPool) Add(prefixesToAdd []string) error {
	prefixesToAdd, _ = impl.deleteEqualPrefixes(prefixesToAdd)
	impl.lock.Lock()
	newPrefixes := make([]string, 0, len(impl.prefixes)+len(prefixesToAdd))
	prefixesToRemove := make(map[int]struct{})

	for _, newPrefix := range prefixesToAdd {
		intersected := false
		_, newPrefixSubnet, err := net.ParseCIDR(newPrefix)
		if err != nil {
			logrus.Errorf("Wrong CIDR: %v", prefixesToAdd)
			return err
		}
		for prefixIndex, prefix := range impl.prefixes {
			_, prefixSubnet, _ := net.ParseCIDR(prefix)
			if intersect, firstIsWider := intersect(newPrefixSubnet, prefixSubnet); intersect {
				intersected = true
				if firstIsWider {
					newPrefixes = append(newPrefixes, newPrefix)
					prefixesToRemove[prefixIndex] = struct{}{}
				}
			}
		}
		if !intersected {
			newPrefixes = append(newPrefixes, newPrefix)
		}
	}

	for i, prefix := range impl.prefixes {
		if _, ok := prefixesToRemove[i]; ok {
			continue
		}

		newPrefixes = append(newPrefixes, prefix)
	}

	impl.prefixes = newPrefixes
	impl.lock.Unlock()

	return nil
}

func (impl *ExcludePrefixPool) deleteEqualPrefixes(prefixesToAdd []string) ([]string, error) {
	newPrefixes := make([]string, 0, len(prefixesToAdd))
	prefixesByMask := make(map[string][]*net.IPNet)

	for _, prefix := range prefixesToAdd {
		_, ipnet, err := net.ParseCIDR(prefix)
		if err != nil {
			return nil, err
		}
		maskSize, _ := ipnet.Mask.Size()
		baseIP := &net.IPNet{
			IP:   clearNetIndexInIP(ipnet.IP, maskSize),
			Mask: ipnet.Mask,
		}
		prefixesByMask[baseIP.String()] = append(prefixesByMask[baseIP.String()], baseIP)
	}

	for mask := range prefixesByMask {
		newPrefixes = append(newPrefixes, mask)
	}

	return newPrefixes, nil
}

func clearNetIndexInIP(ip net.IP, prefixLen int) net.IP {
	ipInt, totalBits := fromIP(ip)
	ipInt.SetBit(ipInt, totalBits-prefixLen, 0)
	return toIP(ipInt, totalBits)
}

func toIP(ipInt *big.Int, bits int) net.IP {
	ipBytes := ipInt.Bytes()
	ret := make([]byte, bits/8)
	// Pack our IP bytes into the end of the return array,
	// since big.Int.Bytes() removes front zero padding.
	for i := 1; i <= len(ipBytes); i++ {
		ret[len(ret)-i] = ipBytes[len(ipBytes)-i]
	}
	return ret
}

func (impl *ExcludePrefixPool) GetPrefixes() []string {
	impl.lock.Lock()
	copyArray := make([]string, len(impl.prefixes))
	copy(copyArray, impl.prefixes)
	defer impl.lock.Unlock()
	return copyArray
}

func intersect(first, second *net.IPNet) (widerContains, firstIsBigger bool) {
	f, _ := first.Mask.Size()
	s, _ := second.Mask.Size()
	firstIsBigger = false

	var widerRange, narrowerRange *net.IPNet
	if f < s {
		widerRange, narrowerRange = first, second
		firstIsBigger = true
	} else {
		widerRange, narrowerRange = second, first
	}

	widerContains = widerRange.Contains(narrowerRange.IP)
	return
}

func validatePrefixes(prefixes []string) error {
	for _, prefix := range prefixes {
		_, _, err := net.ParseCIDR(prefix)
		if err != nil {
			return err
		}
	}

	return nil
}
