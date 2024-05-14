// Copyright (c) 2020-2022 Doc.ai and/or its affiliates.
//
// Copyright (c) 2023 Cisco and/or its affiliates.
//
// Copyright (c) 2024 OpenInfra Foundation Europe. All rights reserved.
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
	"net"
	"time"

	"github.com/pkg/errors"
)

const (
	// ConfigMapOutputType is excluded prefixes k8s config map output type
	ConfigMapOutputType = "config-map"
	// FileOutputType is excluded prefixes file output type
	FileOutputType = "file"
)

// Config - configuration for cmd-exclude-prefixes-k8s
type Config struct {
	ExcludedPrefixes      []string      `desc:"List of excluded prefixes" split_words:"true"`
	ConfigMapNamespace    string        `default:"default" desc:"Namespace of user config map" split_words:"true"`
	ConfigMapName         string        `default:"excluded-prefixes-config" desc:"Name of user config map" split_words:"true"`
	ConfigMapKey          string        `default:"excluded_prefixes_input.yaml" desc:"key in the input configmap by which we retrieve data('filename' in data section in configmap specification yaml file)" split_words:"true"`
	OutputConfigMapName   string        `default:"nsm-config" desc:"Name of nsm config map" split_words:"true"`
	OutputConfigMapKey    string        `default:"excluded_prefixes_output.yaml" desc:"key in the output configmap by which we retrieve data('filename' in data section in configmap specification yaml file)" split_words:"true"`
	OutputFilePath        string        `default:"/var/lib/networkservicemesh/config/excluded_prefixes.yaml" desc:"Path of output prefixes file" split_words:"true"`
	PrefixesOutputType    string        `default:"file" desc:"Where to write excluded prefixes" split_words:"true"`
	LogLevel              string        `default:"INFO" desc:"Log level" split_words:"true"`
	OpenTelemetryEndpoint string        `default:"otel-collector.observability.svc.cluster.local:4317" desc:"OpenTelemetry Collector Endpoint" split_words:"true"`
	MetricsExportInterval time.Duration `default:"10s" desc:"interval between mertics exports" split_words:"true"`
}

// Validate - validates config. Checks PrefixesOutputType and every CIDR from config.ExcludedPrefixes.
func (c *Config) Validate() error {
	for _, prefix := range c.ExcludedPrefixes {
		_, _, err := net.ParseCIDR(prefix)
		if err != nil {
			return errors.Wrap(err, "Failed to parse prefixes from environment")
		}
	}

	if c.PrefixesOutputType != ConfigMapOutputType && c.PrefixesOutputType != FileOutputType {
		return errors.New("Wrong prefixes output type")
	}

	return nil
}
