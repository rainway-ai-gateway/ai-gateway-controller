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

package product_pool

type UpsertParam struct {
	Type      *string     `json:"type"`
	Name      *string     `json:"name" uri:"instance_pool_name" validate:"required,min=2"`
	Instances []*Instance `json:"instances" uri:"instances" validate:"min=1,dive"`

	//Domains   []*icluster_conf.Domain  `json:"domains"`
	//EPPServer *icluster_conf.EPPServer `json:"epp_server"`
	EPPServer *EPPServer `json:"epp_server"`
	Role      *string    `json:"role"`
}

type EPPServer struct {
	Domain    *string        `json:"domain"`
	Port      *int           `json:"port"`
	Endpoints []*EPPEndpoint `json:"endpoints"`
}

type EPPEndpoint struct {
	IP   *string `json:"ip"`
	Port *int    `json:"port"`
}
