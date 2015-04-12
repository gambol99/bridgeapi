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
	"net"
	"net/http"
	"net/url"

	"github.com/gambol99/bridgeapi/pkg/bridge/client"

	"github.com/gorilla/mux"
)

const (
	DEFAULT_API_INTERFACE = "127.0.0.1"
	DEFAULT_API_PORT      = "8989"
	DEFAULT_API_BINDING   = DEFAULT_API_INTERFACE + ":" + DEFAULT_API_PORT
	SESSION_REQUEST       = "sess_request"
	SESSION_HIJACKED      = "sess_hijacked"
)

//
// The Bridge act as a hook the the API, as requests a passed through the chain,
// the bridge is called to see if anyone is listening to the API hook, forwards it on,
// hands back the mutated resource and allows it to continue;
//
type Bridge interface {
	// retrieve the confir
	Config() *Config
	// a hook event
	HookEvent(string, string, []byte) error
	// hand back a list of subscriptions
	Subscriptions() []*client.Subscription
	// add a new subscription
	AddSubscription(*client.Subscription) (string, error)
	// remove a subscription from the bridge
	DeleteSubscription(string) error
	// shutdown and release the resources
	Close() error
}

// an interface for the pipe
type Pipe interface {
	Close() error
}

// A pipe is source and sink connection to the docker api
// i.e. create unix://var/run/docker.sock -> tcp://0.0.0.0:2375
type PipeImpl struct {
	// the source of pipe
	source *url.URL
	// the sink of the pipe
	sink *url.URL
	// a reference to the bridge
	bridge Bridge
	// the listener for the service
	listener net.Listener
	// the http service running at source
	server *http.Server
	// the http clinet
	client *http.Client
	// the mux router
	router *mux.Router
}

// the configuration for the bridge
type Config struct {
	// the interface spec the admin api bind to
	Bind string `json:"bind"`
	// the token used to authenticate
	Token string `json:"token"`
	// a map of the registrations
	Subscriptions []*client.Subscription `json:"subscriptions"`
	// an array of source and sinks -H tcp://0.0.0.0:3000,unix://var/run/docker.sock
	Pipes []string `json:"pipes"`
	// the verbose level
	Verbosity int `json:"verbose"`
}

// The REST API for processing dynamic hook registration
type API struct {
	// the rest api router
	router *mux.Router
	// the http server
	server *http.Server
	// a reference to the bridge
	bridge Bridge
}

type Subscribers struct {

}

type Subscriber struct {
	Location string
	//

}
