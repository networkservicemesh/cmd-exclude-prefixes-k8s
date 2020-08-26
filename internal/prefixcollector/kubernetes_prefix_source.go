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
	"cmd-exclude-prefixes-k8s/internal/utils"
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// KubernetesPrefixSource is excluded prefix source, which get prefixes
// from Kubernetes pods and services addresses
type KubernetesPrefixSource struct {
	prefixes utils.SynchronizedPrefixesContainer
	ctx      context.Context
}

// Start - starts monitoring Kubernetes pods and services. Notifies notifyChan after reading prefixes.
func (kps *KubernetesPrefixSource) Start(notifyChan chan<- struct{}) {
	go kps.start(notifyChan)
}

// GetPrefixes returns prefixes from source
func (kps *KubernetesPrefixSource) GetPrefixes() []string {
	return kps.prefixes.GetList()
}

// NewKubernetesPrefixSource creates KubernetesPrefixSource
func NewKubernetesPrefixSource(ctx context.Context) *KubernetesPrefixSource {
	kps := &KubernetesPrefixSource{
		ctx: ctx,
	}

	return kps
}

func (kps *KubernetesPrefixSource) start(notifyChan chan<- struct{}) {
	clientSet := utils.FromContext(kps.ctx)
	for {
		kps.watchSubnets(notifyChan, clientSet)
	}
}

func (kps *KubernetesPrefixSource) watchSubnets(notifyChan chan<- struct{}, clientSet kubernetes.Interface) {
	pw, err := watchPodCIDR(clientSet)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer pw.Stop()

	sw, err := watchServiceIPAddr(clientSet)
	if err != nil {
		logrus.Error(err)
		return
	}
	defer sw.Stop()

	kps.waitForSubnets(notifyChan, pw, sw)
}

func (kps *KubernetesPrefixSource) waitForSubnets(notifyChan chan<- struct{}, pw, sw *SubnetWatcher) {
	var podSubnet, serviceSubnet string
	for {
		select {
		case subnet, ok := <-pw.ResultChan():
			if !ok {
				return
			}
			podSubnet = subnet.String()
		case subnet, ok := <-sw.ResultChan():
			if !ok {
				return
			}
			serviceSubnet = subnet.String()
		}

		prefixes := getPrefixes(podSubnet, serviceSubnet)
		kps.prefixes.SetList(prefixes)
		utils.Notify(notifyChan)
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
