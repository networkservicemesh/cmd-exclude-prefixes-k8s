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

package utils

import "context"

// Notifiable is entity used for listener notification
type Notifiable interface {
	Notify()
	Wait(ctx context.Context) error
}

type channelNotifier struct {
	ch chan struct{}
}

// NewChannelNotifiable - creates new channelNotifier
func NewChannelNotifiable() Notifiable {
	return &channelNotifier{
		ch: make(chan struct{}, 1),
	}
}

// Notify - notifies listener about event
func (cn *channelNotifier) Notify() {
	cn.ch <- struct{}{}
}

// Wait - waits for notification or context cancel
func (cn *channelNotifier) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-cn.ch:
	}
	return nil
}
