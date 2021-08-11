// Copyright (c) 2020-2021 Doc.ai and/or its affiliates.
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

package prefixsource

import (
	"context"
	"net"

	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"

	"github.com/networkservicemesh/sdk/pkg/tools/log"

	"k8s.io/client-go/kubernetes"
)

// KubernetesPrefixSource is excluded prefix source, which get prefixes
// from Kubernetes pods and services addresses
type KubernetesPrefixSource struct {
	prefixes *utils.SynchronizedPrefixesContainer
	notify   chan<- struct{}
	ctx      context.Context
}

// Prefixes returns prefixes from source
func (kps *KubernetesPrefixSource) Prefixes() []string {
	return kps.prefixes.Load()
}

// NewKubernetesPrefixSource creates KubernetesPrefixSource
func NewKubernetesPrefixSource(ctx context.Context, notify chan<- struct{}) *KubernetesPrefixSource {
	kps := &KubernetesPrefixSource{
		ctx:      ctx,
		notify:   notify,
		prefixes: utils.NewSynchronizedPrefixesContainer(),
	}

	go func() {
		clientSet := prefixcollector.KubernetesInterface(kps.ctx)
		for kps.ctx.Err() == nil {
			kps.watchSubnets(clientSet)
		}
	}()
	return kps
}

func (kps *KubernetesPrefixSource) watchSubnets(clientSet kubernetes.Interface) {
	logger := log.FromContext(kps.ctx)
	logger.Infof("Watch k8s subnets")

	podChan, err := watchPodCIDR(kps.ctx, clientSet)
	if err != nil {
		logger.Error(err)
		return
	}

	serviceChan, err := watchServiceIPAddr(kps.ctx, clientSet)
	if err != nil {
		logger.Error(err)
		return
	}

	kps.waitForSubnets(podChan, serviceChan)
}

func (kps *KubernetesPrefixSource) waitForSubnets(podChan, serviceChan <-chan *net.IPNet) {
	var podSubnet, serviceSubnet string
	for {
		select {
		case <-kps.ctx.Done():
			return
		case subnet, ok := <-podChan:
			if !ok {
				return
			}
			podSubnet = subnet.String()
		case subnet, ok := <-serviceChan:
			if !ok {
				return
			}
			serviceSubnet = subnet.String()
		}

		prefixes := getPrefixes(podSubnet, serviceSubnet)
		kps.prefixes.Store(prefixes)
		kps.notify <- struct{}{}
	}
}

func getPrefixes(podSubnet, serviceSubnet string) []string {
	var prefixes []string
	if len(podSubnet) > 0 {
		prefixes = append(prefixes, podSubnet)
	}
	if len(serviceSubnet) > 0 {
		prefixes = append(prefixes, serviceSubnet)
	}

	return prefixes
}
