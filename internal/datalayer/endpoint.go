// Copyright (c) 2025 The BFE Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package datalayer

import (
	"fmt"
	"sync/atomic"
)

// Endpoint represents an inference serving endpoint and its related attributes.
type Endpoint interface {
	fmt.Stringer
	GetMetadata() *EndpointMetadata
	UpdateMetadata(*EndpointMetadata)
}

// ModelServer is an implementation of the Endpoint interface.
type ModelServer struct {
	pod atomic.Pointer[EndpointMetadata]
	// metrics    atomic.Pointer[Metrics]
	// attributes *Attributes
}

// NewEndpoint returns a new ModelServer with the given EndpointMetadata and Metrics.
func NewEndpoint(meta *EndpointMetadata) *ModelServer {
	if meta == nil {
		meta = &EndpointMetadata{}
	}
	ep := &ModelServer{}
	ep.UpdateMetadata(meta)
	return ep
}

// String returns a representation of the ModelServer. For brevity, only names of
// extended attributes are returned and not their values.
func (srv *ModelServer) String() string {
	return fmt.Sprintf("ModelServer Metadata: %v", srv.GetMetadata())
}

func (srv *ModelServer) GetMetadata() *EndpointMetadata {
	return srv.pod.Load()
}

func (srv *ModelServer) UpdateMetadata(pod *EndpointMetadata) {
	srv.pod.Store(pod)
}
