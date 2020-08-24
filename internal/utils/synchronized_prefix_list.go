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

package utils

import "sync"

type SynchronizedPrefixList interface {
	Append(string)
	SetList([]string)
	GetList() []string
}

type SynchronizedPrefixListImpl struct {
	lock     sync.Mutex
	prefixes []string
}

func NewSynchronizedPrefixListImpl() *SynchronizedPrefixListImpl {
	return &SynchronizedPrefixListImpl{
		prefixes: make([]string, 0, 16),
	}
}

func (s *SynchronizedPrefixListImpl) Append(prefix string) {
	s.lock.Lock()
	s.prefixes = append(s.prefixes, prefix)
	s.lock.Unlock()
}

func (s *SynchronizedPrefixListImpl) GetList() []string {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.prefixes
}

func (s *SynchronizedPrefixListImpl) SetList(list []string) {
	s.lock.Lock()
	s.prefixes = list
	s.lock.Unlock()
}
