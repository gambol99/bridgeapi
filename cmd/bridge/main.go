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
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gambol99/bridgeapi/pkg/bridge"

	log "github.com/Sirupsen/logrus"
	"github.com/alecthomas/kingpin"
)

var (
	cmd_conf  = kingpin.Flag("config", "the full path to an optional config file").Short('c').String()
	cmd_pipes = kingpin.Flag("pipe", "a pipe specification, detail the source and sink (this can be used multiple times)").Short('p').Strings()
	cmd_bind  = kingpin.Flag("bind", "the interface to bind the admin rest api").Short('b').String()
)

type BridgeService struct {
	// references to the pipes
	pipes []bridge.Pipe
	// reference to the bridge
	bridge bridge.Bridge
}

func main() {
	log.SetLevel(log.DebugLevel)
	// lets get the configuration
	config, err := parseConfiguration()
	if err != nil {
		log.Fatalf("Invalid configuration, error: %s", err)
	}
	// lets create the bridge
	bg, err := createBridge(config)
	if err != nil {
		log.Fatalf("Failed to create the bridge, error: %s", err)
	}
	// lets wait for a signal term
	signalChannel := make(chan os.Signal)
	// step: register the signals
	signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	// step: wait on the signal
	<-signalChannel
	// close the bridge
	shutdownBridge(bg)
}

func createBridge(config *bridge.Config) (*BridgeService, error) {
	var err error
	bdg := new(BridgeService)

	log.Infof("Creating the bridge, config: %s", config)

	// lets create the bridge
	bdg.bridge, err = bridge.NewBridge(config)
	if err != nil {
		return nil, err
	}
	// lets create the pipes
	for _, cfg := range config.Pipes {
		source, sink, err := parsePipeConfig(cfg)
		if err != nil {
			return nil, err
		}
		pipe, err := bridge.NewPipe(source, sink, bdg.bridge)
		if err != nil {
			return nil, err
		}
		bdg.pipes = append(bdg.pipes, pipe)
	}
	return bdg, nil
}

func shutdownBridge(bdg *BridgeService) error {
	// shutdown the pipes
	for _, pipe := range bdg.pipes {
		pipe.Close()
	}
	// shutdown the bridge
	return bdg.bridge.Close()
}

// Parse the pipe config and hands back the source and sink urls
func parsePipeConfig(cfg string) (*url.URL, *url.URL, error) {
	items := strings.Split(cfg, ",")
	if len(items) != 2 {
		return nil, nil, fmt.Errorf("Invalid pipe config, should be SOURCE,SINK")
	}
	source, err := url.Parse(items[0])
	if err != nil {
		return nil, nil, fmt.Errorf("Invalid source resource should be proto://resource, error: %s", err)
	}

	sink, err := url.Parse(items[1])
	if err != nil {
		return nil, nil, fmt.Errorf("Invalid sink resource should be proto://resource, error: %s", err)
	}
	return source, sink, nil
}

// Parse and process the configuration
func parseConfiguration() (*bridge.Config, error) {
	var err error
	kingpin.Parse()
	config := bridge.DefaultConfig()
	if *cmd_conf != "" {
		if config, err = bridge.LoadConfig(*cmd_conf); err != nil {
			return nil, err
		}
	}
	if *cmd_bind != "" {
		config.Bind = *cmd_bind
	}
	for _, pipe := range *cmd_pipes {
		config.Pipes = append(config.Pipes, pipe)
	}
	return validateConfiguration(config)
}

// validates we have everything we need in the configuration
func validateConfiguration(config *bridge.Config) (*bridge.Config, error) {
	if config.Bind == "" {
		return nil, fmt.Errorf("You have not specified a binding for the api service")
	}
	// step: lets check we have something to run - at the very least we need a pipe
	if config.Pipes == nil || len(config.Pipes) <= 0 {
		return nil, fmt.Errorf("You have not specified any pipes")
	}
	return config, nil
}
