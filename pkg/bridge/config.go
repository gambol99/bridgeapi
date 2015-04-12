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
	"io/ioutil"

	"github.com/gambol99/bridgeapi/pkg/bridge/client"
	"github.com/gambol99/bridgeapi/pkg/bridge/utils"

	"github.com/golang/glog"
)

func (r Config) String() string {
	return fmt.Sprintf(`
 Server API: %s
 Pipes: %s
 Verbosity: %d
`, r.Bind, r.Pipes, r.Verbosity)
}

// Returns a default configuration
func DefaultConfig() *Config {
	config := new(Config)
	config.Bind = DEFAULT_API_BINDING
	config.Pipes = []string{}
	config.Subscriptions = make([]*client.Subscription, 0)
	return config
}

// Load the configuration for the bridge from the config file
// 	filename:		the full path to the configuration file
func LoadConfig(filename string) (*Config, error) {
	if content, err := loadFile(filename); err != nil {
		return nil, err
	} else {
		return decodeConfig(content)
	}
}

func loadFile(filename string) (string, error) {
	glog.Infof("Loading the configuration file: %s", filename)
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(content), err
}

func decodeConfig(content string) (*Config, error) {
	config := new(Config)
	if err := utils.JsonDecode([]byte(content), config); err != nil {
		return nil, err
	}
	return config, nil
}
