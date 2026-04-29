// Copyright (c) 2026 The BFE Authors.
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

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

type EndpointPool struct {
	Namespace   string //inference pool namespace
	Name        string //inference pool name
	Selector    map[string]string
	TargetPorts []int
}

func (epPool *EndpointPool) Equal(oldEndpointPool *EndpointPool) bool {
	if oldEndpointPool == nil {
		return false
	}

	endpointPool := epPool
	if endpointPool.Namespace != oldEndpointPool.Namespace {
		return false
	}
	if endpointPool.Name != oldEndpointPool.Name {
		return false
	}
	if !epPool.SelectorEqual(oldEndpointPool) {
		return false
	}

	added, reomved := epPool.CalcTargetPortsDiff(oldEndpointPool)
	if len(added) != 0 || len(reomved) != 0 {
		return false
	}

	return true
}

func (epPool *EndpointPool) SelectorEqual(oldEndpointPool *EndpointPool) bool {
	endpointPool := epPool
	return labels.Equals(oldEndpointPool.Selector, endpointPool.Selector)
}

func (epPool *EndpointPool) CalcTargetPortsDiff(oldEndpointPool *EndpointPool) (added []int, removed []int) {
	endpointPool := epPool
	setOld := make(map[int]bool, len(oldEndpointPool.TargetPorts))
	for _, v := range oldEndpointPool.TargetPorts {
		setOld[v] = true
	}
	setNew := make(map[int]bool, len(endpointPool.TargetPorts))
	for _, v := range endpointPool.TargetPorts {
		setNew[v] = true
	}

	for v := range setNew {
		if !setOld[v] {
			added = append(added, v)
		}
	}

	for v := range setOld {
		if !setNew[v] {
			removed = append(removed, v)
		}
	}
	return
}

// EndpointMetadata represents the relevant Kubernetes Pod state of an inference server.
type EndpointMetadata struct {
	NamespacedName types.NamespacedName
	PodName        string
	PodNs          string
	Address        string
	Port           int

	Labels map[string]string
}

// String returns a string representation of the endpoint.
func (e *EndpointMetadata) String() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%+v", *e)
}

// Clone returns a full copy of the object.
func (p *EndpointMetadata) Clone() *EndpointMetadata {
	if p == nil {
		return nil
	}

	clonedLabels := make(map[string]string, len(p.Labels))
	for key, value := range p.Labels {
		clonedLabels[key] = value
	}
	return &EndpointMetadata{
		NamespacedName: types.NamespacedName{
			Name:      p.NamespacedName.Name,
			Namespace: p.NamespacedName.Namespace,
		},
		PodName: p.PodName,
		PodNs:   p.PodNs,
		Address: p.Address,
		Port:    p.Port,
		Labels:  clonedLabels,
	}
}

// GetNamespacedName gets the namespace name of the Endpoint.
func (e *EndpointMetadata) GetNamespacedName() types.NamespacedName {
	return e.NamespacedName
}

// GetIPAddress returns the Endpoint's IP address.
func (e *EndpointMetadata) GetIPAddress() string {
	return e.Address
}

// GetPort returns the Endpoint's inference port.
func (e *EndpointMetadata) GetPort() string {
	return fmt.Sprintf("%d", e.Port)
}
