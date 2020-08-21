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
	"k8s.io/client-go/kubernetes"
)

func MonitorReservedSubnets(ch chan []string, errCh chan<- error, clientset *kubernetes.Clientset) {
	pw, err := WatchPodCIDR(clientset)
	if err != nil {
		errCh <- err
		return
	}
	defer pw.Stop()

	sw, err := WatchServiceIpAddr(clientset)
	if err != nil {
		errCh <- err
		return
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
		sendPrefixes(ch, podSubnet, serviceSubnet)
	}
}

func sendPrefixes(ch chan []string, podSubnet, serviceSubnet string) {
	prefixes := getPrefixes(podSubnet, serviceSubnet)
	select {
	case <-ch:
	default:
	}
	ch <- prefixes
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
