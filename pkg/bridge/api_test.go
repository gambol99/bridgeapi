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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/gambol99/bridgeapi/pkg/bridge/client"

	"github.com/stretchr/testify/assert"
)

const (
	API_BINDING = "127.0.0.1:3001"
)

var (
	api *API
)

func testAPIPath(uri string) string {
	return fmt.Sprintf("http://%s/%s/%s", API_BINDING, client.API_VERSION, uri)
}

func getJSON(url string, result interface{}, t *testing.T) {
	response, err := http.Get(url)
	if err != nil {
		t.Fatal("Failed to perform the http get: %s, error: %s", url, err)
	}
	content, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatal("Failed to read the request body, error: %s", err)
	}

	err = json.NewDecoder(strings.NewReader(string(content))).Decode(result)
	if err != nil {
		t.Fatal("Failed to decode the response, error: %s", err)
	}
}

func TestNewAPI(t *testing.T) {
	config := DefaultConfig()
	config.Bind = "127.0.0.1:3001"
	bridge := createTestBridge(config)
	assert.NotNil(t, bridge)
}

func TestAPIRegistrations(t *testing.T) {
	response, err := http.Get(testAPIPath("subscriptions"))
	assert.Nil(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, 200, response.StatusCode)
	content, err := ioutil.ReadAll(response.Body)
	assert.Nil(t, err)
	assert.NotEmpty(t, content)
}

func TestAPISubscribe(t *testing.T) {
	s := new(client.Subscription)
	s.Subscriber = "127.0.0.1:8080"
	s.ID = "test"
	s.Requests = make([]*client.Hook, 0)
	hk := new(client.Hook)
	hk.Enforcing = false
	hk.HookType = "PRE"
	hk.URI = "*/containers/start"

}
