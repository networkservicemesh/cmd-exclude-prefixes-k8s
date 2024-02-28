// Copyright (c) 2020-2021 Doc.ai and/or its affiliates.
//
// Copyright (c) 2022-2024 Cisco and/or its affiliates.
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
	"cmd-exclude-prefixes-k8s/internal/prefixcollector"
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"

	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/log"
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
	log.FromContext(kps.ctx).Infof("KubernetesPrefixSource watch subnets")
	subnetCtx, cancelSubnetCtx := context.WithCancel(kps.ctx)
	defer cancelSubnetCtx()

	podChan, err := watchPodCIDR(subnetCtx, clientSet)
	if err != nil {
		return
	}

	serviceChan, err := watchServiceIPAddr(subnetCtx, clientSet)
	if err != nil {
		log.FromContext(kps.ctx).Error(err)
		return
	}

	kps.waitForSubnets(podChan, serviceChan)
}

func (kps *KubernetesPrefixSource) waitForSubnets(podChan, serviceChan <-chan []string) {
	var podPrefixes, svcPrefixes []string
	for {
		select {
		case <-kps.ctx.Done():
			log.FromContext(kps.ctx).Warn("kubernetesPrefixSource context is canceled")
			return
		case subnet, ok := <-podChan:
			if !ok {
				log.FromContext(kps.ctx).Warn("podChan watcher closed")
				return
			}
			podPrefixes = subnet
		case subnet, ok := <-serviceChan:
			if !ok {
				log.FromContext(kps.ctx).Warn("serviceChan closed")
				return
			}
			svcPrefixes = subnet
		}

		var prefixes []string
		prefixes = append(prefixes, svcPrefixes...)
		prefixes = append(prefixes, podPrefixes...)
		kps.prefixes.Store(prefixes)
		kps.notify <- struct{}{}
	}
}
