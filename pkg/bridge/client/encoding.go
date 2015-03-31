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
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

func (r *ClientImpl) isJsonContent(request *http.Request) (bool, error) {
	content_type := request.Header.Get("Content-Type")
	if content_type == "" {
		if request.Body == nil || request.ContentLength == 0 {
			return false, nil
		}
	}
	if content_type == "application/json" {
		return true, nil
	}
	return false, fmt.Errorf("Content-Type specified: %s must be 'application/json'", content_type)
}

func (r *ClientImpl) decodeHttpRequest(request *http.Request) (*APIRequest, error) {
	// step: ensure we are dealing with json
	if valid, err := r.isJsonContent(request); err != nil {
		return nil, err
	} else if !valid {
		return nil, fmt.Errorf("invalid request, the content should be json")
	}

	// step: we read in the content the http request
	content, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	return r.decodeRequest(string(content))
}

func (r *ClientImpl) decodeRequest(content string) (*APIRequest, error) {
	api_request := new(APIRequest)
	err := json.NewDecoder(strings.NewReader(content)).Decode(api_request)
	if err != nil {
		return nil, err
	}
	return api_request, nil
}

func (r *ClientImpl) encodeRequest(request *APIRequest) ([]byte, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(request)
	if err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}
