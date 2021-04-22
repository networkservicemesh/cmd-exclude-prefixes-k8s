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
	"context"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/networkservicemesh/sdk-k8s/pkg/k8s"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

const (
	envPrefix            = "exclude_prefixes_k8s"
	currentNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

func main() {
	// Capture signals to cleanup before exiting
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		// More Linux signals here
		syscall.SIGHUP,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)
	defer cancel()

	closer := jaeger.InitJaeger("prefix-service")
	defer func() { _ = closer.Close() }()

	span := spanhelper.FromContext(context.Background(), "Start prefix service")
	defer span.Finish()

	// Get clientSetConfig from environment
	config := &prefixcollector.Config{}
	if err := envconfig.Usage(envPrefix, config); err != nil {
		span.Logger().Fatal(err)
	}
	if err := envconfig.Process(envPrefix, config); err != nil {
		span.Logger().Fatalf("Error processing clientSetConfig from env: %v", err)
	}
	if err := config.Validate(); err != nil {
		span.Logger().Fatalf("Error validating Config from env: %v", err)
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

	prefixesOutputOption := prefixcollector.WithFileOutput(config.OutputFilePath)
	if config.PrefixesOutputType == prefixcollector.ConfigMapOutputType {
		currentNamespaceBytes, ioErr := ioutil.ReadFile(currentNamespacePath)
		if ioErr != nil {
			span.Logger().Fatalf("Error reading namespace from secret: %v", ioErr)
		}
		currentNamespace := strings.TrimSpace(string(currentNamespaceBytes))
		prefixesOutputOption = prefixcollector.WithConfigMapOutput(config.NSMConfigMapName, currentNamespace)
	}

	if err != nil {
		span.Logger().Fatal(err)
	}

	notifyChan := make(chan struct{}, 1)
	prefixCollector := prefixcollector.NewExcludePrefixCollector(
		prefixesOutputOption,
		prefixcollector.WithNotifyChan(notifyChan),
		prefixcollector.WithSources(
			prefixsource.NewEnvPrefixSource(config.ExcludedPrefixes),
			prefixsource.NewKubeAdmPrefixSource(ctx, notifyChan),
			prefixsource.NewKubernetesPrefixSource(ctx, notifyChan),
			prefixsource.NewConfigMapPrefixSource(ctx, notifyChan, config.ConfigMapName, config.ConfigMapNamespace),
		),
	)

	go prefixCollector.Serve(ctx)

	span.Finish() // exclude main cycle run time from span timing
	<-ctx.Done()
}
