package testutil

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// AssertCMDataFieldEquals asserts cm.Data[key] == want.
func AssertCMDataFieldEquals(t *testing.T, cm *corev1.ConfigMap, key, want string) {
	t.Helper()
	got, ok := cm.Data[key]
	if !ok {
		t.Fatalf("configmap %s has no data key %q", cm.Name, key)
	}
	if got != want {
		t.Fatalf("configmap %s data.%s: want %q, got %q", cm.Name, key, want, got)
	}
}

// AssertLabelEquals asserts cm.Labels[label] == want.
func AssertLabelEquals(t *testing.T, cm *corev1.ConfigMap, label, want string) {
	t.Helper()
	got := cm.Labels[label]
	if got != want {
		t.Fatalf("configmap %s label %s: want %q, got %q (labels=%v)", cm.Name, label, want, got, cm.Labels)
	}
}

// AssertAnnotationEquals asserts svc.Annotations[key] == want.
func AssertAnnotationEquals(t *testing.T, svc *corev1.Service, key, want string) {
	t.Helper()
	got := svc.Annotations[key]
	if got != want {
		t.Fatalf("service %s annotation %s: want %q, got %q (annotations=%v)", svc.Name, key, want, got, svc.Annotations)
	}
}

// AssertAnnotationContains asserts svc.Annotations[key] contains substr.
func AssertAnnotationContains(t *testing.T, svc *corev1.Service, key, substr string) {
	t.Helper()
	got := svc.Annotations[key]
	if !strings.Contains(got, substr) {
		t.Fatalf("service %s annotation %s: %q does not contain %q", svc.Name, key, got, substr)
	}
}

// HasFinalizer reports whether svc carries the given finalizer.
func HasFinalizer(svc *corev1.Service, finalizer string) bool {
	for _, f := range svc.Finalizers {
		if f == finalizer {
			return true
		}
	}
	return false
}

// instance mirrors the JSON body shape of product_pool.Instance for assertions.
type instance struct {
	Hostname string         `json:"hostname"`
	IP       string         `json:"ip"`
	Weight   int64          `json:"weight"`
	Ports    map[string]int `json:"ports"`
}

type eppServer struct {
	Domain *string `json:"domain"`
	Port   *int    `json:"port"`
}

// upsertBody mirrors the JSON shape the controller POSTs for a product pool.
type upsertBody struct {
	Type      string     `json:"type"`
	Role      string     `json:"role"`
	Name      string     `json:"name"`
	Instances []instance `json:"instances"`
	EPPServer eppServer  `json:"epp_server"`
}

// ParseUpsertBody parses a stored product-pool body.
func ParseUpsertBody(t *testing.T, body string) upsertBody {
	t.Helper()
	var up upsertBody
	if err := json.Unmarshal([]byte(body), &up); err != nil {
		t.Fatalf("parse upsert body %q: %v", body, err)
	}
	return up
}
