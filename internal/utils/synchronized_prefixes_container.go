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

// Container with synchronized getter and setter
type SynchronizedPrefixesContainer struct {
	lock     sync.Mutex
	prefixes []string
}

// Get prefixes list
func (s *SynchronizedPrefixesContainer) GetList() []string {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.prefixes
}

// Set prefixes list
func (s *SynchronizedPrefixesContainer) SetList(list []string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.prefixes = list
}
