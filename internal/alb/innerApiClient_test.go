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

package openapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis/k8s_pool"
)

func newTestInnerClient(t *testing.T, handler http.HandlerFunc) (*InnerApiClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewInnerApiClient(srv.URL, "Token testtoken", 3000), srv
}

func TestInnerApiClient_ReplaceK8sPoolInstances(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	c, _ := newTestInnerClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)

		data, _ := json.Marshal(map[string]interface{}{
			"name":           "svc-a",
			"instances":      []interface{}{map[string]interface{}{"addr": "10.0.0.1", "port": 8000}},
			"instance_count": 1,
			"last_sync_time": 1716883200,
		})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ErrNum": 200, "ErrMsg": "success", "Data": json.RawMessage(data),
		})
	})

	entry, code, err := c.ReplaceK8sPoolInstances(context.Background(), "svc-a",
		[]*k8s_pool.Instance{{Addr: "10.0.0.1", Port: 8000}})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method: want PUT, got %s", gotMethod)
	}
	if gotPath != "/inner-api/v1/k8s_pools/svc-a/instances" {
		t.Fatalf("path: got %s", gotPath)
	}
	if gotAuth != "Token testtoken" {
		t.Fatalf("auth header: got %q", gotAuth)
	}
	// weight is omitted (API defaults to 100)
	if gotBody != `[{"addr":"10.0.0.1","port":8000}]` {
		t.Fatalf("body: got %s", gotBody)
	}
	if code != 200 || entry == nil || entry.Name != "svc-a" || entry.InstanceCount != 1 {
		t.Fatalf("entry: code=%d entry=%+v", code, entry)
	}
}

func TestInnerApiClient_ReplaceK8sPoolInstances_Empty(t *testing.T) {
	var gotBody string
	c, _ := newTestInnerClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ErrNum": 200, "ErrMsg": "success"})
	})

	if _, _, err := c.ReplaceK8sPoolInstances(context.Background(), "svc-a", nil); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotBody != "[]" {
		t.Fatalf("empty snapshot should be [], got %s", gotBody)
	}
}

func TestInnerApiClient_DeleteK8sPool(t *testing.T) {
	cases := []struct {
		status  int
		wantErr bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false}, // idempotent delete
		{http.StatusInternalServerError, true},
	}

	for _, tc := range cases {
		status := tc.status
		c, _ := newTestInnerClient(t, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method: want DELETE, got %s", r.Method)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ErrNum": status, "ErrMsg": "x"})
		})

		err := c.DeleteK8sPool(context.Background(), "svc-a")
		if tc.wantErr && err == nil {
			t.Fatalf("status %d: want error, got nil", status)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("status %d: want nil, got %v", status, err)
		}
	}
}

func TestInnerApiClient_ReplaceK8sPoolInstances_Unreachable(t *testing.T) {
	c := NewInnerApiClient("http://127.0.0.1:1", "Token testtoken", 500)
	if _, _, err := c.ReplaceK8sPoolInstances(context.Background(), "svc-a",
		[]*k8s_pool.Instance{{Addr: "10.0.0.1", Port: 8000}}); err == nil {
		t.Fatalf("unreachable server should return error")
	}
}
