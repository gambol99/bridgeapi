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

package main

import (
	"flag"
	"os"

	"github.com/gambol99/bridge.io/pkg/bridge/client"

	log "github.com/Sirupsen/logrus"
)

var options struct {
	// the endpoint for the bridge
	bridge string
	// the interface we should listen on
	subscriber string
	}

func init() {
 	flag.StringVar(&options.bridge, "bridge", "unix://var/run/docker.sock", "the endpoint of the bridge")
	flag.StringVar(&options.subscriber, "subscriber", "tcp://127.0.0.1:8989", "the interface we should be listening for events on")
}

func main() {
	flag.Parse()
	config := client.DefaultConfig()
	config.Binding = options.subscriber
	config.Bridge = options.bridge
	config.Logger = os.Stdout
	log.SetLevel(log.DebugLevel)

	c, err := client.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create the bridge client, error: %s", err)
	}

	subscription := new(client.Subscription)
	subscription.ID = "slow_client"
	subscription.Subscriber = options.subscriber
	subscription.PreHook("/.*/containers/create")

	requests := make(client.RequestsChannel, 10)

	id, err := c.Subscribe(subscription, requests)
	if err != nil {
		log.Fatalf("Failed to subscribe to the bridge, error: %s", err)
	}
	log.Infof("Registered subscription to the bridge, id: %s", id)

	for request := range requests {
		log.Infof("Recieved a forwarded request for uei: %s, payload: %s", request.URI, request.Request)
		request.Response <- request
	}
}
