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
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/prefixcollector/prefixsource"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"
	"io/ioutil"
	"net"
	"strings"

	"github.com/networkservicemesh/sdk-k8s/pkg/k8s"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/signalctx"
	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	envPrefix            = "exclude_prefixes_k8s"
	currentNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Config - configuration for cmd-exclude-prefixes-k8s
type Config struct {
	ExcludedPrefixes   []string `desc:"List of excluded prefixes" split_words:"true"`
	ConfigMapNamespace string   `default:"default" desc:"Namespace of user config map" split_words:"true"`
	ConfigMapName      string   `default:"excluded-prefixes-config" desc:"Name of user config map" split_words:"true"`
	NSMConfigMapName   string   `default:"nsm-config" desc:"Name of nsm config map" split_words:"true"`
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
		span.Logger().Fatal(err)
	}
	if err := envconfig.Process(envPrefix, config); err != nil {
		span.Logger().Fatalf("Error processing clientSetConfig from env: %v", err)
	}

	envPrefixes, err := validatedPrefixes(config.ExcludedPrefixes)
	if err != nil {
		span.Logger().Fatalf("Failed to parse prefixes from environment: %v", err)
	}

	span.Logger().Info("Building Kubernetes clientSet...")
	clientSetConfig, err := k8s.NewClientSetConfig()
	if err != nil {
		span.Logger().Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	span.Logger().Info("Starting prefix service...")

	clientSet, err := kubernetes.NewForConfig(clientSetConfig)
	if err != nil {
		span.Logger().Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	ctx = prefixcollector.WithKubernetesInterface(ctx, kubernetes.Interface(clientSet))
	notifiable := utils.NewChannelNotifiable()

	currentNamespace, err := ioutil.ReadFile(currentNamespacePath)
	if err != nil {
		span.Logger().Fatalf("Error reading namespace from secret: %v", err)
	}

	excludePrefixService := prefixcollector.NewExcludePrefixCollector(
		notifiable,
		config.NSMConfigMapName,
		strings.TrimSpace(string(currentNamespace)),
		prefixsource.NewEnvPrefixSource(envPrefixes),
		prefixsource.NewKubeAdmPrefixSource(ctx, notifiable),
		prefixsource.NewKubernetesPrefixSource(ctx, notifiable),
		prefixsource.NewConfigMapPrefixSource(ctx, notifiable, config.ConfigMapName, config.ConfigMapNamespace),
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
