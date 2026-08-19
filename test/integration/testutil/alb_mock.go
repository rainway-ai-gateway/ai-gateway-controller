package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// ALBEntry is a single recorded request to the mock ALB server.
type ALBEntry struct {
	Method string
	Path   string
	Body   string
}

// ALBMock is an in-process httptest mock of the ai-gateway-api open-api server.
// It mirrors the contract used by internal/alb/openApiClient.go:
//
//	GET/POST    /open-api/v1/products/{p}/instance-pools[/{n}]
//	GET/PATCH/DELETE /open-api/v1/products/{p}/instance-pools/{n}
//	GET/POST    /open-api/v1/products/{p}/ai-pools[/{n}]
//	GET/PATCH/DELETE /open-api/v1/products/{p}/ai-pools/{n}
//
// Response body follows internal/alb/apis.Result: {"ErrNum","ErrMsg","Data"}.
type ALBMock struct {
	srv *httptest.Server

	mu     sync.Mutex
	pools  map[string]map[string]json.RawMessage // product -> poolName -> body
	access []ALBEntry
}

// NewALBMock starts the mock ALB server and returns it.
func NewALBMock() *ALBMock {
	m := &ALBMock{
		pools: make(map[string]map[string]json.RawMessage),
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

// ProductPools returns names of instance/ai pools created under product.
func (m *ALBMock) ProductPools(product string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.pools[product]))
	for k := range m.pools[product] {
		out = append(out, k)
	}
	return out
}

// GetProductPool returns the raw stored body of a pool, or "" if absent.
func (m *ALBMock) GetProductPool(product, name string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pools[product] == nil {
		return ""
	}
	raw, ok := m.pools[product][name]
	if !ok {
		return ""
	}
	return string(raw)
}

// ProductPoolExists reports whether the named pool exists under product.
func (m *ALBMock) ProductPoolExists(product, name string) bool {
	return m.GetProductPool(product, name) != ""
}

// CreateProductPoolRaw seeds a pool directly into the mock store without an
// HTTP round-trip (used to simulate a pre-existing pool before a reconcile).
func (m *ALBMock) CreateProductPoolRaw(product, name, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pools[product] == nil {
		m.pools[product] = make(map[string]json.RawMessage)
	}
	m.pools[product][name] = json.RawMessage(body)
	return nil
}

func (m *ALBMock) handle(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)

	m.mu.Lock()
	m.access = append(m.access, ALBEntry{Method: r.Method, Path: r.URL.Path, Body: body})
	m.mu.Unlock()

	path := r.URL.Path
	switch {
	case strings.Contains(path, "/instance-pools"):
		product, name := splitProductRest(path, "/instance-pools")
		m.handleProductPool(w, r, product, name, body)
	case strings.Contains(path, "/ai-pools"):
		product, name := splitProductRest(path, "/ai-pools")
		m.handleProductPool(w, r, product, name, body)
	default:
		m.write(w, http.StatusNotFound, "not found", nil)
	}
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

func (m *ALBMock) handleProductPool(w http.ResponseWriter, r *http.Request, product, name, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ensure := func() {
		if m.pools[product] == nil {
			m.pools[product] = make(map[string]json.RawMessage)
		}
	}

	switch {
	case name == "" && r.Method == http.MethodGet:
		ensure()
		names := make([]string, 0, len(m.pools[product]))
		for k := range m.pools[product] {
			names = append(names, k)
		}
		data, _ := json.Marshal(names)
		m.write(w, http.StatusOK, "OK", data)
	case name == "" && r.Method == http.MethodPost:
		ensure()
		n := extractName(body)
		if n == "" {
			m.write(w, http.StatusBadRequest, "missing name", nil)
			return
		}
		m.pools[product][n] = json.RawMessage(body)
		m.write(w, http.StatusOK, "OK", json.RawMessage(body))
	case name != "" && r.Method == http.MethodGet:
		ensure()
		data, ok := m.pools[product][name]
		if !ok {
			m.write(w, http.StatusNotFound, "product pool not found", nil)
			return
		}
		m.write(w, http.StatusOK, "OK", data)
	case name != "" && r.Method == http.MethodPatch:
		ensure()
		if _, ok := m.pools[product][name]; !ok {
			m.write(w, http.StatusNotFound, "product pool not found", nil)
			return
		}
		m.pools[product][name] = json.RawMessage(body)
		m.write(w, http.StatusOK, "OK", json.RawMessage(body))
	case name != "" && r.Method == http.MethodDelete:
		ensure()
		delete(m.pools[product], name)
		m.write(w, http.StatusOK, "OK", nil)
	default:
		m.write(w, http.StatusMethodNotAllowed, "method not allowed", nil)
	}
}

// splitProductRest extracts the product name and pool name (or "" for a list
// request) from a path like:
//
//	/open-api/v1/products/{product}/{marker}[/{name}]
func splitProductRest(path, marker string) (product, name string) {
	idx := strings.Index(path, marker)
	if idx < 0 {
		return "", ""
	}
	prefix := strings.TrimPrefix(path[:idx], "/open-api/v1/products/")
	prefix = strings.TrimPrefix(prefix, "/")
	product = prefix
	name = strings.TrimPrefix(path[idx+len(marker):], "/")
	return product, name
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

// extractName pulls "name" from a JSON body.
func extractName(body string) string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		return ""
	}
	if v, ok := m["name"].(string); ok {
		return v
	}
	return ""
}

var _ = fmt.Sprintf
