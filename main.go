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

package main

import (
	"cmd-exclude-prefixes-k8s/pkg/prefixcollector"
	"cmd-exclude-prefixes-k8s/pkg/utils"
	"context"
	"net"
	"sync"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/signalctx"
	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	envPrefix = "exclude_prefixes_k8s"
)

// Config - configuration for cmd-exclude-prefixes-k8s
type Config struct {
	ExcludedPrefixes   []string `desc:"List of excluded prefixes" split_words:"true"`
	ConfigMapNamespace string   `default:"default" desc:"Namespace of kubernetes config map" split_words:"true"`
	ConfigMapName      string   `default:"nsm-config" desc:"Name of kubernetes config map" split_words:"true"`
	PrefixesFilePath   string   `default:"/var/lib/networkservicemesh/config/excluded_prefixes.yaml" desc:"Excluded prefixes file absolute path" split_words:"true"`
}

func main() {
	// Capture signals to cleanup before exiting
	ctx := signalctx.WithSignals(context.Background())

	closer := jaeger.InitJaeger("prefix-service")
	defer func() { _ = closer.Close() }()

	span := spanhelper.FromContext(context.Background(), "Start prefix service")
	defer span.Finish()

	// Get clientSetConfig from environment
	config := &Config{}
	if err := envconfig.Usage(envPrefix, config); err != nil {
		logrus.Fatal(err)
	}
	if err := envconfig.Process(envPrefix, config); err != nil {
		logrus.Fatalf("Error processing clientSetConfig from env: %v", err)
	}

	envPrefixes, err := validatedPrefixes(config.ExcludedPrefixes)
	if err != nil {
		span.Logger().Fatalf("Failed to parse prefixes from environment: %v", err)
	}

	span.Logger().Printf("Building Kubernetes clientSet...")
	clientSetConfig, err := utils.NewClientSetConfig()
	if err != nil {
		span.Logger().Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	span.Logger().Info("Starting prefix service...")

	clientSet, err := kubernetes.NewForConfig(clientSetConfig)
	if err != nil {
		span.Logger().Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	ctx = prefixcollector.WithKubernetesInterface(ctx, kubernetes.Interface(clientSet))
	cond := sync.NewCond(&sync.Mutex{})

	excludePrefixService := prefixcollector.NewExcludePrefixCollector(
		config.PrefixesFilePath,
		cond,
		prefixcollector.NewEnvPrefixSource(envPrefixes),
		prefixcollector.NewKubeAdmPrefixSource(ctx, cond),
		prefixcollector.NewKubernetesPrefixSource(ctx, cond),
		prefixcollector.NewConfigMapPrefixSource(ctx, cond, config.ConfigMapName, config.ConfigMapNamespace),
	)

	go excludePrefixService.Serve(ctx)

	span.Finish() // exclude main cycle run time from span timing
	<-ctx.Done()
}

// validatedPrefixes returns list of validated via CIDR notation parsing prefixes
func validatedPrefixes(prefixes []string) ([]string, error) {
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
