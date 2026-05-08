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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis"
	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis/product_pool"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"
)

const (
	version = "/open-api/v1"

	productPoolPath   = version + "/products/%s/instance-pools"
	aiproductPoolPath = version + "/products/%s/ai-pools"
)

type OpenApiClient struct {
	remote string
	token  string
	client *http.Client
}

func NewOpenApiClient(addr, token string, timeout int) *OpenApiClient {
	return &OpenApiClient{
		remote: addr,
		token:  token,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Millisecond,
		},
	}
}

func (c *OpenApiClient) CreateProductPool(product string, req *product_pool.UpsertParam) (*product_pool.OneRsp, int, error) {
	l7poolpath := productPoolPath
	if option.Opts.EnableAiPoolPath {
		l7poolpath = aiproductPoolPath
	}
	uri := c.genURI(l7poolpath, product, "")

	result, err := c.doReq(uri, http.MethodPost, req)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, %s", result.ErrNum, result.RetMsg)
	}

	rsp := &product_pool.OneRsp{}
	if err = json.Unmarshal(result.Data, rsp); err != nil {
		return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s", err)
	}

	return rsp, result.ErrNum, nil
}

func (c *OpenApiClient) ListProductPool(product string) (*[]string, int, error) {
	uri := c.genURI(productPoolPath, product, "")
	result, err := c.doReq(uri, http.MethodGet, nil)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	rsp := &[]string{}
	if result.Data == nil || len(result.Data) == 0 {
		return rsp, result.ErrNum, nil
	}

	if err = json.Unmarshal(result.Data, rsp); err != nil {
		return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, data:%s, err:%s ", result.Data, err)
	}

	return rsp, result.ErrNum, nil
}

func (c *OpenApiClient) GetProductPool(product string, name string) (*product_pool.OneRsp, int, error) {
	uri := c.genURI(productPoolPath, product, name)
	result, err := c.doReq(uri, http.MethodGet, nil)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	rsp := &product_pool.OneRsp{}
	if err = json.Unmarshal(result.Data, rsp); err != nil {
		return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s ", err)
	}

	return rsp, result.ErrNum, nil
}

func (c *OpenApiClient) UpdateProductPool(product string, req *product_pool.UpsertParam) (*product_pool.OneRsp, int, error) {
	uri := c.genURI(productPoolPath, product, *req.Name)
	result, err := c.doReq(uri, http.MethodPatch, req)
	if err != nil {
		return nil, -1, err
	}
	if result.ErrNum != http.StatusOK {
		return nil, result.ErrNum, fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}

	rsp := &product_pool.OneRsp{}
	if err = json.Unmarshal(result.Data, rsp); err != nil {
		return nil, result.ErrNum, fmt.Errorf("fail to unmarshal data from API, err:%s ", err)
	}

	return rsp, result.ErrNum, nil
}

func (c *OpenApiClient) DeleteProductPool(product string, name string) error {
	l7poolpath := productPoolPath
	if option.Opts.EnableAiPoolPath {
		l7poolpath = aiproductPoolPath
	}

	uri := c.genURI(l7poolpath, product, name)
	result, err := c.doReq(uri, http.MethodDelete, nil)

	if err != nil {
		if result.ErrNum == http.StatusUnprocessableEntity && strings.Contains(err.Error(), "Product Not Exist") {
			return nil
		}
		return err
	}

	//err == nil
	if result.ErrNum == http.StatusUnprocessableEntity && strings.Contains(result.RetMsg, "Product Not Exist") {
		return nil
	}

	if result.ErrNum != http.StatusOK && result.ErrNum != http.StatusNotFound {
		return fmt.Errorf("code:%d, error:%s", result.ErrNum, result.RetMsg)
	}
	return err
}

func (c *OpenApiClient) genURI(path, product, name string) string {
	uri := fmt.Sprintf(path, product)
	if name != "" {
		uri = uri + "/" + name
	}
	return uri
}

func (c *OpenApiClient) doReq(uri, method string, obj interface{}) (*apis.Result, error) {
	var apiAddr string
	var useIdx int
	var err error
	var result *apis.Result

	apiAddr = option.Opts.ExternalLB.ApiServerAddr

	isHttpDoFailed := false
	srv_url := apiAddr + uri
	result, isHttpDoFailed, err = c.doReqImpl(srv_url, method, obj)

	util.ApiLogger.Info("doReq", "url", srv_url, "method", method, "iserr", err != nil, "useIdx", useIdx, "isHttpDoFailed", isHttpDoFailed)

	return result, err
}

func (c *OpenApiClient) doReqImpl(url, method string, obj interface{}) (*apis.Result, bool, error) {
	var body io.Reader
	if obj != nil {
		jsonStr, err := json.Marshal(obj)
		if err != nil {
			return nil, false, err
		}
		body = bytes.NewBuffer(jsonStr)
	}

	req, err := http.NewRequest(method, url, body)
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

	result := &apis.Result{}

	resbody, err := io.ReadAll(resp.Body)
	if err != nil {
		return result, false, fmt.Errorf("fail to read response body. error:%s", err.Error())
	}

	resbodyString := string(resbody)
	err = json.Unmarshal(resbody, result)
	if err != nil {
		return result, false, fmt.Errorf("fail to unmarshal respone result error: %s, resbody:%s", err.Error(), resbodyString)
	}

	return result, false, nil
}
