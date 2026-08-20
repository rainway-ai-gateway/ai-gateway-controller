package inferencepooldiscovery

import (
	"testing"

	"github.com/yf-networks/ai-gateway-controller/test/integration/testutil"
)

const (
	ns       = "yxy-it-infer"
	poolName = "vllm-sim-pool"
	product  = "AI_product"
	cluster  = "testk8s"
	eppName  = "vllm-sim-endpoint-picker"
)

func eppPool() string {
	return product + ".k8s_" + ns + "_" + poolName + "_inferp_" + cluster
}

func buildPool(t *testing.T, selector map[string]string, targetPorts []int32, eppPort int32) *testutil.InferencePoolEnv {
	t.Helper()
	ip := testutil.BuildInferencePool(poolName, ns, selector, targetPorts, eppName, eppPort)
	env := testutil.NewInferencePoolEnv(t, testutil.DefaultOpts(), ip)
	env.Namespace = ns
	env.Product = product
	env.Cluster = cluster
	return env
}

// TC-01: 命名规则 + 实例组装 + EPP Server 信息
func TestInferPool_NamingAndInstances(t *testing.T) {
	env := buildPool(t, map[string]string{"app": "vllm"}, []int32{8200}, 9002)
	// seed one ready pod matching the selector
	pod := testutil.BuildPod("pod-0", ns, map[string]string{"app": "vllm"}, "10.0.0.1", true)
	if err := env.Client.Create(t.Context(), pod); err != nil {
		t.Fatalf("seed pod: %v", err)
	}

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	want := eppPool()
	if !env.MockALB.ProductPoolExists(product, want) {
		t.Fatalf("epp pool %s not created, pools=%v", want, env.MockALB.ProductPools(product))
	}

	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, want))
	if up.Type != "IP" {
		t.Fatalf("type should be IP, got %q", up.Type)
	}
	if up.Role != "EPP" {
		t.Fatalf("role should be EPP, got %q", up.Role)
	}
	if len(up.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(up.Instances))
	}
	ins := up.Instances[0]
	if ins.IP != "10.0.0.1" {
		t.Fatalf("instance ip should be pod ip, got %q", ins.IP)
	}
	if ins.Ports["Default"] != 8200 {
		t.Fatalf("instance port should be targetPort 8200, got %v", ins.Ports)
	}
	if up.EPPServer.Domain == nil || *up.EPPServer.Domain != eppName+"."+ns+".svc" {
		t.Fatalf("epp_server.domain should be %s.%s.svc, got %v", eppName, ns, up.EPPServer.Domain)
	}
	if up.EPPServer.Port == nil || *up.EPPServer.Port != 9002 {
		t.Fatalf("epp_server.port should be 9002, got %v", up.EPPServer.Port)
	}
}

// TC-02: 多 Ready Pod -> 多实例
func TestInferPool_MultiPodInstances(t *testing.T) {
	env := buildPool(t, map[string]string{"app": "vllm"}, []int32{8200}, 9002)
	if err := env.Client.Create(t.Context(), testutil.BuildPod("pod-0", ns, map[string]string{"app": "vllm"}, "10.0.0.1", true)); err != nil {
		t.Fatalf("seed pod0: %v", err)
	}
	if err := env.Client.Create(t.Context(), testutil.BuildPod("pod-1", ns, map[string]string{"app": "vllm"}, "10.0.0.2", true)); err != nil {
		t.Fatalf("seed pod1: %v", err)
	}

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, eppPool()))
	if len(up.Instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(up.Instances))
	}
}

// TC-03: selector 过滤 —— 仅匹配的 Ready Pod 纳入实例
func TestInferPool_SelectorFiltersPods(t *testing.T) {
	env := buildPool(t, map[string]string{"app": "vllm"}, []int32{8200}, 9002)
	seeds := []struct {
		name   string
		labels map[string]string
		ip     string
		ready  bool
	}{
		{"match-ready", map[string]string{"app": "vllm"}, "10.0.0.1", true},
		{"match-notready", map[string]string{"app": "vllm"}, "10.0.0.2", false},
		{"nomatch", map[string]string{"app": "other"}, "10.0.0.3", true},
	}
	for _, s := range seeds {
		if err := env.Client.Create(t.Context(), testutil.BuildPod(s.name, ns, s.labels, s.ip, s.ready)); err != nil {
			t.Fatalf("seed pod %s: %v", s.name, err)
		}
	}

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, eppPool()))
	if len(up.Instances) != 1 {
		t.Fatalf("expected 1 instance (only match-ready), got %d", len(up.Instances))
	}
	if up.Instances[0].IP != "10.0.0.1" {
		t.Fatalf("expected ip 10.0.0.1, got %q", up.Instances[0].IP)
	}
}

// TC-04: 多 targetPort -> 每个 Pod 每个端口一个 endpoint
func TestInferPool_MultiTargetPorts(t *testing.T) {
	env := buildPool(t, map[string]string{"app": "vllm"}, []int32{8200, 8201}, 9002)
	if err := env.Client.Create(t.Context(), testutil.BuildPod("pod-0", ns, map[string]string{"app": "vllm"}, "10.0.0.1", true)); err != nil {
		t.Fatalf("seed pod: %v", err)
	}

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, eppPool()))
	if len(up.Instances) != 2 {
		t.Fatalf("expected 2 endpoints (1 pod x 2 ports), got %d", len(up.Instances))
	}
	ports := map[int]bool{}
	for _, ins := range up.Instances {
		ports[ins.Ports["Default"]] = true
	}
	if !ports[8200] || !ports[8201] {
		t.Fatalf("expected ports 8200 and 8201, got %v", ports)
	}
}

// TC-05: 删除路径 —— 删除 InferencePool 后 EPP 池被清理
func TestInferPool_DeletePath(t *testing.T) {
	env := buildPool(t, map[string]string{"app": "vllm"}, []int32{8200}, 9002)
	if err := env.Client.Create(t.Context(), testutil.BuildPod("pod-0", ns, map[string]string{"app": "vllm"}, "10.0.0.1", true)); err != nil {
		t.Fatalf("seed pod: %v", err)
	}

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	want := eppPool()
	if !env.MockALB.ProductPoolExists(product, want) {
		t.Fatalf("epp pool not created")
	}

	env.DeleteInferencePool(t, poolName)

	if err := env.ReconcileOnce(t, poolName); err != nil {
		t.Fatalf("delete reconcile: %v", err)
	}
	if env.MockALB.ProductPoolExists(product, want) {
		t.Fatalf("epp pool should be deleted, pools=%v", env.MockALB.ProductPools(product))
	}
}
