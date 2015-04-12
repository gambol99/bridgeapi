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

	"github.com/gambol99/bridgeapi/pkg/bridge/client"
	"github.com/gambol99/bridgeapi/pkg/bridge/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

// Create a new API service
//	bridge:		the reference to the bridge itself
func NewAPI(bridge Bridge) (*API, error) {
	api := new(API)
	chain := alice.New(api.recoveryHandler, api.loggingHandler, api.authenticationHandler)
	router := mux.NewRouter()
	router.Handle(client.API_SUBSCRIPTION, chain.ThenFunc(api.subscribeHandler)).Methods("POST")
	router.Handle(client.API_SUBSCRIPTION, chain.ThenFunc(api.subscriptionsHandler)).Methods("GET")
	router.Handle(client.API_SUBSCRIPTION+"/{id}", chain.ThenFunc(api.unsubscribeHandler)).Methods("DELETE")
	api.router = router
	api.server = &http.Server{
		Addr:    bridge.Config().Bind,
		Handler: router,
	}
	go api.server.ListenAndServe()
	return api, nil
}

func (r *API) recoveryHandler(next http.Handler) http.Handler {
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

func (r *API) loggingHandler(next http.Handler) http.Handler {
	fn := func(wr http.ResponseWriter, req *http.Request) {
		start_time := time.Now()
		next.ServeHTTP(wr, req)
		end_time := time.Now()
		log.Infof("[%s] %q %v", req.Method, req.URL.String(), end_time.Sub(start_time))
		next.ServeHTTP(wr, req)
	}
	return http.HandlerFunc(fn)
}

// Not exactly the most sophisticated security, but it will do for now
func (r *API) authenticationHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		if r.bridge.Config().Token != "" {
			secretToken := request.Header.Get("X-Auth-ApiBridge")
			if secretToken != r.bridge.Config().Token {
				log.Infof("Remote client: %s failed authentication, invalid token", request.RemoteAddr)
				w.WriteHeader(401)
				return
			}
		}
		next.ServeHTTP(w, request)
	}
	return http.HandlerFunc(fn)
}

func (r *API) subscribeHandler(writer http.ResponseWriter, request *http.Request) {
	// step: we grab and decode
	subscription := new(client.Subscription)
	if err := utils.HttpJsonDecode(request.Body, request.ContentLength, subscription); err != nil {
		r.error(writer, "unable to decode the request, error: %s", err)
	}
	// step: pass the request to the bridge
	if subscriptionID, err := r.bridge.AddSubscription(subscription); err != nil {
		r.error(writer, "unable to add subscription, error: %s", err)
	} else {
		r.send(writer, &client.SubscriptionResponse{ID: subscriptionID})
	}
}

func (r *API) subscriptionsHandler(writer http.ResponseWriter, request *http.Request) {
	response := new(client.SubscriptionsResponse)
	response.Subscriptions = r.bridge.Subscriptions()
	r.send(writer, response)
}

func (r *API) unsubscribeHandler(writer http.ResponseWriter, request *http.Request) {
	subscriptionID := mux.Vars(request)["id"]
	if subscriptionID == "" {
		r.error(writer, "you have not specified the subscription id")
	} else {
		r.bridge.DeleteSubscription(subscriptionID)
	}
}

func (r *API) error(writer http.ResponseWriter, message string, args ...interface{}) error {
	msg := &client.MessageResponse{
		Message: fmt.Sprintf(message, args...),
	}
	return r.send(writer, msg)
}

func (r *API) send(writer http.ResponseWriter, data interface{}) error {
	if content, err := utils.JsonEncode(data); err != nil {
		return err
	} else {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(200)
		writer.Write(content)
		return nil
	}
}
