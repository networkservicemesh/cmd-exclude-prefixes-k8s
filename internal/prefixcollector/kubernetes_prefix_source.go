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
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	"strings"
)

const (
	KubeNamespace = "kube-system"
	KubeName      = "kubeadm-config"
)

type KubernetesPrefixSource struct {
	PrefixChan chan []string
	errorChan  chan error
}

func (kps *KubernetesPrefixSource) ErrorChan() <-chan error {
	return kps.errorChan
}

func (kps *KubernetesPrefixSource) ResultChan() <-chan []string {
	return kps.PrefixChan
}

func NewKubernetesPrefixSource(context context.Context) (*KubernetesPrefixSource, error) {
	kps := &KubernetesPrefixSource{
		make(chan []string, 1),
		make(chan error),
	}
	kubeAdmPrefixes, err := getExcludedPrefixesFromConfigMap(context)
	if err == nil {
		kps.PrefixChan <- kubeAdmPrefixes
		return kps, nil
	}

	go kps.watchSubnets(context)
	return kps, nil
}

func getExcludedPrefixesFromConfigMap(context context.Context) ([]string, error) {
	clientSet := FromContext(context)
	kubeadmConfig, err := clientSet.CoreV1().ConfigMaps(KubeNamespace).
		Get(context, KubeName, metav1.GetOptions{})
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	clusterConfiguration := &v1beta2.ClusterConfiguration{}
	err = yaml.NewYAMLOrJSONDecoder(strings.NewReader(kubeadmConfig.Data["ClusterConfiguration"]), 4096).
		Decode(clusterConfiguration)
	if err != nil {
		return nil, err
	}

	podSubnet := clusterConfiguration.Networking.PodSubnet
	serviceSubnet := clusterConfiguration.Networking.ServiceSubnet

	if podSubnet == "" {
		return nil, errors.New("ClusterConfiguration.Networking.PodSubnet is empty")
	}
	if serviceSubnet == "" {
		return nil, errors.New("ClusterConfiguration.Networking.ServiceSubnet is empty")
	}

	return []string{
		podSubnet,
		serviceSubnet,
	}, nil
}

func (kps *KubernetesPrefixSource) watchSubnets(context context.Context) {
	clientSet := FromContext(context)
	for {
		go MonitorReservedSubnets(kps.PrefixChan, kps.errorChan, clientSet)
		err := <-kps.errorChan
		logrus.Error(err)
	}
}
