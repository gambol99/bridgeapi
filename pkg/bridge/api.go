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
	"net/http"
	"time"

	"github.com/gambol99/bridge.io/pkg/bridge/client"

	"github.com/justinas/alice"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

const (
	API_VERSION      = "/v1"
	API_PING         = API_VERSION + "/ping"
	API_SUBSCRIBE    = API_VERSION + "/subscribe"
	API_SUBSCRIPTION = API_VERSION + "/subscriptions"
)


// the implementation for the rest api
func NewBridgeAPI(cfg *Config, bridge Bridge) (*BridgeAPI, error) {
	api := new(BridgeAPI)
	api.bridge = bridge

	// step: create the middle handlers
	middleware := alice.New(api.recoveryHandler, api.loggingHandler)

	// step: we construct the webservice
	router := mux.NewRouter()
	router.Handle(API_PING, middleware.ThenFunc(api.pingHandler)).Methods("GET")
	router.Handle(API_SUBSCRIBE, middleware.ThenFunc(api.subscribeHandler)).Methods("POST")
	router.Handle(API_SUBSCRIPTION, middleware.ThenFunc(api.subscriptionsHandler)).Methods("GET")
	router.Handle(API_SUBSCRIPTION+"/{id}", middleware.ThenFunc(api.unsubscribeHandler)).Methods("DELETE")
	api.router = router

	// step: we start listening
	api.server = &http.Server{
		Addr:    cfg.ApiBinding,
		Handler: router,
	}

	// step: start listening to the requests
	go api.server.ListenAndServe()
	return api, nil
}

func (r *BridgeAPI) recoveryHandler(next http.Handler) http.Handler {
	fn := func(wr http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(wr, http.StatusText(500), 500)
			}
		}()
		next.ServeHTTP(wr, req)
	}
	return http.HandlerFunc(fn)
}

func (r *BridgeAPI) loggingHandler(next http.Handler) http.Handler {
	fn := func(wr http.ResponseWriter, req *http.Request) {
		start_time := time.Now()
		next.ServeHTTP(wr, req)
		end_time := time.Now()
		log.Infof("[%s] %q %v", req.Method, req.URL.String(), end_time.Sub(start_time))
		next.ServeHTTP(wr, req)
	}
	return http.HandlerFunc(fn)
}

func (r *BridgeAPI) pingHandler(writer http.ResponseWriter, request *http.Request) {
	msg := new(client.MessageResponse)
	msg.Message = "pong"
	r.send(writer, request, msg)
}

func (r *BridgeAPI) subscribeHandler(writer http.ResponseWriter, request *http.Request) {
	// step: we grab and decode
	subscription := new(client.Subscription)
	if _, err := decodeByContent(request, subscription); err != nil {
		r.errorMessage(writer, request, "unable to decode the request, error: %s", err)
		return
	}

	// step: parse the request to the bridge
	id, err := r.bridge.Add(subscription)
	if err != nil {
		r.errorMessage(writer, request, "unable to add subscription, error: %s", err)
		return
	}
	// step: compose the response
	r.send(writer, request, &client.SubscriptionResponse{ ID: id, })
}

func (r *BridgeAPI) subscriptionsHandler(writer http.ResponseWriter, request *http.Request) {
	response := new(client.SubscriptionsResponse)
	//response.Subscriptions = r.bridge.Subscriptions()
	r.send(writer, request, response)
}

func (r *BridgeAPI) unsubscribeHandler(writer http.ResponseWriter, request *http.Request) {
	id := mux.Vars(request)["id"]
	if id == "" {
		r.errorMessage(writer, request, "you have not specified the subscription id")
		return
	}

	// check the subscription id exists


	// send a request to remove from the bridge

}

func (r *BridgeAPI) errorMessage(writer http.ResponseWriter, request *http.Request, message string, args ...interface{}) error {
	msg := &client.MessageResponse{
		Message: fmt.Sprintf(message, args...),
	}
	return r.send(writer, request, msg)
}

// Send a response back the client
func (r *BridgeAPI) send(writer http.ResponseWriter, request *http.Request, data interface{}) error {
	// encode to the appropriate content type
	content, content_type, err := encodeByContent(request, data)
	if err != nil {
		return err
	}
	writer.Header().Set("Content-Type", content_type)
	writer.WriteHeader(200)
	writer.Write(content)
	return nil
}

