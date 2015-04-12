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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gambol99/bridgeapi/pkg/bridge/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

// the implementation of the client
type ClientImpl struct {
	// the url for the bridge
	bridge *url.URL
	// the url for the http service
	server_url *url.URL
	// the http client for us
	client *http.Client
	// the channel we should be sending requests to
	requests RequestsChannel
	// the configuration of the client
	config *Config
	// the http server
	server *http.Server
	// the listener interface
	listener net.Listener
	// the router for the http service
	router *mux.Router
	// a list of our subscriptions
	subscriptions []string
}

func DefaultConfig() *Config {
	return &Config{
		Bridge:  DEFAULT_BRIDGE_ENDPOINT,
		Binding: DEFAULT_BINDING,
		Token: "",
		MaxTime: (time.Duration(300) * time.Second),
	}
}

// Create a new client to communicate with the bridge
//	config:		 	the configuration to use with the client
func NewClient(cfg *Config) (Client, error) {
	var err error
	if cfg == nil {
		return nil, errors.New("you have not specified any configuration")
	}
	client := new(ClientImpl)
	if client.bridge, err = url.Parse(cfg.Bridge); err != nil {
		return nil, fmt.Errorf("Invalid bridge specified: %s", cfg.Bridge)
	}
	client.client = &http.Client{}
	client.requests = make(RequestsChannel, 10)
	client.config = cfg
	client.subscriptions = make([]string, 0)
	if err := client.setupHttpService(client); err != nil {
		return nil, fmt.Errorf("Failed to create the http service, error: %s", err)
	}
	return client, nil
}

// Request to close off any resources, disconnect our self as an endpoint (if required)
// and close the client
func (r *ClientImpl) Close() error {
	for _, id := range r.subscriptions {
		log.Debugf("Unsubscribing subscription: %s", id)
		r.Unsubscribe(id)
	}
	return nil
}

// Retrieve a list of subscriptions from the bridge
func (r *ClientImpl) Subscriptions() ([]*Subscription, error) {
	list := new(SubscriptionsResponse)
	_, err := r.send("GET", API_SUBSCRIPTION, nil, list)
	if err != nil {
		log.Debugf("Failed to retrieve a list of subscriptions, error: %s", err)
		return nil, err
	}
	return list.Subscriptions, nil
}

// Perform a registration request to the bridge
// 	register:		the registration structure containing the hooks
//  channel:		the channel you want to receive the api requests on
func (r *ClientImpl) Subscribe(subscription *Subscription, channel RequestsChannel) (string, error) {
	// step: validate the subscription
	if err := subscription.Valid(); err != nil {
		log.Debugf("The subscription: %s is not valid, error: %s", subscription, err)
		return "", err
	}

	// step: register the subscription
	r.requests = channel
	response := new(SubscriptionResponse)
	code, err := r.send("POST", API_SUBSCRIPTION, subscription, response)
	if err != nil {
		return "", err
	}
	if code != 200 {
		return "", fmt.Errorf("status: %d", code)
	}
	// step: we add to the list
	r.subscriptions = append(r.subscriptions, response.ID)
	return response.ID, nil
}

// Unregister from the bridge.io service
// 	id:				the registration id which was given when you registered
func (r *ClientImpl) Unsubscribe(id string) error {
	code, err := r.send("DELETE", fmt.Sprintf("%s/%s", API_SUBSCRIPTION, id), nil, nil)
	if err != nil {
		log.Debugf("Failed to unsubscribe from the bridge, error: %s", err)
		return fmt.Errorf("unable to unsubscribe id: %s, error: %s", id, err)
	}
	if code != 200 {
		return fmt.Errorf("failed to unsubscribe the id: %s", id)
	}
	return nil
}

// Setup the http server
// 	client:			the Client implementation reference
func (r *ClientImpl) setupHttpService(client *ClientImpl) error {
	var err error
	// step: parse the binding
	if client.server_url, err = url.Parse(client.config.Binding); err != nil {
		return errors.New(fmt.Sprintf("invalid http binding, error: %s", err))
	}
	log.Debugf("Parsing the client binding address: %s:%s", client.server_url.Scheme, client.server_url.Host)

	// step: create a listener for the interface
	if client.listener, err = net.Listen(client.server_url.Scheme, client.server_url.Host); err != nil {
		return errors.New(fmt.Sprintf("unable to create the listener, error: %s", err))
	}

	// step: setup the routing
	log.Debugf("Creating the router for the client listener service")
	middleware := alice.New(client.recoveryHandler, client.loggingHandler).ThenFunc(client.requestHandler)
	router := mux.NewRouter()
	router.Handle("/", middleware)
	client.router = router

	// step: create the http server
	client.server = &http.Server{
		Handler:        router,
		ReadTimeout:    time.Duration(120) * time.Second,
		WriteTimeout:   time.Duration(120) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go client.server.Serve(client.listener)
	return nil
}

func (r *ClientImpl) recoveryHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, http.StatusText(500), 500)
			}
		}()
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func (r *ClientImpl) loggingHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		log.Debugf("Recieved request from: %s", request.RemoteAddr)
		next.ServeHTTP(w, request)
	}
	return http.HandlerFunc(fn)
}

func (r *ClientImpl) requestHandler(writer http.ResponseWriter, request *http.Request) {

	event := new(Event)
	// step: we parse and decode the request and send on the channel
	if err := utils.HttpJsonDecode(request.Body, request.ContentLength, event); err != nil {
		log.Debugf("Failed to decode the request from: %s, error: %s", request.RemoteAddr, err)
		return
	}

	log.Debugf("Request from: %s, content: %s", request.RemoteAddr, event)

	// step: we create a channel for sending the response back to the client and pass
	// the reference in the api request struct. To ensure we don't end up with a plethora of
	// these, we have a fail-safe timer
	event.response = make(RequestsChannel)

	go func() {
		r.requests <- event
	}()

	// step: wait for a response from the consumer and reply back to the client
	select {
	case response := <-event.response:
		log.Debugf("Recieved the response from client, sending back the response")

		// step: we encode the api request
		var content bytes.Buffer
		if err := json.NewEncoder(&content).Encode(response); err != nil {
			panic("failed to encode the api request")
		}

		// step: send the content back
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(200)
		writer.Write(content.Bytes())

	case <-time.After(r.config.MaxTime):
		panic("we have timed out wait for the client to process the request")
	}
}

// Send a json request to the bridge, get and decode the response
//	uri:		the uri on the bridge to target
//  result:		the data structure we should decode into
func (r *ClientImpl) send(method, uri string, post_data, result interface{}) (int, error) {
	full_url := fmt.Sprintf("%s://%s/%s", r.bridge.Scheme, utils.Dial(r.bridge), uri)
	status_code, err := utils.HttpJsonSend(method, full_url, post_data, result, 30)
	if err != nil {
		log.Errorf("unable to process request to bridge: %s, error: %s", full_url, err)
	}
	log.Debugf("Recieved response from request, code: %s, payload: %V", status_code, result)
	return status_code, nil
}
