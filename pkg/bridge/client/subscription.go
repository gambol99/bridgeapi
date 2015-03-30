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
	"fmt"
	"net/url"
	"strings"
)

func (r *Subscription) Add(h *APIHook) {
	if r.Requests == nil {
		r.Requests = make([]*APIHook, 0)
	}
	r.Requests = append(r.Requests, h)
}

func (r Subscription) Valid() error {
	if r.ID == "" {
		return fmt.Errorf("You have not specified a application ID in the subscription")
	}
	if r.Endpoint == "" {
		return fmt.Errorf("You have not specified an endpoint for the subscription")
	}
	if _, err := url.Parse(r.Endpoint); err != nil {
		return fmt.Errorf("The endpoint url is invalid, please check")
	}
	if len(r.Requests) <= 0 {
		return fmt.Errorf("You have not specified any hooks")
	}

	// validate each of the hooks
	for _, hook := range r.Requests {
		if err := hook.Valid(); err != nil {
			return err
		}
	}

	return nil
}

func (r *APIHook) Valid() error {
	if r.URI == "" {
		return fmt.Errorf("the uri for the hook is empty")
	}
	// convert to uppercase
	r.HookType = strings.ToUpper(r.HookType)
	if r.HookType != "PRE" || r.HookType != "POST" {
		return fmt.Errorf("the hook type: %s is invalid, must be pre or post", r.HookType)
	}
	return nil
}
