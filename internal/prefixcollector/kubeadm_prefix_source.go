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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	"strings"
)

const (
	KubeNamespace = "kube-system"
	KubeName      = "kubeadm-config"
)

type KubeAdmPrefixSource struct {
	configMapInterface v1.ConfigMapInterface
	prefixes           utils.SynchronizedPrefixList
	notifyChan         chan struct{}
}

func (kaps *KubeAdmPrefixSource) GetNotifyChannel() <-chan struct{} {
	return kaps.notifyChan
}

func (kaps *KubeAdmPrefixSource) GetPrefixes() []string {
	return kaps.prefixes.GetList()
}

func NewKubeAdmPrefixSource(context context.Context) *KubeAdmPrefixSource {
	clientSet := utils.FromContext(context)
	configMapInterface := clientSet.CoreV1().ConfigMaps(KubeNamespace)
	kaps := KubeAdmPrefixSource{
		configMapInterface,
		utils.NewSynchronizedPrefixListImpl(),
		make(chan struct{}, 1),
	}

	go kaps.watchKubeAdmConfigMap(context)

	return &kaps
}

func (cmps *KubeAdmPrefixSource) watchKubeAdmConfigMap(context context.Context) {
	for {
		clientSet := utils.FromContext(context)
		kubeadmConfig, err := clientSet.CoreV1().ConfigMaps(KubeNamespace).
			Get(context, KubeName, metav1.GetOptions{})
		if err != nil {
			logrus.Error(err)
			continue
		}

		clusterConfiguration := &v1beta2.ClusterConfiguration{}
		err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(kubeadmConfig.Data["ClusterConfiguration"]), 4096).
			Decode(clusterConfiguration)
		if err != nil {
			logrus.Error(err)
			continue
		}

		podSubnet := clusterConfiguration.Networking.PodSubnet
		serviceSubnet := clusterConfiguration.Networking.ServiceSubnet

		if podSubnet == "" {
			logrus.Error("ClusterConfiguration.Networking.PodSubnet is empty")
		}
		if serviceSubnet == "" {
			logrus.Error("ClusterConfiguration.Networking.ServiceSubnet is empty")
		}

		cmps.prefixes.SetList([]string{
			podSubnet,
			serviceSubnet,
		})
		utils.Notify(cmps.notifyChan)
	}
}
