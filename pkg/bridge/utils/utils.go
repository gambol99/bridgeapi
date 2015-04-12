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

package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

var (
	uuidNumbers = []rune("0123456789")
)

func init() {
	rand.Seed(time.Nanosecond.Nanoseconds())
}

func Dial(location *url.URL) string {
	if location.Scheme == "unix" {
		return fmt.Sprintf("/%s%s", location.Host, location.Path)
	}
	return location.Host
}

func IsJsonContent(r *http.Request) bool {
	if HttpContentType(r) == "application/json" {
		return true
	}
	return false
}

// Encode the data structure in a json string
//	data:		the structure you wish to marshal into json
func JsonEncode(data interface{}) ([]byte, error) {
	// step: do we have anything to encode?
	if data == nil {
		return []byte{}, nil
	}
	// step: encode the structure
	var buffer bytes.Buffer
	if err := json.NewEncoder(&buffer).Encode(data); err != nil {
		return []byte{}, fmt.Errorf("unable to encode the data, error: %s", err)
	}
	return buffer.Bytes(), nil
}

// Decode the content and place into the result structure
//	content:		a string which hopefully contains some json object
//	result:			a structure you want to unmarshall the data into
func JsonDecode(content []byte, result interface{}) error {
	err := json.NewDecoder(strings.NewReader(string(content))).Decode(result)
	if err != nil {
		return fmt.Errorf("unable to decode content, error: %s", err)
	}
	return nil
}

// Essentially a wrapper for the HttpJsonSend method
// 	method:			the http method e.g. get, post etc
//	location:		the full url for the endpoint
//	payload:		a pointer to a structure which needs to be sent with the request
// 	result:			the structure we should decode the output into
func HttpJsonRequest(method, location string, payload, result interface{}, timeout time.Duration) error {
	if status, err := HttpJsonSend(method, location, payload, result, timeout); err != nil {
		return err
	} else {
		switch status {
		case 500:
			return fmt.Errorf("server responsed with internal error")
		case 404:
			return fmt.Errorf("resource not found")
		case 302:
			return fmt.Errorf("resource relocated")
		default:
			return nil
		}
	}
}

// Performs a json request and decodes the result for us
// 	method:			the http method e.g. get, post etc
//	location:		the full url for the endpoint
//	payload:		a pointer to a structure which needs to be sent with the request
// 	result:			the structure we should decode the output into
func HttpJsonSend(method, location string, payload, result interface{}, timeout time.Duration) (int, error) {
	// step: we parse the url
	endpoint, err := url.Parse(location)
	if err != nil {
		return 0, fmt.Errorf("invalid location url: %s", location)
	}

	// step: encode the post data
	content, err := JsonEncode(payload)
	if err != nil {
		return -1, err
	}

	// step: we compose the request
	request, err := http.NewRequest(method, endpoint.RequestURI(), bytes.NewReader(content))
	if err != nil {
		return 0, fmt.Errorf("unable to compose the request, error: %s", err)
	}

	// step: we compose the dialing to the bridge endpoint
	dial, err := net.Dial(endpoint.Scheme, Dial(endpoint))
	if err != nil {
		return 0, err
	}

	// step: we create a connection
	cli := httputil.NewClientConn(dial, nil)
	defer cli.Close()

	chError := make(chan error)
	chComplete := make(chan *http.Response)

	// step: we send the request in the backend and wait for either a timeout
	// or a response
	sendJsonRequest(cli, request, chComplete, chError)
	select {

	case <-time.After(timeout):
		return 0, fmt.Errorf("timed out waiting for response")

	case err := <-chError:
		return 0, fmt.Errorf("unable to recieve response, error: %s", err)

	case response := <-chComplete:
		// step: check the content is json
		content_type := response.Header.Get("Content-Type")
		if content_type != "application/json" {
			return 0, fmt.Errorf("the response content was not json")
		}
		// step: read in the content
		content, err := ReadHttpContentBody(response.Body, response.ContentLength)
		if err != nil {
			return 0, err
		}
		// step: decode the response
		err = json.NewDecoder(strings.NewReader(string(content))).Decode(result)
		if err != nil {
			log.Debugf("Failed to decode json response: %s", string(content))
			return response.StatusCode, fmt.Errorf("Failed to decode the response, error: %s", err)
		}
		return response.StatusCode, nil
	}
}

func HttpJsonDecode(request io.ReadCloser, size int64, data interface{}) error {
	// read in the content from the request
	content, err := ReadHttpContentBody(request, size)
	if err != nil {
		return err
	}
	return json.NewDecoder(strings.NewReader(string(content))).Decode(data)
}

func HttpJsonEncode(body io.ReadCloser, size int64, data interface{}) ([]byte, error) {
	var err error
	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(data)
	return buffer.Bytes(), err
}

// Read in the contents of a request
//	body:		the io reader from the request / response
//	length:		the length of the content
func ReadHttpContentBody(body io.ReadCloser, length int64) ([]byte, error) {
	if length > 0 || length < 0 {
		if content, err := ioutil.ReadAll(body); err != nil {
			return []byte{}, err
		} else {
			return content, nil
		}
	}
	return []byte{}, nil
}

// Generate a random number string of x amount
//	min:		the length of the string you want generated
func RandomUUID(min int) string {
	id := make([]rune, min)
	for i := range id {
		id[i] = uuidNumbers[rand.Intn(len(uuidNumbers))]
	}
	return string(id)
}

// Retrieve the content-type from the header of the request
// 	request:		the http request
func HttpContentType(request *http.Request) string {
	return request.Header.Get("Content-Type")
}

// Retrieve the http accept type of defaults
//	request:		the http request pointer
func HttpAcceptType(request *http.Request) string {
	return request.Header.Get("Accept")
}

// Transfer bytes from the src to destination
//	src:		the source for the data
//  dest:		the destination to write the data
//	wg:			a pointer to a wait group to indicate when complete
func TransferBytes(src io.Reader, dest io.Writer, wg *sync.WaitGroup) (int64, error) {
	defer wg.Done()
	copied, err := io.Copy(dest, src)
	if err != nil {
		return copied, err
	}
	src.(net.Conn).Close()
	dest.(net.Conn).Close()
	return copied, nil
}

func sendJsonRequest(cli *httputil.ClientConn, req *http.Request, finished chan *http.Response, errored chan error) {
	go func() {
		res, err := cli.Do(req)
		if err != nil {
			errored <- fmt.Errorf("unable to perform request: %s, error: %s", req.RequestURI, err)
		}
		finished <- res
	}()
}

