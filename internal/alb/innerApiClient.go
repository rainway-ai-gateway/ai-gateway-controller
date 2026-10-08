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

package openapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis"
	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis/k8s_pool"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"
)

const (
	innerAPIVersion      = "/inner-api/v1"
	k8sPoolPath          = innerAPIVersion + "/k8s_pools/%s"
	k8sPoolInstancesPath = k8sPoolPath + "/instances"
)

// InnerApiClient talks to the ai-gateway-api InnerAPI under /inner-api/v1. The
// k8s_pools domain is the discovery component's exclusive write channel for
// instance snapshots (k8s-pools.md).
type InnerApiClient struct {
	remote string
	token  string
	client *http.Client
}

// NewInnerApiClient creates an InnerApiClient for the given server address,
// token and timeout (milliseconds).
func NewInnerApiClient(addr, token string, timeout int) *InnerApiClient {
	return &InnerApiClient{
		remote: strings.TrimRight(addr, "/"),
		token:  token,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		},
	}
}

// ReplaceK8sPoolInstances performs PUT /k8s_pools/{name}/instances: it replaces
// the pool's instance list with the given full snapshot (idempotent upsert;
// an empty, non-nil slice means "zero instances").
func (c *InnerApiClient) ReplaceK8sPoolInstances(ctx context.Context, name string,
	instances []*k8s_pool.Instance) (*k8s_pool.PoolEntry, int, error) {

	if instances == nil {
		instances = []*k8s_pool.Instance{}
	}

	uri := fmt.Sprintf(k8sPoolInstancesPath, name)
	result, err := c.doReq(ctx, uri, http.MethodPut, instances)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	rsp := &k8s_pool.PoolEntry{}
	if len(result.Data) > 0 {
		if err := json.Unmarshal(result.Data, rsp); err != nil {
			return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s", err)
		}
	}
	return rsp, result.ErrNum, nil
}

// DeleteK8sPool performs DELETE /k8s_pools/{name}. A missing pool (404) is
// treated as success so deletion is idempotent.
func (c *InnerApiClient) DeleteK8sPool(ctx context.Context, name string) error {
	uri := fmt.Sprintf(k8sPoolPath, name)
	result, err := c.doReq(ctx, uri, http.MethodDelete, nil)
	if err != nil {
		if result != nil && result.ErrNum == http.StatusNotFound {
			return nil
		}
		return err
	}

	if result.ErrNum == http.StatusOK || result.ErrNum == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
}

// GetK8sPool performs GET /k8s_pools/{name} (read/diagnostics).
func (c *InnerApiClient) GetK8sPool(ctx context.Context, name string) (*k8s_pool.PoolEntry, int, error) {
	uri := fmt.Sprintf(k8sPoolPath, name)
	result, err := c.doReq(ctx, uri, http.MethodGet, nil)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	rsp := &k8s_pool.PoolEntry{}
	if len(result.Data) > 0 {
		if err := json.Unmarshal(result.Data, rsp); err != nil {
			return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s", err)
		}
	}
	return rsp, result.ErrNum, nil
}

// ListK8sPools performs GET /k8s_pools (read/diagnostics).
func (c *InnerApiClient) ListK8sPools(ctx context.Context) ([]*k8s_pool.PoolListEntry, int, error) {
	result, err := c.doReq(ctx, innerAPIVersion+"/k8s_pools", http.MethodGet, nil)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	data := struct {
		List []*k8s_pool.PoolListEntry `json:"list"`
	}{}
	if len(result.Data) > 0 {
		if err := json.Unmarshal(result.Data, &data); err != nil {
			return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s", err)
		}
	}
	return data.List, result.ErrNum, nil
}

func (c *InnerApiClient) doReq(ctx context.Context, uri, method string, obj interface{}) (*apis.Result, error) {
	srvURL := c.remote + uri
	result, isHTTPDoFailed, err := c.doReqImpl(ctx, srvURL, method, obj)

	util.ApiLogger.Info("doReq", "url", srvURL, "method", method, "iserr", err != nil, "isHttpDoFailed", isHTTPDoFailed)
	return result, err
}

func (c *InnerApiClient) doReqImpl(ctx context.Context, url, method string, obj interface{}) (*apis.Result, bool, error) {
	var body io.Reader
	if obj != nil {
		jsonStr, err := json.Marshal(obj)
		if err != nil {
			return nil, false, err
		}
		body = bytes.NewBuffer(jsonStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, false, err
	}
	req.Header.Add("Authorization", c.token)
	if obj != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer resp.Body.Close()

	result := &apis.Result{}
	resbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, false, fmt.Errorf("fail to read response body. error:%s", err.Error())
	}

	if err := json.Unmarshal(resbody, result); err != nil {
		return result, false, fmt.Errorf("fail to unmarshal respone result error: %s, resbody:%s", err.Error(), string(resbody))
	}
	return result, false, nil
}

// K8sPoolClient abstracts the InnerApiClient k8s_pools methods so AlbProvider
// can depend on an interface and unit tests can inject a fake.
type K8sPoolClient interface {
	ReplaceK8sPoolInstances(ctx context.Context, name string, instances []*k8s_pool.Instance) (*k8s_pool.PoolEntry, int, error)
	DeleteK8sPool(ctx context.Context, name string) error
	GetK8sPool(ctx context.Context, name string) (*k8s_pool.PoolEntry, int, error)
	ListK8sPools(ctx context.Context) ([]*k8s_pool.PoolListEntry, int, error)
}

var _ K8sPoolClient = (*InnerApiClient)(nil)
