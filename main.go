// Copyright (c) 2020 Doc.ai and/or its affiliates.
//
// Copyright (c) 2019 Cisco and/or its affiliates.
//
// Copyright (c) 2019 Red Hat Inc. and/or its affiliates.
//
// Copyright (c) 2019 VMware, Inc.
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

	ctx = context.WithValue(ctx, prefixcollector.ClientsetKey, kubernetes.Interface(clientset))

	filePath := prefixpool.PrefixesFilePathDefault
	filePath = "D:\\GO\\test\\excluded_prefixes.yaml"

	excludePrefixChan := prefixcollector.GetExcludePrefixChan(ctx,
		prefixcollector.FromEnv(),
		prefixcollector.FromConfigMap("nsm-config-volume", "default"),
		prefixcollector.FromKubernetes(),
	)

	err = prefixcollector.StartServer(filePath, excludePrefixChan)

	if err != nil {
		span.Logger().Fatalln(err)
	}

	span.Finish() // exclude main cycle run time from span timing
	<-ctx.Done()
}

/*
	Todo: 1)design or rename
	2) namespace from env + default if doenst exist
	3) prefix pool check
	4) tests
*/
