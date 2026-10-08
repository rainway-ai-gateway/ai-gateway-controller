//go:build ignore

// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package k8s_pools exposes the InnerAPI endpoints of the K8s instance pool
// domain (k8s-pools.md): the discovery component's exclusive write channel for
// discovered instance snapshots, plus read-only status queries.
package k8s_pools

import (
	"net/http"

	"github.com/gorilla/mux"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ik8s_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iprovider"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// PoolEntry is the response payload of one pool (k8s-pools.md §3).
type PoolEntry struct {
	Name          string                       `json:"name"`
	Instances     []iprovider.ProviderInstance `json:"instances"`
	InstanceCount int                          `json:"instance_count"`
	LastSyncTime  int64                        `json:"last_sync_time"`
}

// PoolListEntry is the summary payload of one pool in the list response
// (k8s-pools.md §4).
type PoolListEntry struct {
	Name          string `json:"name"`
	InstanceCount int    `json:"instance_count"`
	LastSyncTime  int64  `json:"last_sync_time"`
}

func poolEntryOf(pool *ik8s_pool.K8sPool) *PoolEntry {
	return &PoolEntry{
		Name:          pool.Name,
		Instances:     pool.Instances,
		InstanceCount: len(pool.Instances),
		LastSyncTime:  pool.UpdateTime,
	}
}

// ReplaceInstancesRoute replaces a pool's instance list (idempotent upsert).
var ReplaceInstancesRoute = &xreq.Endpoint{
	Path:       "/k8s_pools/{name}/instances",
	Method:     http.MethodPut,
	Handler:    xreq.Convert(ReplaceInstancesAction),
	Authorizer: iauth.FA(iauth.FeatureK8sPool, iauth.ActionUpdate),
}

var _ xreq.Handler = ReplaceInstancesAction

// ReplaceInstancesAction handles PUT /k8s_pools/{name}/instances.
func ReplaceInstancesAction(req *http.Request) (interface{}, error) {
	name, err := poolNameFromPath(req)
	if err != nil {
		return nil, err
	}

	params := []ik8s_pool.InstanceParam{}
	// The body is a bare JSON array, so the go-playground struct validation in
	// BindJSON does not apply; JSONDeserializer parses it and ValidateInstances
	// performs the field-level checks.
	if err := xreq.JSONDeserializer(req, &params); err != nil {
		return nil, err
	}
	instances, err := ik8s_pool.ValidateInstances(params)
	if err != nil {
		return nil, err
	}

	pool, err := container.K8sPoolManager.ReplaceInstances(req.Context(), name, instances)
	if err != nil {
		return nil, err
	}
	return poolEntryOf(pool), nil
}

// GetPoolRoute queries one pool entry.
var GetPoolRoute = &xreq.Endpoint{
	Path:       "/k8s_pools/{name}",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(GetPoolAction),
	Authorizer: iauth.FA(iauth.FeatureK8sPool, iauth.ActionRead),
}

var _ xreq.Handler = GetPoolAction

// GetPoolAction handles GET /k8s_pools/{name}.
func GetPoolAction(req *http.Request) (interface{}, error) {
	name, err := poolNameFromPath(req)
	if err != nil {
		return nil, err
	}

	pool, err := container.K8sPoolManager.FetchPool(req.Context(), name)
	if err != nil {
		return nil, err
	}
	if pool == nil {
		return nil, xerror.WrapRecordNotExist("k8s pool")
	}
	return poolEntryOf(pool), nil
}

// ListPoolsRoute queries all pool entries.
var ListPoolsRoute = &xreq.Endpoint{
	Path:       "/k8s_pools",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(ListPoolsAction),
	Authorizer: iauth.FA(iauth.FeatureK8sPool, iauth.ActionRead),
}

var _ xreq.Handler = ListPoolsAction

// ListPoolsAction handles GET /k8s_pools.
func ListPoolsAction(req *http.Request) (interface{}, error) {
	pools, err := container.K8sPoolManager.FetchPoolList(req.Context())
	if err != nil {
		return nil, err
	}

	list := make([]*PoolListEntry, 0, len(pools))
	for _, pool := range pools {
		list = append(list, &PoolListEntry{
			Name:          pool.Name,
			InstanceCount: len(pool.Instances),
			LastSyncTime:  pool.UpdateTime,
		})
	}
	return map[string]interface{}{"list": list}, nil
}

// DeletePoolRoute deletes a pool (no reference protection).
var DeletePoolRoute = &xreq.Endpoint{
	Path:       "/k8s_pools/{name}",
	Method:     http.MethodDelete,
	Handler:    xreq.Convert(DeletePoolAction),
	Authorizer: iauth.FA(iauth.FeatureK8sPool, iauth.ActionDelete),
}

var _ xreq.Handler = DeletePoolAction

// DeletePoolAction handles DELETE /k8s_pools/{name}.
func DeletePoolAction(req *http.Request) (interface{}, error) {
	name, err := poolNameFromPath(req)
	if err != nil {
		return nil, err
	}

	if err := container.K8sPoolManager.DeletePool(req.Context(), name); err != nil {
		return nil, err
	}
	return nil, nil
}

// poolNameFromPath extracts and validates the {name} path variable.
func poolNameFromPath(req *http.Request) (string, error) {
	name := mux.Vars(req)["name"]
	if name == "" {
		return "", xerror.WrapParamErrorWithMsg("name is required")
	}
	if err := iprovider.K8sPoolName(name); err != nil {
		return "", err
	}
	return name, nil
}
