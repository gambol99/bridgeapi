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
	"net/http"
	"strings"

	"github.com/gambol99/bridge.io/client"

	"github.com/gorilla/mux"
)

const (
	API_VERSION      = "/v1"
	API_PING         = API_VERSION + "/ping"
	API_SUBSCRIBE    = API_VERSION + "/subscribe"
	API_SUBSCRIPTION = API_VERSION + "/subscriptions"
)

type ResponseMessage struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// the implementation for the rest api
func NewBridgeAPI(cfg *Config, bridge Bridge) (*BridgeAPI, error) {
	api := new(BridgeAPI)
	api.bridge = bridge
	// step: we construct the webservice
	router := mux.NewRouter()
	router.HandleFunc(API_PING, api.handlePing)
	router.HandleFunc(API_SUBSCRIBE, api.handleSubscribe).Methods("POST")
	router.HandleFunc(API_SUBSCRIPTION, api.handleSubscriptions).Methods("GET")
	router.HandleFunc(API_SUBSCRIPTION+"/{id}", api.handleUnsubscribe).Methods("DELETE")
	api.router = router

	// step: we start listening
	api.server = &http.Server{
		Addr:    cfg.ApiBinding,
		Handler: api.router,
	}
	// step: start listening to the requests
	go api.server.ListenAndServe()
	return api, nil
}

func (r *BridgeAPI) handlePing(writer http.ResponseWriter, request *http.Request) {
	writer.Write([]byte("pong"))
}

func (r *BridgeAPI) handleSubscribe(writer http.ResponseWriter, request *http.Request) {
	subscription := new(client.Subscription)
	if _, err := r.decode(request, subscription); err != nil {
		http.Error(writer, "invalid request", 500)
		return
	}
}

func (r *BridgeAPI) handleSubscriptions(writer http.ResponseWriter, request *http.Request) {
	// step: get the list of subscriptions
	var response struct {
		Subscriptions []*client.Subscription `json:"subscriptions"`
	}
	response.Subscriptions = r.bridge.Subscriptions()
	content, err := r.encode(response)
	if err != nil {
		http.Error(writer, "unable to encode the subscriptions", 500)
		return
	}

	writer.Write([]byte(content))
}

func (r *BridgeAPI) handleUnsubscribe(writer http.ResponseWriter, request *http.Request) {
	id := mux.Vars(request)["id"]
	if id == "" {
		r.errorMessage(writer, request, "you have not specified the subscription id")
	}
	// check the subscription id exists

	// send a request to remove from the bridge

}

func (r *BridgeAPI) errorMessage(writer http.ResponseWriter, request *http.Request, message string, args ...interface{}) {
	msg := &ResponseMessage{
		Status:  "error",
		Message: fmt.Sprintf(message, args...),
	}
	r.sendResponse(writer, request, msg)
}

func (r *BridgeAPI) sendResponse(writer http.ResponseWriter, request *http.Request, data interface{}) {

}

func (r *BridgeAPI) decode(request *http.Request, data interface{}) (string, error) {
	content, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return "", err
	}
	err = json.NewDecoder(strings.NewReader(string(content))).Decode(data)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (r *BridgeAPI) encode(data interface{}) (string, error) {
	buffer := new(bytes.Buffer)
	if err := json.NewEncoder(buffer).Encode(data); err != nil {
		return "", nil
	}
	return buffer.String(), nil
}

func (r *BridgeAPI) apiPath(path string) string {
	return fmt.Sprintf("/%s/%s", API_VERSION, path)
}
