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
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/justinas/alice"
	"github.com/gorilla/context"
	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

func NewPipe(source, sink *url.URL, bridge Bridge) (Pipe, error) {
	var err error

	pipe := new(PipeImpl)
	pipe.source = source
	pipe.bridge = bridge
	pipe.sink = sink
	pipe.client = &http.Client{}

	source_socket := ""
	switch source.Scheme {
	case "unix":
		source_socket = fmt.Sprintf("/%s%s", source.Host, source.Path)
	case "tcp":
		source_socket = fmt.Sprintf("%s%s", source.Host, source.RequestURI())
	}

	log.Infof("Creating the source socket: %s:%s", source.Scheme, source_socket)

	// create the listener for the http service
	if pipe.listener, err = net.Listen(source.Scheme, source_socket); err != nil {
		log.Errorf("Failed to create the socket, error: %s", err)
		return nil, err
	}

	// create the handler chain
	middleware := alice.New(pipe.recoveryHandler, pipe.loggingHandler,
		pipe.preSinkRequestHandler, pipe.postSinkRequestHandler).ThenFunc(pipe.finalHandler)

	// create the router and apply the chain
	router := mux.NewRouter()
	router.Handle("/", middleware)
	router.PathPrefix("/").Handler(middleware)
	pipe.router = router

	// create the http server
	pipe.server = &http.Server{
		Handler:        router,
		ReadTimeout:    time.Duration(30) * time.Second,
		WriteTimeout:   time.Duration(30) * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	pipe.server.SetKeepAlivesEnabled(true)

	go pipe.server.Serve(pipe.listener)
	return pipe, nil
}

func (pipe *PipeImpl) Close() error {
	// remove any
	if pipe.source.Scheme == "unix" {
		if err := os.Remove(strings.Join([]string{"/", pipe.source.Host, pipe.source.Path}, "")); err != nil {
			log.Errorf("Failed to remove the unix socket: %s, error: %s", pipe.source.String(), err)
		}
	}
	if pipe.sink.Scheme == "unix" {
		if err := os.Remove(strings.Join([]string{"/", pipe.sink.Host, pipe.sink.Path}, "")); err != nil {
			log.Errorf("Failed to remove the unix socket: %s, error: %s", pipe.sink.String(), err)
		}
	}
	return nil
}

func (pipe *PipeImpl) recoveryHandler(next http.Handler) http.Handler {
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

func (pipe *PipeImpl) loggingHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		log.Infof("Recieved request %s:%q", request.Method, request.URL.String())
		start_time := time.Now()
		next.ServeHTTP(w, request)
		end_time := time.Now()
		log.Infof("Processed request %s:%q in %v", request.Method, request.URL.String(), end_time.Sub(start_time))
	}
	return http.HandlerFunc(fn)
}

func (pipe *PipeImpl) preSinkRequestHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		// generate the url and read in the original content
		content, err := pipe.extractRequestBody(request)
		if err != nil {
			log.Panicf("failed to read in the request body from the original request, error: %s", err)
		}

		// BRIDGE PRE HOOK CALL
		if content, err = pipe.bridge.PreHookEvent(request.URL.RequestURI(), content); err != nil {
			log.Errorf("Failed to call the bridge pre hook, error: %s", err)
		}

		// we inject the mutated response into the context
		context.Set(request, SESSION_REQUEST, content)

		// move on to the next level
		next.ServeHTTP(w, request)
	}
	return http.HandlerFunc(fn)
}

// The handler is responsible for processing the response from the sink and forwarding
// to anyone that is
func (pipe *PipeImpl) postSinkRequestHandler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, request *http.Request) {
		// we retrieve the request from the context
		mutation := (context.Get(request, SESSION_REQUEST)).([]byte)

		// are we hijacking the connection?
		hijacking := false
		if request.Header.Get("Upgrade") == "tcp" {
			hijacking = true
		}

		if !hijacking {

			// the forwarded url
			forward_url := pipe.parseForwardingURL(request)
			log.Debugf("Forwarded url: %s, content: %s", forward_url, mutation)

			// construct the request to be forwarded on to the sink
			forwarded, err := http.NewRequest(request.Method, forward_url, bytes.NewReader(mutation))
			if err != nil {
				log.Panicf("failed to construct the forwarding request to the sink, error: %s", err)
			}

			// add the headers to the forwarded request
			headers := []string{"User-Agent", "Content-Type", "Accept", "Host", "Upgrade"}
			for _, header := range headers {
				if request.Header.Get(header) != "" {
					forwarded.Header.Add(header, request.Header.Get(header))
				}
			}

			// we send the forwarded request onto the sink and read in the response
			response, err := pipe.client.Do(forwarded)
			if err != nil {
				log.Panicf("failed to forward the request on the sink, error: %s", err)
			}

			content, err := pipe.extractResponseBody(response)
			if err != nil {
				log.Panicf("failed to read in the content from the sink response, error: %s", err)
			}

			// BRIDGE POST HOOK CALL
			if content, err = pipe.bridge.PostHookEvent(request.RequestURI, content); err != nil {
				log.Errorf("Failed to call the bridge post hook, error: %s", err)
			}

			if len(content) > 0 {
				forwarded.Header.Add("Content-Length", fmt.Sprintf("%d", len(content)))
			}

			// write the content back to the client
			for _, header := range headers {
				if response.Header.Get(header) != "" {
					w.Header().Add(header, response.Header.Get(header))
				}
			}
			w.WriteHeader(response.StatusCode)
			w.Write(content)

			// move to the next item in the middleware chain
			next.ServeHTTP(w, request)

		} else {

			if err := pipe.hijack(w, request); err != nil {
				log.Panicf("failed to hijack the connection, error: %s", err)
			}

		}
	}
	return http.HandlerFunc(fn)
}

