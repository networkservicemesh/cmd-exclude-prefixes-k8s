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
	"github.com/networkservicemesh/sdk/pkg/tools/prefixpool"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/jaeger"
	"github.com/networkservicemesh/sdk/pkg/tools/signalctx"
	"github.com/networkservicemesh/sdk/pkg/tools/spanhelper"
)

func main() {
	logrus.Info("Starting prefix service...")
	utils.PrintAllEnv(logrus.StandardLogger())
	// Capture signals to cleanup before exiting
	ctx := signalctx.WithSignals(context.Background())

	closer := jaeger.InitJaeger("prefix-service")
	defer func() { _ = closer.Close() }()

	span := spanhelper.FromContext(context.Background(), "Start prefix service")
	defer span.Finish()

	span.Logger().Printf("Building Kubernetes clientset...")
	config, err := utils.NewClientSetConfig()
	if err != nil {
		span.LogError(err)
		span.Logger().Fatalln("Failed to build Kubernetes clientset: ", err)
	}

	span.Logger().Infof("Starting prefix service...")

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		span.Logger().Fatalln("Failed to build Kubernetes clientset: ", err)
	}

	ctx = context.WithValue(ctx, utils.ClientSetKey, kubernetes.Interface(clientset))

	if err = utils.CreateDirIfNotExists(prefixpool.NSMConfigDir); err != nil {
		span.Logger().Fatalf("Failed to create exclude prefixes directory %v: %v", prefixpool.NSMConfigDir, err)
	}
	filePath := prefixpool.PrefixesFilePathDefault

	excludePrefixService := prefixcollector.NewExcludePrefixCollector(filePath, ctx,
		prefixcollector.WithConfigMapSource(),
		prefixcollector.WithKubeadmConfigSource(),
		prefixcollector.WithKubernetesSource(),
	)
	excludePrefixService.Start()

	span.Finish() // exclude main cycle run time from span timing
	<-ctx.Done()
}
