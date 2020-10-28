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

package prefixsource_test

import (
	"cmd-exclude-prefixes-k8s/internal/prefixcollector/prefixsource"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"go.uber.org/goleak"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

type dummyResource struct {
	name   string
	subnet string
}

func (*dummyResource) GetObjectKind() schema.ObjectKind {
	panic("implement me")
}

func (*dummyResource) DeepCopyObject() runtime.Object {
	panic("implement me")
}

func keyFuncDummy(event watch.Event) (string, error) {
	return event.Object.(*dummyResource).name, nil
}

func subnetFuncDummy(event watch.Event) (*net.IPNet, error) {
	_, ipNet, _ := net.ParseCIDR(event.Object.(*dummyResource).subnet)
	return ipNet, nil
}

// DummyWatcher is dummy implementation of watch.Interface
type DummyWatcher struct {
	eventCh chan watch.Event
}

// NewDummyWatcher returns new DummyWatcher
func NewDummyWatcher() *DummyWatcher {
	return &DummyWatcher{
		eventCh: make(chan watch.Event),
	}
}

func (d *DummyWatcher) Stop() {
	close(d.eventCh)
}

func (d *DummyWatcher) ResultChan() <-chan watch.Event {
	return d.eventCh
}

func (d *DummyWatcher) send(t watch.EventType, dr runtime.Object) {
	d.eventCh <- watch.Event{
		Type:   t,
		Object: dr,
	}
}

func checkSubnetWatcher(t *testing.T, subnetSequence, expectedSequence []string) {
	g := NewWithT(t)

	dw := NewDummyWatcher()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subnetChan, err := prefixsource.WatchSubnet(ctx, dw, keyFuncDummy, subnetFuncDummy)
	g.Expect(err).To(BeNil())

	for i := 0; i < len(subnetSequence); i++ {
		dw.send(watch.Added, &dummyResource{fmt.Sprintf("r%d", i), subnetSequence[i]})
	}

	for i := 0; i < len(expectedSequence); i++ {
		select {
		case e := <-subnetChan:
			g.Expect(e.String()).To(Equal(expectedSequence[i]))
		case <-time.After(1 * time.Second):
			if expectedSequence[i] != "-" {
				logrus.Error("Timeout waiting for next subnet")
				t.Fail()
			}
		}
	}
}

func TestSimpleSubnetCollector(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	subnetSequence := []string{
		"10.20.1.0/24",
		"10.20.2.0/24",
	}
	expectedSubnets := []string{
		"10.20.1.0/24",
		"10.20.0.0/22",
	}
	checkSubnetWatcher(t, subnetSequence, expectedSubnets)
}

func TestLastIpsAlreadyInSubnet(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	subnetSequence := []string{
		"10.96.10.10/32",
		"10.98.2.0/32",
		"10.98.2.255/32",
		"10.99.1.255/32",
	}
	expectedSubnets := []string{
		"10.96.10.10/32",
		"10.96.0.0/14",
		"-",
		"-",
	}
	checkSubnetWatcher(t, subnetSequence, expectedSubnets)
}

func TestIntermediateIpsAlreadyInSubnet(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	subnetSequence := []string{
		"10.96.10.10/32",
		"10.98.2.0/32",
		"10.98.2.255/32",
		"10.99.1.255/32",
		"10.104.1.255/32",
	}
	expectedSubnets := []string{
		"10.96.10.10/32",
		"10.96.0.0/14",
		"10.96.0.0/12",
	}
	checkSubnetWatcher(t, subnetSequence, expectedSubnets)
}

func TestIpv6(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	subnetSequence := []string{
		"100::1:0/112",
		"100::2:0/112",
	}
	expectedSubnets := []string{
		"100::1:0/112",
		"100::/110",
	}
	checkSubnetWatcher(t, subnetSequence, expectedSubnets)
}
