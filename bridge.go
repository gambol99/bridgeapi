/*
Copyright 2014 Rohith All rights reserved.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"sync"

	"github.com/gambol99/bridge.io/client"

	log "github.com/Sirupsen/logrus"
)

// the bridge implementation
type BridgeImpl struct {
	sync.RWMutex
	// the configuration
	config *Config
	// the prehooks
	subscriptions []*client.Subscription
	// api
	api *BridgeAPI
}

// Create a new bridge
func NewBridge(cfg *Config) (Bridge, error) {
	log.Infof("Creating a new docker bridge")
	var err error
	bridge := &BridgeImpl{
		config: cfg,
		subscriptions:  make([]*client.Subscription, 0),
	}

	// step: create an bridge api
	if bridge.api, err = NewBridgeAPI(cfg, bridge); err != nil {
		log.Errorf("Failed to create the Bridge API, error: %s", err)
		return nil, err
	}

	return bridge, nil
}

func (b *BridgeImpl) PreHookEvent() error {

	return nil
}

func (b *BridgeImpl) PostHookEvent() error {

	return nil
}

func (b *BridgeImpl) Subscriptions() []*client.Subscription {
	return b.subscriptions
}
