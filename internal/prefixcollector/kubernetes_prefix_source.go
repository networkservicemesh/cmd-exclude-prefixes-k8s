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
)

type KubernetesPrefixSource struct {
	notifyChan chan struct{}
	prefixes   utils.SynchronizedPrefixList
}

func (kps *KubernetesPrefixSource) GetNotifyChannel() <-chan struct{} {
	return kps.notifyChan
}

func (kps *KubernetesPrefixSource) GetPrefixes() []string {
	return kps.prefixes.GetList()
}

func NewKubernetesPrefixSource(context context.Context) (*KubernetesPrefixSource, error) {
	kps := &KubernetesPrefixSource{
		make(chan struct{}, 1),
		utils.NewSynchronizedPrefixListImpl(),
	}

	go kps.watchSubnets(context)
	return kps, nil
}

func (kps *KubernetesPrefixSource) watchSubnets(context context.Context) {
	clientSet := FromContext(context)
	for {
		pw, err := WatchPodCIDR(clientSet)
		if err != nil {
			logrus.Error(err)
			continue
		}
		defer pw.Stop()

		sw, err := WatchServiceIpAddr(clientSet)
		if err != nil {
			logrus.Error(err)
			continue
		}
		defer sw.Stop()

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
			kps.notifyChan <- struct{}{}
		}
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
