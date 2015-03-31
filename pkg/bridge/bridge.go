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
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/gambol99/bridge.io/pkg/bridge/client"

	log "github.com/Sirupsen/logrus"


	"bytes"
	"encoding/json"
	"strings"
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
	subscriptions []*client.Subscription
	// the bridge api server
	api *BridgeAPI
	// the client used to connecting to the subscribers
	client *http.Client
}

// Create a new Bridge from the configuration
//	cfg:		the bridge configuration reference
func NewBridge(cfg *Config) (Bridge, error) {
	var err error
	bridge := &BridgeImpl{
		config:        cfg,
		subscriptions: make([]*client.Subscription, 0),
	}
	bridge.client = &http.Client{}

	// step: create an bridge api
	if bridge.api, err = NewBridgeAPI(cfg, bridge); err != nil {
		log.Errorf("Failed to create the Bridge API, error: %s", err)
		return nil, err
	}

	return bridge, nil
}

// Close and release any resource being used by the bride
func (b *BridgeImpl) Close() error {

	return nil
}

func (b *BridgeImpl) Add(subscription *client.Subscription) (string, error) {
	log.Infof("Attempting to add the subscription: %s", subscription)
	// step: validate the hook
	if err := subscription.Valid(); err != nil {
		log.Errorf("Invalid subscription request: %V, error: %s", err)
		return "", err
	}
	b.Lock()
	defer b.Unlock()
	subscription.SubscriptionID = b.generateSubscriptionID()
	b.subscriptions = append(b.subscriptions, subscription)
	return subscription.SubscriptionID, nil
}

// Remove the subscription from the bridge
// 	id:			the subscription id which was given on subscribe()
func (b *BridgeImpl) Remove(id string) error {
	log.Infof("Attempting to remove the subscription id: %s", id)
	if id == "" || len(id) < SUBSCRIPTION_ID_LENGTH {
		return fmt.Errorf("Invalid subscription id, please check")
	}

	b.Lock()
	defer b.Unlock()
	sub_index := -1
	for index, subscription := range b.subscriptions {
		if subscription.SubscriptionID == id {
			sub_index = index
			break
		}
	}

	if sub_index < 0 {
		return fmt.Errorf("The subscription id: %s does not exists", id)
	}

	b.subscriptions = append(b.subscriptions[:sub_index], b.subscriptions[sub_index+1])
	return nil
}

// Called on a prehook event, i.e. when a client *first* makes a request to the API, but *before*
// its been forwarded to the sink
//  uri:		the uri of the resource
//	request:	the content of the request
func (b *BridgeImpl) PreHookEvent(uri string, request []byte) ([]byte, error) {
	log.Infof("Bridge recieved a pre hook request, uri: %s", uri)
	forwarders := b.getListeners(uri, client.PRE_EVENT)
	if len(forwarders) <= 0 {
		log.Infof("Found %d subscribers listening out for: %s", len(forwarders), uri)
		return request, nil
	}
	// step: we call each of the subscribers in turn
	api_request := new(client.APIRequest)
	api_request.ID, _ = os.Hostname()
	api_request.Stamp = time.Now()
	api_request.HookType = client.PRE_EVENT
	api_request.Request = string(request)
	api_request.URI = uri

	for _, l := range forwarders {

		log.Debugf("Forwarding the request uri: %s to subscriber: %s", uri, l.Endpoint)
		// step: forward the request on to the subscriber
		response, err := b.performHTTP("POST", l.Endpoint, request)
		if err != nil {
			log.Errorf("Failed to call the subscriber: %s, error: %s", l.Endpoint, err)
			continue
		}

		// step: decode the result
		err = json.NewDecoder(strings.NewReader(string(response))).Decode(api_request)
		if err != nil {
			log.Errorf("Failed to decode the response from subscriber: %s, error: %s", l.Endpoint, err)
			continue
		}

		log.Debugf("Response from subscribe: %V", api_request)

		request = []byte(api_request.Request)
	}
	return request, nil
}

// Called on a posthook event, i.e. the response from the sink
//  uri:		the uri of the resource
//	request:	the content of the request
func (b *BridgeImpl) PostHookEvent(uri string, request []byte) ([]byte, error) {
	log.Infof("Bridge recieved a post hook request, uri: %s", uri)
	forwarders := b.getListeners(uri, client.POST_EVENT)
	if len(forwarders) <= 0 {
		log.Infof("Found %d subscribers listening out for: %s", len(forwarders), uri)
		return request, nil
	}

	return request, nil
}

// Retrieve the current subscriptions which are in the bridge
func (b *BridgeImpl) Subscriptions() []*client.Subscription {
	b.RLock()
	defer b.RUnlock()
	return b.subscriptions
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
	log.Debugf("Dialing the subscriber: %s:%s", u.Scheme, b.dialHost(u))
	dial, err := net.Dial(u.Scheme, b.dialHost(u))
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

func (r *BridgeImpl) dialHost(host *url.URL) (string) {
	if host.Scheme == "unix" {
		return fmt.Sprintf("/%s%s", host.Host, host.Path)
	}
	return host.Host
}

func (b *BridgeImpl) getListeners(uri, hook_type string) []*client.Subscription {
	b.RLock()
	defer b.RUnlock()
	forwarders := make([]*client.Subscription, 0)
	// step: we build a list of subscribers for this uri
	for _, subscription := range b.subscriptions {
		for _, hook := range subscription.Requests {
			if hook.HookType == hook_type {
				if matched, err := regexp.MatchString(hook.URI, uri); err != nil {
					log.Errorf("The regex for the hook: %s is invalid, error: %s", err)
				} else if matched {
					forwarders = append(forwarders, subscription)
				}
			}
		}
	}
	return forwarders
}

func (b *BridgeImpl) generateSubscriptionID() string {
	numbers := []rune("0123456789")
	id := make([]rune, SUBSCRIPTION_ID_LENGTH)
	for i := range id {
		id[i] = numbers[rand.Intn(len(numbers))]
	}
	return string(id)
}
