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
	"cmd-exclude-prefixes-k8s/internal/utils"
	"sync"
	"time"
)

// EnvPrefixSource is environment excluded prefixes source
type EnvPrefixSource struct {
	prefixes utils.SynchronizedPrefixesContainer
}

// Prefixes returns prefixes from source
func (e *EnvPrefixSource) Prefixes() []string {
	return e.prefixes.GetList()
}

// NewEnvPrefixSource creates EnvPrefixSource
func NewEnvPrefixSource(uncheckedPrefixes []string, notify *sync.Cond) *EnvPrefixSource {
	prefixes := utils.GetValidatedPrefixes(uncheckedPrefixes)
	source := &EnvPrefixSource{}
	source.prefixes.SetList(prefixes)
	if prefixes != nil {
		go source.notifyListeners(notify)
	}
	return source
}

func (e *EnvPrefixSource) notifyListeners(notify *sync.Cond) {
	for {
		notify.Broadcast()
		<-time.After(time.Second * 10)
	}
}
