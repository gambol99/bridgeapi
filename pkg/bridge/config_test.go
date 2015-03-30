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
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	TEST_CONFIG = `
	{
		"api": "0.0.0.0:8989",
		"subscriptions": [
			{
	           "id": "ec2_rds",
	           "endpoint": "http://127.0.0.1:9900/requests",
	           "hooks": [
	           		{
						"enforcing": false,
						"type": "pre",
						"uri": "/containers/create"

	                }
	           ]
			}
		],
	 	"pipes": [
	 	 	"tcp://127.0.0.1:4000,unix://var/run/docker.sock",
	 	 	"tcp://127.0.0.1:4000,unix://var/run/docker.sock"
	 	]
	}
	`
)

func TestDecodeConfig(t *testing.T) {
	config, err := decodeConfig(TEST_CONFIG)
	assert.Nil(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, "0.0.0.0:8989", config.ApiBinding)
	assert.NotNil(t, config.Subscriptions)
	assert.NotNil(t, config.Pipes)
	assert.Equal(t, 2, len(config.Pipes), "we should have had 2 pipes")
	assert.Equal(t, 1, len(config.Subscriptions), "we didn't find / decode any subscriptions")
}
