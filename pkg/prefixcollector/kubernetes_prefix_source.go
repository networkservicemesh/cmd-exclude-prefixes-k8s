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

package prefixcollector

import (
	"cmd-exclude-prefixes-k8s/pkg/utils"
	"context"
	"net"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// KubernetesPrefixSource is excluded prefix source, which get prefixes
// from Kubernetes pods and services addresses
type KubernetesPrefixSource struct {
	prefixes *utils.SynchronizedPrefixesContainer
	ctx      context.Context
}

// Prefixes returns prefixes from source
func (kps *KubernetesPrefixSource) Prefixes() []string {
	return kps.prefixes.Load()
}

// NewKubernetesPrefixSource creates KubernetesPrefixSource
func NewKubernetesPrefixSource(ctx context.Context, notify Notifier) *KubernetesPrefixSource {
	kps := &KubernetesPrefixSource{
		ctx:      ctx,
		prefixes: utils.NewSynchronizedPrefixesContainer(),
	}

	go func() {
		clientSet := KubernetesInterface(kps.ctx)
		for kps.ctx.Err() == nil {
			kps.watchSubnets(notify, clientSet)
		}
	}()
	return kps
}

func (kps *KubernetesPrefixSource) watchSubnets(notify Notifier, clientSet kubernetes.Interface) {
	podChan, err := watchPodCIDR(kps.ctx, clientSet)
	if err != nil {
		logrus.Error(err)
		return
	}

	serviceChan, err := watchServiceIPAddr(kps.ctx, clientSet)
	if err != nil {
		logrus.Error(err)
		return
	}

	kps.waitForSubnets(notify, podChan, serviceChan)
}

func (kps *KubernetesPrefixSource) waitForSubnets(notify Notifier, podChan, serviceChan <-chan *net.IPNet) {
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
		notify.Broadcast()
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
