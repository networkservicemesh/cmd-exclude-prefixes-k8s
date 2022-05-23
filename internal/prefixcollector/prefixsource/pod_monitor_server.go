// Copyright (c) 2020-2021 Doc.ai and/or its affiliates.
//
// Copyright (c) 2022 Cisco and/or its affiliates.
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

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"github.com/networkservicemesh/sdk/pkg/tools/log"
)

type keyFunc func(event watch.Event) (string, error)
type subnetFunc func(event watch.Event) (*net.IPNet, error)

func watchPodCIDR(ctx context.Context, clientset kubernetes.Interface) (<-chan []string, error) {
	nodeWatcher, err := clientset.CoreV1().Nodes().Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.FromContext(ctx).Errorf("error creating nodeWatcher: %v", err)
		return nil, err
	}

	nodeCast := func(obj runtime.Object) (*v1.Node, error) {
		node, ok := obj.(*v1.Node)
		if !ok {
			return nil, errors.Errorf("Casting object to *v1.Service failed. Object: %v", obj)
		}
		return node, nil
	}

	keyFunc := func(event watch.Event) (string, error) {
		node, err := nodeCast(event.Object)
		if err != nil {
			return "", err
		}
		return node.Name, nil
	}

	subnetFunc := func(event watch.Event) (*net.IPNet, error) {
		node, err := nodeCast(event.Object)
		if err != nil {
			return nil, err
		}
		_, ipNet, err := net.ParseCIDR(node.Spec.PodCIDR)
		if err != nil {
			return nil, err
		}
		return ipNet, nil
	}

	return WatchSubnet(ctx, nodeWatcher, keyFunc, subnetFunc)
}

func watchServiceIPAddr(ctx context.Context, cs kubernetes.Interface) (<-chan []string, error) {
	serviceWatcher, err := cs.CoreV1().Services(metav1.NamespaceAll).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		log.FromContext(ctx).Errorf("error creating serviceWatcher: %v", err)
		return nil, err
	}
	go func() {
		<-ctx.Done()
		serviceWatcher.Stop()
	}()

	serviceCast := func(obj runtime.Object) (*v1.Service, error) {
		service, ok := obj.(*v1.Service)
		if !ok {
			return nil, errors.Errorf("Casting object to *v1.Service failed. Object: %v", obj)
		}
		return service, nil
	}

	keyFunc := func(event watch.Event) (string, error) {
		service, err := serviceCast(event.Object)
		if err != nil {
			return "", err
		}
		return service.Name, nil
	}

	subnetFunc := func(event watch.Event) (*net.IPNet, error) {
		service, err := serviceCast(event.Object)
		if err != nil {
			return nil, err
		}
		ipAddr := service.Spec.ClusterIP
		return ipToNet(net.ParseIP(ipAddr)), nil
	}

	return WatchSubnet(ctx, serviceWatcher, keyFunc, subnetFunc)
}

func ipToNet(ipAddr net.IP) *net.IPNet {
	mask := net.CIDRMask(len(ipAddr)*8, len(ipAddr)*8)
	return &net.IPNet{IP: ipAddr, Mask: mask}
}

// WatchSubnet waits for subnets from resourceWatcher, gets subnetwork from watch.Event using subnetFunc.
// All subnets received from resourceWatcher will be forwarded to prefixCh of returned SubnetWatcher.
func WatchSubnet(ctx context.Context, resourceWatcher watch.Interface,
	keyFunc keyFunc, subnetFunc subnetFunc) (<-chan []string, error) {
	prefixesCh := make(chan []string, 10)

	var prefixes = make(map[string]struct{})

	go func() {
		defer resourceWatcher.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-resourceWatcher.ResultChan():
				if !ok {
					close(prefixesCh)
					return
				}

				if event.Type == watch.Error {
					continue
				}

				ipNet, err := subnetFunc(event)
				if err != nil {
					continue
				}

				if _, ok := prefixes[ipNet.String()]; ok && event.Type != watch.Deleted {
					continue
				}
				if event.Type == watch.Deleted {
					delete(prefixes, ipNet.String())
				} else {
					prefixes[ipNet.String()] = struct{}{}
				}
				var actualPrefixes []string

				for p := range prefixes {
					actualPrefixes = append(actualPrefixes, p)
				}

				prefixesCh <- actualPrefixes
			}
		}
	}()

	return prefixesCh, nil
}