func (pipe *PipeImpl) hijack(w http.ResponseWriter, request *http.Request) error {
	// firstly, we check we can hijack
	log.Infof("Hijacking the connection to sink")

	// we retrieve the request from the context
	mutation := (context.Get(request, SESSION_REQUEST)).([]byte)

	// we write the headers
	w.Write([]byte(fmt.Sprintf("Content-Type: application/vnd.docker.raw-stream\r\n\r\n")))
	w.WriteHeader(200)

	hijack, ok := w.(http.Hijacker)
	if !ok {
		return fmt.Errorf("the webserver does not support hijacking connections")
	}
	// we grab the underlining connection
	conn, _, err := hijack.Hijack()
	if err != nil {
		return err
	}

	// a dial connection and create a connection to the sink
	dial, err := net.Dial(pipe.sink.Scheme, pipe.sink.Host)
	if err != nil {
		return err
	}

	log.Debugf("Dialing to: %s:%s", pipe.sink.Scheme, pipe.sink.Host)
	log.Debugf("Content for connection: %s", string(mutation))

	req, err := http.NewRequest(request.Method, request.RequestURI, bytes.NewReader(mutation))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "plain/text")
	req.Header.Set("Upgrade", "tcp")

	// dial into the sink
	http_client := httputil.NewClientConn(dial, nil)
	defer http_client.Close()

	// post the request to docker api (sink)
	response, err := http_client.Do(req)
	if err != nil {
		return err
	}
	if response.StatusCode >= 500 {
		return err
	}

	sink_conn, _ := http_client.Hijack()
	defer sink_conn.Close()

	var wg sync.WaitGroup
	wg.Add(2)
	go pipe.tranferBytes(sink_conn, conn, &wg)
	go pipe.tranferBytes(conn, sink_conn, &wg)
	wg.Wait()

	return nil
}

// The is effectivily a noop, perhaps using it for an audit trail
func (pipe *PipeImpl) finalHandler(w http.ResponseWriter, r *http.Request) {

}

func (pipe *PipeImpl) extractRequestBody(request *http.Request) ([]byte, error) {
	if request.ContentLength > 0 || request.ContentLength < 0 {
		content, err := ioutil.ReadAll(request.Body)
		if err != nil {
			return []byte{}, err
		}
		return content, nil
	}
	return []byte{}, nil
}

func (pipe *PipeImpl) extractResponseBody(response *http.Response) ([]byte, error) {
	if response.ContentLength > 0 || response.ContentLength < 0 {
		content, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return []byte{}, err
		}
		return content, nil
	}
	return []byte{}, nil
}

func (pipe *PipeImpl) tranferBytes(src io.Reader, dest io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()
	copied, err := io.Copy(dest, src)
	if err != nil {
		log.Errorf("Failed to copy from hijacked connections, error: %s", err)
	}
	src.(net.Conn).Close()
	dest.(net.Conn).Close()
	log.Infof("Copied %d bytes on hijacked connections", copied)
}

func (pipe *PipeImpl) parseForwardingURL(request *http.Request) string {
	host := pipe.parseForwardingHost(request)
	uri := pipe.parseForwardingURI(request)
	return fmt.Sprintf("%s%s", host, uri)
}

func (pipe *PipeImpl) parseForwardingURI(request *http.Request) string {
	uri := request.URL.RequestURI()
	if request.URL.Query() != nil && len(request.URL.Query()) > 0 {
		uri = fmt.Sprintf("%s?%s", uri, request.URL.RawQuery)
	}
	return uri
}

func (pipe *PipeImpl) parseForwardingHost(request *http.Request) string {
	host := ""
	switch pipe.sink.Scheme {
	case "tcp":
		host = fmt.Sprintf("http://%s", pipe.sink.Host)
	case "unix":
		host = fmt.Sprintf("unix://%s%s", pipe.sink.Host, pipe.sink.Path)
	}
	return host
}
