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
	"time"
	"io"
	"errors"
)

const (
	DEFAULT_BRIDGE_ENDPOINT   = "http://127.0.0.1:8989/"
	DEFAULT_BINDING_PORT      = "8787"
	DEFAULT_BINDING_INTERFACE = "0.0.0.0"
	DEFAULT_BINDING           = DEFAULT_BINDING_INTERFACE+":"+DEFAULT_BINDING_PORT
)

var (
	ErrInvalidBridge = errors.New("Invalid bridge endpoint")
)

// the configuration of the client
type Config struct {
	// the rest endpoint for the bridge io server - e.g http://127.0.0.1:8989
	Bridge string `json:"bridge"`
	// the binding for the client i.e. where the endpoint will run
	Binding string `json:"bind"`
	// the output stream for logging
	Logger io.Writer
	// the max time to wait for a request to fulfil
	MaxTime time.Duration
}

// The client interface
type Client interface {
	// Close the resource and unregister if required
	Close() error
	// Register a hook in the API
	Subscribe(*Subscription, RequestsChannel) (string, error)
	// Unsubscribe from the provider
	Unsubscribe(string) error
}

// a channel for the below
type RequestsChannel chan *APIRequest

// A structure to define and incoming request
type APIRequest struct {
	// an id for the origin - normally the hostname
	ID string `json:"id"`
	// a timestamp for the request
	Stamp time.Time `json:"timestamp"`
	// the uri for the request
	URI string `json:"uri"`
	// the hook type
	HookType string `json:"type"`
	// the payload itself
	Request string `json:"request"`
	// the channel to send the response on
	Response RequestsChannel
}

// A registration request structure: used buy the client register for hook events
// into the API
type Subscription struct {
	// an application ID
	ID string `json:"id"`
	// the endpoint to send these requests
	Endpoint string `json:"endpoint"`
	// an array of hook requests
	Requests []*APIHook `json:"hooks"`
}

// Filters are used and applied against image, environment variables, hostnames, etc etc
// I.e. you can say, send me all requests from /container/*/(create|start) from hostname =~ docker101
// where it containers a environment variable of X
type Filter map[string]interface {}

//
// A Hook definition / request for access to the API
//
type APIHook struct {
	// should this hook be enforcing, i.e. if were not able to contact you, kill the request
	Enforcing bool `json:"enforcing"`
	// when the hook should be fired, i.e. pre, post or both
	HookType string `json:"type"`
	// the entrypoint for the request, a regex applied to URI
	URI string `json:"uri"`
	// a map of filters to be applied
	Filters Filter `json:"filters,omitempty"`
}


