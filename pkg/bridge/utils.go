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
    "encoding/xml"
    "io/ioutil"
    "net/http"
    "strings"
)

func decodeByContent(request *http.Request, data interface{}) (string, error) {
    // read in the content from the request
	content, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return "", err
	}

	// we get the content type and decode appropriately
	ct := getRequestContentType(request, "application/json")
    switch ct {
    case "application/xml":
        err = xml.NewDecoder(strings.NewReader(string(content))).Decode(data)
    default:
        err = json.NewDecoder(strings.NewReader(string(content))).Decode(data)
    }

	return ct, err
}

func encodeByContent(request *http.Request, data interface{}) ([]byte, string, error) {
	var err error
	var buffer bytes.Buffer
	ct := getRequestAcceptType(request, "application/json")
	switch ct {
	case "application/xml":
        err = xml.NewEncoder(&buffer).Encode(data)
	default:
        err = json.NewEncoder(&buffer).Encode(data)
	}
    return buffer.Bytes(), ct, err
}

// Retrieve the content type from the request, otherwise we default
func getRequestContentType(request *http.Request, default_type string) string {
    return getRequestHeader(request, "Content-Type", default_type)
}

// Retrieve the http accept type of defaults
func getRequestAcceptType(request *http.Request, default_type string) string {
    return getRequestHeader(request, "Accept", default_type)
}

func getRequestHeader(request *http.Request, header string, dft string) string {
    ct := request.Header.Get(header)
    if ct == "" {
        return dft
    }
    return ct
}

