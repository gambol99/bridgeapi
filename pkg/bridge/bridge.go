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

package bridge

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/gambol99/bridge.io/pkg/bridge/client"

	log "github.com/Sirupsen/logrus"
)

const (
	SUBSCRIPTION_ID_LENGTH = 32
)

// the bridge implementation
type BridgeImpl struct {
	sync.RWMutex
	// the configuration
	config *Config
	// the subscriptions
	subscriptions map[string]*client.Subscription
	// the bridge api server
	api *BridgeAPI
}

// Create a new bridge
func NewBridge(cfg *Config) (Bridge, error) {
	var err error
	bridge := &BridgeImpl{
		config:        cfg,
		subscriptions: make(map[string]*client.Subscription, 0),
	}

	// step: create an bridge api
	if bridge.api, err = NewBridgeAPI(cfg, bridge); err != nil {
		log.Errorf("Failed to create the Bridge API, error: %s", err)
		return nil, err
	}

	return bridge, nil
}

func (b *BridgeImpl) Close() error {

	return nil
}

func (b *BridgeImpl) Add(subscription *client.Subscription) (string, error) {
	log.Infof("Attempting to add the subscription: %s", subscription)
	b.Lock()
	defer b.Unlock()
	// lets create a new id
	id := b.generateID()
	b.subscriptions[id] = subscription
	return id, nil
}

// remove a subscription from the bridge
func (b *BridgeImpl) Remove(id string) error {
	log.Infof("Attempting to remove the subscription id: %s", id)
	b.Lock()
	defer b.Unlock()
	if id == "" || len(id) < SUBSCRIPTION_ID_LENGTH {
		return fmt.Errorf("Invalid subscription id, please check")
	}
	_, found := b.subscriptions[id]
	if !found {
		return fmt.Errorf("The subscription id: %s does not exists", id)
	}
	delete(b.subscriptions, id)
	return nil
}

func (b *BridgeImpl) PreHookEvent(request []byte) ([]byte, error) {
	log.Infof("Bridge recieved a pre hook request")


	return request, nil
}

func (b *BridgeImpl) PostHookEvent(request []byte) ([]byte, error) {
	log.Infof("Bridge recieved a post hook request")

	return request, nil
}

func (b *BridgeImpl) Subscriptions() map[string]*client.Subscription {
	return b.subscriptions
}

func (b *BridgeImpl) generateID() string {
	numbers := []rune("0123456789")
	id := make([]rune, SUBSCRIPTION_ID_LENGTH)
	for i := range id {
		id[i] = numbers[rand.Intn(len(numbers))]
	}
	return string(id)
}
