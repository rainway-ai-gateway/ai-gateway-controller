package filtertest

import (
	"testing"

	"github.com/yf-networks/ai-gateway-controller/internal/controllers/filter"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/test/integration/testutil"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

const ns = "yxy-it-filter"

func newRawSvc(labels map[string]string) *corev1.Service {
	svc := &corev1.Service{}
	svc.Name = "filter-svc"
	svc.Namespace = ns
	svc.UID = types.UID("aabbccdd-0000-4000-8000-000000000050")
	svc.Labels = labels
	return svc
}

func apiNameOK(t *testing.T, svc *corev1.Service) bool {
	t.Helper()
	return filter.LabelFilter().Create(event.CreateEvent{Object: svc})
}

func withOpts(t *testing.T) {
	t.Helper()
	testutil.SetOpts(t, testutil.DefaultOpts())
}

// TC-01: ai-gateway-api-name 匹配 -> true
func TestFilter_ApiNameMatch(t *testing.T) {
	withOpts(t)
	svc := newRawSvc(map[string]string{"ai-gateway-api-name": testutil.DefaultAPIName})
	if !apiNameOK(t, svc) {
		t.Fatalf("matching ai-gateway-api-name should pass filter")
	}
}

// TC-02: ai-gateway-api-name 缺失 -> false
func TestFilter_NoApiName(t *testing.T) {
	withOpts(t)
	svc := newRawSvc(map[string]string{"app": "x"})
	if apiNameOK(t, svc) {
		t.Fatalf("service without ai-gateway-api-name should be filtered out")
	}
}

// TC-03: ai-gateway-api-name 值不匹配 -> false
func TestFilter_ApiNameMismatch(t *testing.T) {
	withOpts(t)
	svc := newRawSvc(map[string]string{"ai-gateway-api-name": "other"})
	if apiNameOK(t, svc) {
		t.Fatalf("mismatched ai-gateway-api-name should be filtered out")
	}
}

// TC-04: 无任何 labels -> false
func TestFilter_NoLabels(t *testing.T) {
	withOpts(t)
	svc := newRawSvc(nil)
	if apiNameOK(t, svc) {
		t.Fatalf("service without labels should be filtered out")
	}
}

// TC-05: api-name 配置为 "*" 时任意带标签 Service -> true
func TestFilter_ApiNameWildcard(t *testing.T) {
	opts := testutil.DefaultOpts()
	opts.ApiName = "*"
	testutil.SetOpts(t, opts)

	svc := newRawSvc(map[string]string{"ai-gateway-api-name": "anything"})
	if !apiNameOK(t, svc) {
		t.Fatalf("api-name '*' should match any labeled service")
	}
}

// TC-06: NamespaceFilter 命中（namespace 在列表内）
func TestFilter_NamespaceIncluded(t *testing.T) {
	opts := &option.Options{NamespaceList: []string{"ns-a", ns}}
	testutil.SetOpts(t, opts)

	svc := newRawSvc(nil)
	if !filter.NamespaceFilter().Create(event.CreateEvent{Object: svc}) {
		t.Fatalf("namespace in list should match")
	}
}

// TC-07: NamespaceFilter 未命中（namespace 不在列表内）
func TestFilter_NamespaceExcluded(t *testing.T) {
	opts := &option.Options{NamespaceList: []string{"ns-a"}}
	testutil.SetOpts(t, opts)

	svc := newRawSvc(nil)
	if filter.NamespaceFilter().Create(event.CreateEvent{Object: svc}) {
		t.Fatalf("namespace outside list should be filtered out")
	}
}

// TC-08: NamespaceFilter 通配（*）
func TestFilter_NamespaceAll(t *testing.T) {
	opts := &option.Options{NamespaceList: []string{"*"}}
	testutil.SetOpts(t, opts)

	svc := newRawSvc(nil)
	if !filter.NamespaceFilter().Create(event.CreateEvent{Object: svc}) {
		t.Fatalf("namespace '*' should match all")
	}
}
