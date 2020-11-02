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

package prefixwriter

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"io/ioutil"

	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	outputFilePermissions = 0600
)

// NewFileWriter - creates file WritePrefixesFunc
func NewFileWriter(filePath string) prefixcollector.WritePrefixesFunc {
	return func(ctx context.Context, newPrefixes []string) {
		span := spanhelper.FromContext(ctx, "Update excluded prefixes file")
		defer span.Finish()

		data, err := utils.PrefixesToYaml(newPrefixes)
		if err != nil {
			span.Logger().Errorf("Can not create marshal prefixes, err: %v", err.Error())
			return
		}

		err = ioutil.WriteFile(filePath, data, outputFilePermissions)
		if err != nil {
			span.Logger().Fatalf("Unable to write into file: %v", err.Error())
		}
	}
}
