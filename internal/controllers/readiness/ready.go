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

package readiness

import (
	"fmt"
	"net/http"
)

type Event int

const (
	EventRunning Event = iota
)

var (
	readyStatus = map[Event]bool{
		EventRunning: false,
	}
)

func Checker(_ *http.Request) error {
	if !readyStatus[EventRunning] {
		return fmt.Errorf("Controller is not ready")
	}

	return nil
}

func SetReady(event Event) {
	if !readyStatus[event] {
		readyStatus[event] = true
	}
}

func SetUnready(event Event) {
	readyStatus[event] = false
}
