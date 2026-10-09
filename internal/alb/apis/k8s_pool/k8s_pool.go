// Copyright(c) 2026 Beijing Yingfei Networks Technology Co.Ltd.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http: //www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Copyright (c) 2026 The BFE Authors.
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

// Package k8s_pool mirrors the InnerAPI k8s_pools payloads (k8s-pools.md):
// the discovery component's full-replacement instance snapshot request and the
// pool entry/status responses.
package k8s_pool

// Instance is one element of the PUT /k8s_pools/{name}/instances body.
// Weight is omitted when zero so the API applies its default (100).
type Instance struct {
	Addr   string `json:"addr"`
	Port   int    `json:"port"`
	Weight int64  `json:"weight,omitempty"`
}

// PoolEntry is the response payload of one pool (k8s-pools.md §3).
type PoolEntry struct {
	Name          string      `json:"name"`
	Instances     []*Instance `json:"instances"`
	InstanceCount int         `json:"instance_count"`
	LastSyncTime  int64       `json:"last_sync_time"`
}

// PoolListEntry is the summary payload of one pool in the list response
// (k8s-pools.md §4).
type PoolListEntry struct {
	Name          string `json:"name"`
	InstanceCount int    `json:"instance_count"`
	LastSyncTime  int64  `json:"last_sync_time"`
}
