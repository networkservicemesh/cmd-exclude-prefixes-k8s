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
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"

	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/signalctx"
	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

// Config - configuration for cmd-exclude-prefixes-k8s
type Config struct {
	ExcludedPrefixes   []string             `desc:"List of excluded prefixes" split_words:"true"`
	ConfigMapNamespace string               `default:"default" desc:"Namespace of kubernetes config map" split_words:"true"`
	KubeConfig         utils.KubeConfigPath `desc:"Path to kubeconfig file"`
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
	if err := envconfig.Usage("", config); err != nil {
		logrus.Fatal(err)
	}
	if err := envconfig.Process("", config); err != nil {
		logrus.Fatalf("error processing clientSetConfig from env: %+v", err)
	}

	span.Logger().Printf("Building Kubernetes clientset...")
	clientSetConfig, err := clientcmd.BuildConfigFromFlags("", string(config.KubeConfig))
	if err != nil {
		span.Logger().Fatalln("Failed to build Kubernetes clientset: ", err)
	}

	span.Logger().Infof("Starting prefix service...")

	clientset, err := kubernetes.NewForConfig(clientSetConfig)
	if err != nil {
		span.Logger().Fatalln("Failed to build Kubernetes clientset: ", err)
	}

	ctx = context.WithValue(ctx, utils.ClientSetKey, kubernetes.Interface(clientset))

	excludePrefixService := prefixcollector.NewExcludePrefixCollector(
		ctx,
		config.ExcludedPrefixes,
		config.ConfigMapNamespace,
	)
	excludePrefixService.Start()

	span.Finish() // exclude main cycle run time from span timing
	<-ctx.Done()
}
