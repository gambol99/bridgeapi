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
	"net/url"
	"os"
	"strings"
	"time"

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
}

// Create a new client to communicate with the bridge
//	config:		 	the configuration to use with the client
func NewClient(cfg *Config) (Client, error) {
	var err error
	if cfg == nil {
		return nil, errors.New("you have not specified any configuration")
	}
	// step: parse the url
	client := new(ClientImpl)
	client.bridge, err = url.Parse(cfg.Bridge)
	if err != nil {
		return nil, ErrInvalidBridge
	}
	client.client = &http.Client{}
	client.requests = make(RequestsChannel, 10)
	client.config = cfg

	// step: start listening to requests from the
	err = client.setupHttpService(client)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func DefaultConfig() *Config {
	return &Config{
		Bridge:  DEFAULT_BRIDGE_ENDPOINT,
		Binding: DEFAULT_BINDING,
		Logger:  os.Stdout,
		MaxTime: (time.Duration(300) * time.Second),
	}
}

// Request to close off any resources, disconnect our self as an endpoint (if required)
// and close the client
func (r *ClientImpl) Close() error {

	return nil
}

// Perform a registration request to the bridge
// 	register:		the registration structure containing the hooks
//  channel:		the channel you want to receive the api requests on
func (r *ClientImpl) Subscribe(register *Subscription, channel RequestsChannel) (string, error) {
	if err := register.Valid(); err != nil {
		return "", err
	}
	r.requests = channel
	response := new(SubscriptionResponse)
	_, err := r.send("POST", API_SUBSCRIPTION, register, response)
	if err != nil {
		return "", err
	}
	return response.ID, nil
}

// Unregister from the bridge.io service
// 	id:				the registration id which was given when you registered
func (r *ClientImpl) Unsubscribe(id string) error {
	uri := fmt.Sprintf("%s/%s", API_SUBSCRIPTION, id)
	code, err := r.send("DELETE", uri, nil, nil)
	if err != nil {
		return err
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

	client.server_url, err = url.Parse(client.config.Binding)
	if err != nil {
		return errors.New(fmt.Sprintf("invalid http binding, error: %s", err))
	}

	// step: setup the routing
	client.router = mux.NewRouter()
	client.router.HandleFunc(client.server_url.RequestURI(), client.handleRequest).Methods("POST")

	// step: create the http server
	client.server = &http.Server{
		Addr:           client.config.Binding,
		Handler:        client.router,
		MaxHeaderBytes: 1 << 20,
	}
	client.server.SetKeepAlivesEnabled(true)

	// step: create a listener for the interface
	client.listener, err = net.Listen(client.server_url.Scheme, client.server_url.Host)
	if err != nil {
		return errors.New(fmt.Sprintf("failed to create the listener, error: %s", err))
	}
	// step: start listening
	go func() {
		client.server.ListenAndServe()
	}()

	return nil
}

func (r *ClientImpl) handleRequest(writer http.ResponseWriter, request *http.Request) {
	// step: we parse and decode the request and send on the channel
	req, err := r.decodeHttpRequest(request)
	if err != nil {
		return
	}
	// step: we create a channel for sending the response back to the client and pass
	// the reference in the api request struct. To ensure we don't end up with aplethoraa of
	// these, we have a fail-safe timer
	req.Response = make(RequestsChannel)
	// step: wait for a response from the consumer and reply back to the client
	go func(w http.ResponseWriter, rq *http.Request) {
		select {
		case response := <-req.Response:
			// step: we encode the response
			content, err := r.encodeRequest(response)
			if err != nil {
				return
			}

			w.WriteHeader(200)
			w.Write(content)

		case <-time.After(r.config.MaxTime):
			w.Write([]byte("timeout"))
		}
	}(writer, request)
}

// Send a json request to the bridge, get and decode the response
//	uri:		the uri on the bridge to target
//  result:		the data structure we should decode into
func (r *ClientImpl) send(method, uri string, data, result interface {}) (int, error) {
	// step: encode the post data
	var buffer bytes.Buffer
	if data != nil {
		err := json.NewEncoder(&buffer).Encode(data)
		if err != nil {
			return 0, fmt.Errorf("Failed to encode the data for request, error: %s", err)
		}
	}

	// step: we compose the request
	request, err := http.NewRequest(method, uri, &buffer)
	if err != nil {
		return 0, fmt.Errorf("Failed to compose the request to the bridge, error: %s", err)
	}

	// step: we perform the request
	response, err := r.client.Do(request)
	if err != nil {
		return response.StatusCode, fmt.Errorf("Failed to perform bridge request: %s, error: %s", uri, err)
	}

	// step: read in the response
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, fmt.Errorf("Failed to read in the response body, error: %s", err)
	}

	// step: decode the response
	err = json.NewDecoder(strings.NewReader(string(content))).Decode(result)
	if err != nil {
		return response.StatusCode, fmt.Errorf("Failed to decode the response, error: %s", err)
	}
	return response.StatusCode, nil
}

