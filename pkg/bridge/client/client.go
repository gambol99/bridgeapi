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
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/justinas/alice"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

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
		Logger:  os.Stdout,
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

// Perform a registration request to the bridge
// 	register:		the registration structure containing the hooks
//  channel:		the channel you want to receive the api requests on
func (r *ClientImpl) Subscribe(register *Subscription, channel RequestsChannel) (string, error) {
	// step: validate the subscription
	if err := register.Valid(); err != nil {
		return "", err
	}

	// step: register the subscription
	r.requests = channel
	response := new(SubscriptionResponse)
	code, err := r.send("POST", API_SUBSCRIPTION, register, response)
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
	client.server_url, err = url.Parse(client.config.Binding)
	if err != nil {
		return errors.New(fmt.Sprintf("invalid http binding, error: %s", err))
	}
	log.Debugf("Parsing the client binding address: %s:%s", client.server_url.Scheme, client.server_url.Host)

	// step: create a listener for the interface
	client.listener, err = net.Listen(client.server_url.Scheme, client.server_url.Host)
	if err != nil {
		return errors.New(fmt.Sprintf("unable to create the listener, error: %s", err))
	}

	// step: setup the routing
	log.Debugf("Creating the router for the client listener service")
	middleware := alice.New(client.recoveryHandler).ThenFunc(client.requestHandler)
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

	// step: start listening
	go client.server.Serve(client.listener)
	return nil
}

func (pipe *ClientImpl) recoveryHandler(next http.Handler) http.Handler {
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

func (r *ClientImpl) requestHandler(writer http.ResponseWriter, request *http.Request) {
	log.Debugf("Recieved request from: %s", request.RemoteAddr)

	// step: we parse and decode the request and send on the channel
	apirq, err := r.decodeAPIRequest(request)
	if err != nil {
		log.Debugf("Failed to decode the request from: %s, error: %s", request.RemoteAddr, err)
		return
	}

	log.Debugf("Request from: %s, content: %s", request.RemoteAddr, apirq)

	// step: we create a channel for sending the response back to the client and pass
	// the reference in the api request struct. To ensure we don't end up with a plethora of
	// these, we have a fail-safe timer
	apirq.Response = make(RequestsChannel)

	go func() {
		r.requests <- apirq
	}()

	// step: wait for a response from the consumer and reply back to the client
	select {
	case response := <-apirq.Response:
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
func (r *ClientImpl) send(method, uri string, data, result interface {}) (int, error) {
	log.Debugf("Sending the request to bridge: %V", data)

	// step: encode the post data
	var buffer bytes.Buffer
	if data != nil {
		if err := json.NewEncoder(&buffer).Encode(data); err != nil {
			return 0, fmt.Errorf("Failed to encode the data for request, error: %s", err)
		}
	}

	// step: we compose the request
	request, err := http.NewRequest(method, uri, &buffer)
	if err != nil {
		return 0, fmt.Errorf("unable to compose the request to the bridge, error: %s", err)
	}

	// step: we compose the dialing to the bridge endpoint
	dial, err := net.Dial(r.bridge.Scheme, r.dialHost())
	if err != nil {
		log.Debugf("unable to dial the bridge, error: %s", err)
		return 0, err
	}

	// step: we create a connection
	http_client := httputil.NewClientConn(dial, nil)
	defer http_client.Close()

	// perform the request to api (sink)
	response, err := http_client.Do(request)
	if err != nil {
		log.Debugf("unable to post the request to bridge endpoint, error: %s", err)
		return 0, fmt.Errorf("Failed to perform bridge request: %s, error: %s", uri, err)
	}

	// step: read in the response
	if response.ContentLength > 0 || response.ContentLength < 0 {
		if response.Header.Get("Content-Type") == "application/json" {

			content, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return response.StatusCode, fmt.Errorf("Failed to read in the response body, error: %s", err)
			}

			// step: decode the response
			err = json.NewDecoder(strings.NewReader(string(content))).Decode(result)
			if err != nil {
				log.Debugf("Failed to decode json response: %s", string(content))
				return response.StatusCode, fmt.Errorf("Failed to decode the response, error: %s", err)
			}
		}
	}
	return response.StatusCode, nil
}

func (r *ClientImpl) dialHost() (string) {
	if r.bridge.Scheme == "unix" {
		return fmt.Sprintf("/%s%s", r.bridge.Host, r.bridge.Path)
	}
	return r.bridge.Host
}

