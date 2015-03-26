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

	"github.com/alecthomas/kingpin"
	log "github.com/Sirupsen/logrus"
)

var (
	config  = DefaultConfig()
	cfgfile = kingpin.Flag("config", "the full path to an optional config file").Short('c').String()
	pipes   = kingpin.Flag("pipe", "a pipe specification, detail the source and sink").Short('p').Strings()
	bind    = kingpin.Flag("bind", "the interface to bind the admin rest api").Short('b').Default("127.0.0.1:7979").String()
	verbose = kingpin.Flag("verbose", "the level of verbosity for logging").Short('v').Int()
)

func main() {
	var err error

	if config, err = parseConfig(); err != nil {
		log.Fatalf("failed to process the configuration, error: %s", err)
	}

	log.Infof("config: %s", config)


}

func parseConfig() (*Config, error) {
	var err error
	// step: parse the command line
	kingpin.Parse()
	// load the config file is required
	if *cfgfile != "" {
		if config, err = loadConfig(*cfgfile); err != nil {
			return nil, err
		}
	}
	if *pipes != nil && len(*pipes) > 0 {
		config.Pipes = *pipes
	}

	if *bind != "" {
		config.ApiBinding = *bind
	}
	return config, nil
}
