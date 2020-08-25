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
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"

	"math/big"
)

type SubnetWatcher struct {
	subnetCh <-chan *net.IPNet
	stopCh   chan struct{}
}

func (s *SubnetWatcher) Stop() {
	close(s.stopCh)
}

func (s *SubnetWatcher) ResultChan() <-chan *net.IPNet {
	return s.subnetCh
}

type keyFunc func(event watch.Event) (string, error)
type subnetFunc func(event watch.Event) (*net.IPNet, error)

func WatchPodCIDR(clientset *kubernetes.Clientset) (*SubnetWatcher, error) {
	nodeWatcher, err := clientset.CoreV1().Nodes().Watch(context.TODO(), metav1.ListOptions{})
	if err != nil {
		logrus.Error(err)
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

	return watchSubnet(nodeWatcher, keyFunc, subnetFunc)
}

func WatchServiceIpAddr(cs *kubernetes.Clientset) (*SubnetWatcher, error) {
	serviceWatcher, err := newServiceWatcher(cs)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

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
		return IpToNet(net.ParseIP(ipAddr)), nil
	}

	return watchSubnet(serviceWatcher, keyFunc, subnetFunc)
}

func IpToNet(ipAddr net.IP) *net.IPNet {
	mask := net.CIDRMask(len(ipAddr)*8, len(ipAddr)*8)
	return &net.IPNet{IP: ipAddr, Mask: mask}
}

type serviceWatcher struct {
	resultCh chan watch.Event
	stopCh   chan struct{}
}

func (s *serviceWatcher) Stop() {
	close(s.stopCh)
}

func (s *serviceWatcher) ResultChan() <-chan watch.Event {
	return s.resultCh
}

func newServiceWatcher(cs *kubernetes.Clientset) (watch.Interface, error) {
	ns, err := getNamespaces(cs)
	if err != nil {
		return nil, err
	}
	resultCh := make(chan watch.Event, 10)
	stopCh := make(chan struct{})

	for _, n := range ns {
		w, err := cs.CoreV1().Services(n).Watch(context.TODO(), metav1.ListOptions{})
		if err != nil {
			logrus.Errorf("Unable to watch services in %v namespace: %v", n, err)
			close(stopCh)
			return nil, err
		}

		go func() {
			for {
				select {
				case <-stopCh:
					return
				case e := <-w.ResultChan():
					resultCh <- e
				}
			}
		}()
	}

	return &serviceWatcher{
		resultCh: resultCh,
		stopCh:   stopCh,
	}, nil
}

func getNamespaces(cs *kubernetes.Clientset) ([]string, error) {
	ns, err := cs.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	rv := []string{}
	for i := range ns.Items {
		rv = append(rv, ns.Items[i].Name)
	}
	return rv, nil
}

func watchSubnet(resourceWatcher watch.Interface, keyFunc keyFunc, subnetFunc subnetFunc) (*SubnetWatcher, error) {
	subnetCh := make(chan *net.IPNet, 10)
	stopCh := make(chan struct{})

	cache := map[string]string{}
	var lastIpNet *net.IPNet

	go func() {
		for {
			select {
			case <-stopCh:
				resourceWatcher.Stop()
				return
			case event, ok := <-resourceWatcher.ResultChan():
				if !ok {
					close(subnetCh)
					return
				}

				if event.Type == watch.Error || event.Type == watch.Deleted {
					continue
				}

				ipNet, err := subnetFunc(event)
				if err != nil {
					continue
				}

				key, err := keyFunc(event)
				if err != nil {
					continue
				}
				logrus.Infof("Receive resource: name %v, subnet %v", key, ipNet.String())

				if subnet, exist := cache[key]; exist && subnet == ipNet.String() {
					continue
				}
				cache[key] = ipNet.String()

				if lastIpNet == nil {
					lastIpNet = ipNet
					subnetCh <- lastIpNet
					continue
				}

				newIpNet := MaxCommonPrefixSubnet(lastIpNet, ipNet)
				if newIpNet.String() != lastIpNet.String() {
					logrus.Infof("Subnet extended from %v to %v", lastIpNet, newIpNet)
					lastIpNet = newIpNet
					subnetCh <- lastIpNet
					continue
				}
			}
		}
	}()

	return &SubnetWatcher{
		subnetCh: subnetCh,
		stopCh:   stopCh,
	}, nil
}

func MaxCommonPrefixSubnet(s1, s2 *net.IPNet) *net.IPNet {
	rawIP1, n1 := fromIP(s1.IP)
	rawIP2, _ := fromIP(s2.IP)

	xored := &big.Int{}
	xored.Xor(rawIP1, rawIP2)
	maskSize := leadingZeros(xored, n1)

	m1, bits := s1.Mask.Size()
	m2, _ := s2.Mask.Size()

	mask := net.CIDRMask(min(min(m1, m2), maskSize), bits)
	return &net.IPNet{
		IP:   s1.IP.Mask(mask),
		Mask: mask,
	}
}

func fromIP(ip net.IP) (ipVal *big.Int, ipLen int) {
	val := &big.Int{}
	val.SetBytes([]byte(ip))
	i := len(ip)
	if i == net.IPv4len {
		return val, 32
	} // else if i == net.IPv6len
	return val, 128
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func leadingZeros(n *big.Int, size int) int {
	i := size - 1
	for ; n.Bit(i) == 0 && i > 0; i-- {
	}
	return size - 1 - i
}
