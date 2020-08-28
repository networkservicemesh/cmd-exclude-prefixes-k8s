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
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
)

const (
	// KubeNamespace is KubeAdm ConfigMap namespace
	KubeNamespace = "kube-system"
	// KubeName is KubeAdm ConfigMap name
	KubeName   = "kubeadm-config"
	bufferSize = 4096
)

// KubeAdmPrefixSource is KubeAdm ConfigMap excluded prefix source
type KubeAdmPrefixSource struct {
	configMapInterface v1.ConfigMapInterface
	prefixes           utils.SynchronizedPrefixesContainer
	ctx                context.Context
}

// Start - starts monitoring KubeAdm ConfigMap's properties "PodSubnet" and "ServiceSubnet".
// Notifies notifyChan after reading prefixes.
func (kaps *KubeAdmPrefixSource) Start(notifyChan chan<- struct{}) {
	go kaps.watchKubeAdmConfigMap(notifyChan)
}

// Prefixes returns prefixes from source
func (kaps *KubeAdmPrefixSource) Prefixes() []string {
	return kaps.prefixes.GetList()
}

// NewKubeAdmPrefixSource creates KubeAdmPrefixSource
func NewKubeAdmPrefixSource(ctx context.Context) *KubeAdmPrefixSource {
	clientSet := utils.FromContext(ctx)
	configMapInterface := clientSet.CoreV1().ConfigMaps(KubeNamespace)
	kaps := KubeAdmPrefixSource{
		configMapInterface: configMapInterface,
		ctx:                ctx,
	}

	return &kaps
}

func (kaps *KubeAdmPrefixSource) watchKubeAdmConfigMap(notifyChan chan<- struct{}) {
	for {
		clientSet := utils.FromContext(kaps.ctx)
		kubeadmConfig, err := clientSet.CoreV1().ConfigMaps(KubeNamespace).
			Get(kaps.ctx, KubeName, metav1.GetOptions{})
		if err != nil {
			logrus.Error(err)
			continue
		}

		clusterConfiguration := &v1beta2.ClusterConfiguration{}
		err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(kubeadmConfig.Data["ClusterConfiguration"]), bufferSize).
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

		kaps.prefixes.SetList([]string{
			podSubnet,
			serviceSubnet,
		})
		utils.Notify(notifyChan)
		<-time.After(time.Second * 10)
	}
}
