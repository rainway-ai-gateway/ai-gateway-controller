// Copyright (c) 2025 The BFE Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package externalLB

import "fmt"

const (
	timeout = 3000 // unit ms
)

// Options of external loadbalancer
type Options struct {
	ApiServerAddr string
	Timeout       int //unit ms
	Token         string
}

func NewOptions() *Options {
	return &Options{
		ApiServerAddr: "http://172.18.1.200:30001",
		Token:         "Token f740c3040fed4bd7e97c",
		Timeout:       timeout,
	}
}

func (opts *Options) Check() error {
	if opts.ApiServerAddr == "" {
		return fmt.Errorf("alb api server address is not not specified")
	}

	if opts.Token == "" {
		return fmt.Errorf("alb api server token is not not specified")
	}

	return nil
}
