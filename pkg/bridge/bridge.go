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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gambol99/bridgeapi/pkg/bridge/client"
	"github.com/gambol99/bridgeapi/pkg/bridge/utils"

	log "github.com/Sirupsen/logrus"
)

const (
	SUBSCRIPTION_MIN_ID = 32
)

// the bridge implementation
type BridgeImpl struct {
	sync.RWMutex
	// the configuration
	config *Config
	// the bridge api server
	api *API
	// an array of subscriptions
	subscribers []*client.Subscription
	// a map of id to subscriptions
	subscribersMap map[string]*client.Subscription
}

// Create a new Bridge from the configuration
//	cfg:		the bridge configuration reference
func NewBridge(cfg *Config) (Bridge, error) {
	bridge := new(BridgeImpl)
	bridge.config = cfg
	bridge.subscribers = make([]*client.Subscription, 0)
	bridge.subscribersMap = make(map[string]*client.Subscription, 0)
	if api, err := NewAPI(bridge); err != nil {
		return nil, err
	} else {
		bridge.api = api
	}
	return bridge, nil
}

// Close and release any resource being used by the bride
func (b *BridgeImpl) Close() error {

	return nil
}

// Return a pointer to the configuration
func (b *BridgeImpl) Config() *Config {
	return b.config
}

// Add a new subscription to the bridge
// 	subscription:		a pointer to the Subscription
func (b *BridgeImpl) AddSubscription(subscription *client.Subscription) (string, error) {
	b.Lock()
	defer b.Unlock()
	log.Infof("Attempting to add the subscription: %s", subscription)

	// step: we validate the subscription
	if err := subscription.Valid(); err != nil {
		log.Errorf("Invalid subscription request: %V, error: %s", err)
		return "", err
	}

	// step: we check if a subscription already exists
	if _, found := b.subscribersMap[subscription.ID]; found {
		return "", fmt.Errorf("a subscription already exists with this ID: %s", subscription.ID)
	}

	// step: we add to the list of subscribers
	subscriptionID := utils.RandomUUID(SUBSCRIPTION_MIN_ID)
	b.subscribersMap[subscriptionID] = subscription
	b.subscribers = append(b.subscribers, subscription)
	log.Infof("Added subscription: %s, id: %s", subscription, subscriptionID)
	return subscriptionID, nil
}

// Remove the subscription from the bridge
// 	id:			the subscription id which was given on subscribe()
func (b *BridgeImpl) DeleteSubscription(ID string) error {
	log.Infof("Attempting to remove the subscription id: %s", ID)
	// step: valid the token
	if ID == "" || len(ID) < SUBSCRIPTION_MIN_ID {
		return fmt.Errorf("invalid subscription id")
	}
	b.Lock()
	defer b.Unlock()
	// step: check the token exists
	if _, found := b.subscribersMap[ID]; !found {
		return fmt.Errorf("the subscription id: %s does not exist", ID)
	}
	log.Infof("Removing subscription: %s from the bridge", ID)
	// step: remove from the map
	delete(b.subscribersMap, ID)
	for index, subscription := range b.subscribers {
		if subscription.ID == ID {
			b.subscribers = append(b.subscribers[:index], b.subscribers[index+1])
			return nil
		}
	}
	return fmt.Errorf("internal error, we shouldn't have reached here")
}

func (b *BridgeImpl) HookEvent(uri, event_type string, request []byte) error {
	log.Debugf("Recieved a hook event, uri: %s, type: %s, request: %s", uri, event_type, string(request))

	// step: we need to check if anyone is hook to this uri?
	subscribers, err := b.filterSubscribers(uri, event_type)
	if err != nil {
		log.Errorf("Failed to retrieve a list of subscribers, error: %s", err)
		return err
	}
	log.Debugf("Found %d subscribers to the uri: %s, processing now", len(subscribers))

	// step: anyone to pass to?
	if subscribers == nil || len(subscribers) <= 0 {
		return nil
	}

	// step: we call each of the subscribers in turn
	payload := &client.Event{
		ID:       "bridge",
		Stamp:    time.Now(),
		HookType: event_type,
		URI:      uri,
		Query:    "",
		Request:  string(request),
	}

	// step: we iterate the subscribers and forward on the request
	for _, sub := range subscribers {
		log.Debugf("Forwarding the request uri: %s to subscriber: %s", uri, sub.Subscriber)

		// step: forward the request on to the subscriber
		response, err := b.performHTTP("POST", sub.Subscriber, request)
		if err != nil {
			log.Errorf("Failed to call the subscriber: %s, error: %s", sub.Subscriber, err)
			continue
		}

		// step: decode the result
		if err = json.NewDecoder(strings.NewReader(string(response))).Decode(payload); err != nil {
			log.Errorf("Failed to decode the response from subscriber: %s, error: %s", sub.Subscriber, err)
			continue
		}

		log.Debugf("Response from subscribe: %V", payload)

		request = []byte(payload.Request)
	}
	return nil
}

func (b *BridgeImpl) filterSubscribers(uri, event string) ([]*client.Subscription, error) {



	return nil, nil
}

// Retrieve the current subscriptions which are in the bridge
func (b *BridgeImpl) Subscriptions() []*client.Subscription {
	b.RLock()
	defer b.RUnlock()
	return b.subscribers
}

func (b *BridgeImpl) performHTTP(method, endpoint string, data []byte) ([]byte, error) {

	// step: parse the endpoint
	u, err := url.Parse(endpoint)
	if err != nil {
		log.Errorf("Failed to parse the subscription endpoint, error: %s", err)
		return []byte{}, err
	}

	// step: we compose the request
	request, err := http.NewRequest(method, "/", bytes.NewReader(data))
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to compose the request to the bridge, error: %s", err)
	}
	request.Header.Set("Content-Type", "application/json")

	// step: create the dialing client
	log.Debugf("Dialing the subscriber: %s:%s", u.Scheme, utils.Dial(u))
	dial, err := net.Dial(u.Scheme, utils.Dial(u))
	if err != nil {
		log.Debugf("Failed to dial the bridge, error: %s", err)
		return []byte{}, err
	}

	http_client := httputil.NewClientConn(dial, nil)
	defer http_client.Close()

	// perform the request to api (sink)
	log.Debugf("Performing the request on subscriber: %s", endpoint)
	response, err := http_client.Do(request)
	if err != nil {
		return []byte{}, fmt.Errorf("Failed to perform request to endpoint: %s, error: %s", endpoint, err)
	}

	// step: read in the response from the client
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorf("Failed to read the response boxy from subscriber: %s, error: %s", endpoint, err)
		return []byte{}, fmt.Errorf("Failed to readin the request body from endpoint: %s, error: %s", endpoint, err)
	}

	return content, nil
}
