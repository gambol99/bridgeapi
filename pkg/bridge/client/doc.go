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

package client

import (
	"fmt"
	"time"
)

const (
	DEFAULT_BRIDGE_ENDPOINT   = "http://127.0.0.1:8989/"
	DEFAULT_BINDING_PORT      = "8787"
	DEFAULT_BINDING_INTERFACE = "0.0.0.0"
	DEFAULT_BINDING           = DEFAULT_BINDING_INTERFACE + ":" + DEFAULT_BINDING_PORT
	PRE_EVENT                 = "PRE"
	POST_EVENT                = "POST"

	API_VERSION      = "/v1"
	API_PING         = API_VERSION + "/ping"
	API_SUBSCRIPTION = API_VERSION + "/subscriptions"
)

// the configuration of the client
type Config struct {
	// the rest endpoint for the bridge io server - e.g http://127.0.0.1:8989
	Bridge string `json:"bridge"`
	// the binding for the client i.e. where the endpoint will run
	Binding string `json:"bind"`
	// the token used to connect to bridge
	Token string `json:"token"`
	// the max time to wait for a request to fulfil
	MaxTime time.Duration
}

// The client interface
type Client interface {
	// close the resource and unregister if required
	Close() error
	// register a hook in the API
	Subscribe(*Subscription, RequestsChannel) (string, error)
	// unsubscribe from the provider
	Unsubscribe(string) error
	// list the current subscriptions
	Subscriptions() ([]*Subscription, error)
}

// a channel for the below
type RequestsChannel chan *Event

// A structure to define and incoming request
type Event struct {
	// an id for the origin - normally the hostname
	ID string `json:"id"`
	// a timestamp for the request
	Stamp time.Time `json:"timestamp"`
	// the hook type
	HookType string `json:"type"`
	// the uri for the request
	URI string `json:"uri"`
	// the query string
	Query string `json:"query"`
	// the payload itself
	Request string `json:"request"`
	// the channel to send the response on
	response RequestsChannel
}

func (r Event) String() string {
	return fmt.Sprintf("time: %s, uri: %s, request: %s",
		r.Stamp, r.URI, r.Request)
}

func (r *Event) Respond() {
	r.response <- r
}

// A registration request structure: used buy the client register for hook events
// into the API
type Subscription struct {
	// an application ID
	ID string `json:"id"`
	// the endpoint to send these requests
	Subscriber string `json:"subscriber"`
	// an array of hook requests
	Requests []*Hook `json:"hooks"`
}

// A Hook definition / request for access to the API
type Hook struct {
	// should this hook be enforcing, i.e. if were not able to contact you, kill the request
	Enforcing bool `json:"enforcing"`
	// when the hook should be fired, i.e. pre, post or both
	HookType string `json:"type"`
	// the entry point for the request, a regex applied to URI
	URI string `json:"uri"`
}

// Response to the requests

type MessageResponse struct {
	Message string `json:"message"`
}

type SubscriptionResponse struct {
	ID string `json:"id"`
}

type SubscriptionsResponse struct {
	Subscriptions []*Subscription `json:"subscriptions"`
}
