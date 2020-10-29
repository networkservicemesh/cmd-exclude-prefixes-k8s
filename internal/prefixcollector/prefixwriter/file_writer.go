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
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"io/ioutil"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"

	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	outputFilePermissions = 0600
)

type FileWriter struct {
	filePath string
}

func NewFileWriter(filePath string) *FileWriter {
	return &FileWriter{
		filePath: filePath,
	}
}

func (f *FileWriter) Write(ctx context.Context, newPrefixes []string) {
	span := spanhelper.FromContext(ctx, "Update excluded prefixes file")
	defer span.Finish()

	data, err := utils.PrefixesToYaml(newPrefixes)
	if err != nil {
		logrus.Errorf("Can not create marshal prefixes, err: %v", err.Error())
		return
	}

	err = ioutil.WriteFile(f.filePath, data, outputFilePermissions)
	if err != nil {
		logrus.Fatalf("Unable to write into file: %v", err.Error())
	}
}

func (f *FileWriter) WatchExcludedPrefixes(ctx context.Context, previousPrefixes *utils.SynchronizedPrefixesContainer) {
	span := spanhelper.FromContext(ctx, "Watch excluded prefixes file")
	defer span.Finish()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer func() { _ = watcher.Close() }()

	err = watcher.Add(f.filePath)
	if err != nil {
		return
	}

	logEntry := span.Logger().WithFields(logrus.Fields{
		"filepath": f.filePath,
	})

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				bytes, err := ioutil.ReadFile(f.filePath)
				if err != nil {
					logEntry.Errorf("Error reading excluded prefixes file: %v", err)
					continue
				}

				prefixes, err := utils.YamlToPrefixes(bytes)
				if err != nil || !utils.UnorderedSlicesEquals(prefixes, previousPrefixes.Load()) {
					logEntry.Warn("Excluded prefixes file external change, restoring last state")
					f.Write(ctx, previousPrefixes.Load())
				}
			}
		case watcherError, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logEntry.Errorf("Error watching file: %v", watcherError)
		}
	}
}
