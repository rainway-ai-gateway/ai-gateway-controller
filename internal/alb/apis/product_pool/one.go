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

package product_pool

// OneReqParam Request Param
// AUTO GEN BY ctrl, MODIFY AS U NEED
type OneReqParam struct {
	InstancePoolName string `json:"instance_pool_name" uri:"instance_pool_name" validate:"required,min=2"`
}

// Instance Request Param
// AUTO GEN BY ctrl, MODIFY AS U NEED
type Instance struct {
	Hostname string            `json:"hostname" uri:"hostname" validate:"required,min=2"`
	IP       string            `json:"ip" uri:"ip" validate:"required,ip"`
	Weight   int64             `json:"weight" uri:"weight" validate:"min=0,max=100"`
	Ports    map[string]int    `json:"ports" uri:"ports" validate:"required,min=1"`
	Tags     map[string]string `json:"tags" uri:"tags" validate:""`
}

//todo

// OneRsp Request Param
// AUTO GEN BY ctrl, MODIFY AS U NEED
type OneRsp struct {
	Name string `json:"name" uri:"name" validate:""`
	// Type      string      `json:"type"`
	Instances []*Instance `json:"instances" uri:"instances" validate:""`
}
