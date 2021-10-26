// Copyright (c) 2021 Doc.ai and/or its affiliates.
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

	"github.com/networkservicemesh/sdk-k8s/pkg/tools/k8s"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/log"
	"github.com/networkservicemesh/sdk/pkg/tools/log/logruslogger"
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

	closer := jaeger.InitJaeger(ctx, "prefix-service")
	defer func() { _ = closer.Close() }()
	ctx = log.WithFields(ctx, map[string]interface{}{"cmd": os.Args[:1]})
	ctx = log.WithLog(ctx, logruslogger.New(ctx))

	// Get clientSetConfig from environment
	config := &prefixcollector.Config{}
	if err := envconfig.Usage(envPrefix, config); err != nil {
		log.FromContext(ctx).Fatal(err)
	}
	if err := envconfig.Process(envPrefix, config); err != nil {
		log.FromContext(ctx).Fatalf("Error processing clientSetConfig from env: %v", err)
	}
	if err := config.Validate(); err != nil {
		log.FromContext(ctx).Fatalf("Error validating Config from env: %v", err)
	}

	level, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		logrus.Fatalf("invalid log level %s", config.LogLevel)
	}
	logrus.SetLevel(level)

	log.FromContext(ctx).Info("Building Kubernetes clientSet...")
	clientSetConfig, err := k8s.NewClientSetConfig()
	if err != nil {
		log.FromContext(ctx).Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	log.FromContext(ctx).Info("Starting prefix service...")

	clientSet, err := kubernetes.NewForConfig(clientSetConfig)
	if err != nil {
		log.FromContext(ctx).Fatalf("Failed to build Kubernetes clientSet: %v", err)
	}

	ctx = prefixcollector.WithKubernetesInterface(ctx, kubernetes.Interface(clientSet))

	prefixesOutputOption := prefixcollector.WithFileOutput(config.OutputFilePath)
	if config.PrefixesOutputType == prefixcollector.ConfigMapOutputType {
		currentNamespaceBytes, ioErr := ioutil.ReadFile(currentNamespacePath)
		if ioErr != nil {
			log.FromContext(ctx).Fatalf("Error reading namespace from secret: %v", ioErr)
		}
		currentNamespace := strings.TrimSpace(string(currentNamespaceBytes))
		prefixesOutputOption = prefixcollector.WithConfigMapOutput(config.NSMConfigMapName, currentNamespace)
	}

	if err != nil {
		log.FromContext(ctx).Fatal(err)
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

	<-ctx.Done()
}
