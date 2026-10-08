package testutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// ALBEntry is a single recorded request to the mock ALB server.
type ALBEntry struct {
	Method string
	Path   string
	Body   string
}

// ALBMock is an in-process httptest mock of the ai-gateway-api InnerAPI. It
// mirrors the contract used by internal/alb/innerApiClient.go (k8s_pools):
//
//	PUT         /inner-api/v1/k8s_pools/{n}/instances
//	GET/DELETE  /inner-api/v1/k8s_pools/{n}
//	GET         /inner-api/v1/k8s_pools
//
// Response body follows internal/alb/apis.Result: {"ErrNum","ErrMsg","Data"}.
type ALBMock struct {
	srv *httptest.Server

	mu       sync.Mutex
	k8sPools map[string]json.RawMessage // k8s pool name -> instances array
	access   []ALBEntry
}

// NewALBMock starts the mock ALB server and returns it.
func NewALBMock() *ALBMock {
	m := &ALBMock{
		k8sPools: make(map[string]json.RawMessage),
	}
	m.srv = httptest.NewServer(http.HandlerFunc(m.handle))
	return m
}

// URL returns the base URL of the mock ALB server.
func (m *ALBMock) URL() string { return m.srv.URL }

// Close shuts down the mock server.
func (m *ALBMock) Close() { m.srv.Close() }

// Entries returns a copy of all recorded requests (oldest first).
func (m *ALBMock) Entries() []ALBEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ALBEntry, len(m.access))
	copy(out, m.access)
	return out
}

// HasPath reports whether any recorded request path contains substr.
func (m *ALBMock) HasPath(substr string) bool {
	for _, e := range m.Entries() {
		if strings.Contains(e.Path, substr) {
			return true
		}
	}
	return false
}

// GetK8sPool returns the raw instance-array body of a k8s pool, or "" if absent.
func (m *ALBMock) GetK8sPool(name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw, ok := m.k8sPools[name]
	if !ok {
		return ""
	}
	return string(raw)
}

// K8sPoolExists reports whether the named k8s pool exists.
func (m *ALBMock) K8sPoolExists(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.k8sPools[name]
	return ok
}

// K8sPools returns the names of all k8s pools (unordered).
func (m *ALBMock) K8sPools() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.k8sPools))
	for k := range m.k8sPools {
		out = append(out, k)
	}
	return out
}

// CreateK8sPoolRaw seeds a k8s pool directly into the mock store without an
// HTTP round-trip (used to simulate a pre-existing pool before a reconcile).
func (m *ALBMock) CreateK8sPoolRaw(name, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.k8sPools[name] = json.RawMessage(body)
	return nil
}

func (m *ALBMock) handle(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)

	m.mu.Lock()
	m.access = append(m.access, ALBEntry{Method: r.Method, Path: r.URL.Path, Body: body})
	m.mu.Unlock()

	path := r.URL.Path
	if strings.HasPrefix(path, "/inner-api/v1/k8s_pools") {
		m.handleK8sPool(w, r, splitK8sPoolRest(path), body)
		return
	}
	m.write(w, http.StatusNotFound, "not found", nil)
}

// splitK8sPoolRest extracts the part after /inner-api/v1/k8s_pools, e.g.
// "" / "{name}" / "{name}/instances".
func splitK8sPoolRest(path string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, "/inner-api/v1/k8s_pools"), "/")
}

func (m *ALBMock) handleK8sPool(w http.ResponseWriter, r *http.Request, name, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch {
	case strings.HasSuffix(name, "/instances") && r.Method == http.MethodPut:
		pool := strings.TrimSuffix(name, "/instances")
		if pool == "" {
			m.write(w, http.StatusBadRequest, "missing name", nil)
			return
		}
		m.k8sPools[pool] = json.RawMessage(body)
		m.write(w, http.StatusOK, "OK", k8sPoolEntry(pool, body))
	case name == "" && r.Method == http.MethodGet:
		list := make([]map[string]interface{}, 0, len(m.k8sPools))
		for k, raw := range m.k8sPools {
			list = append(list, map[string]interface{}{
				"name":           k,
				"instance_count": countK8sInstances(string(raw)),
				"last_sync_time": time.Now().Unix(),
			})
		}
		data, _ := json.Marshal(map[string]interface{}{"list": list})
		m.write(w, http.StatusOK, "OK", data)
	case name != "" && r.Method == http.MethodGet:
		raw, ok := m.k8sPools[name]
		if !ok {
			m.write(w, http.StatusNotFound, "k8s pool not found", nil)
			return
		}
		m.write(w, http.StatusOK, "OK", k8sPoolEntry(name, string(raw)))
	case name != "" && r.Method == http.MethodDelete:
		if _, ok := m.k8sPools[name]; !ok {
			m.write(w, http.StatusNotFound, "k8s pool not found", nil)
			return
		}
		delete(m.k8sPools, name)
		m.write(w, http.StatusOK, "OK", nil)
	default:
		m.write(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

// k8sPoolEntry builds the PoolEntry response payload (k8s-pools.md §3).
func k8sPoolEntry(name, body string) json.RawMessage {
	instances := make([]json.RawMessage, 0)
	_ = json.Unmarshal([]byte(body), &instances)
	entry := map[string]interface{}{
		"name":           name,
		"instances":      instances,
		"instance_count": len(instances),
		"last_sync_time": time.Now().Unix(),
	}
	b, _ := json.Marshal(entry)
	return b
}

func countK8sInstances(body string) int {
	instances := make([]json.RawMessage, 0)
	_ = json.Unmarshal([]byte(body), &instances)
	return len(instances)
}

func readBody(r *http.Request) string {
	if r.Body == nil {
		return ""
	}
	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 256)
	for {
		n, err := r.Body.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return string(buf)
}

func (m *ALBMock) write(w http.ResponseWriter, errNum int, errMsg string, data json.RawMessage) {
	resp := map[string]interface{}{
		"ErrNum": errNum,
		"ErrMsg": errMsg,
	}
	if data != nil {
		resp["Data"] = data
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errNum)
	_ = json.NewEncoder(w).Encode(resp)
}
