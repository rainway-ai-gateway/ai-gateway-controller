package servicediscovery

import (
	"context"
	"testing"
	"time"

	"github.com/yf-networks/ai-gateway-controller/test/integration/testutil"

	"k8s.io/apimachinery/pkg/types"
)

const (
	ns       = "yxy-it-service"
	svcName  = "it-svc"
	product  = "AI_product"
	cluster  = "testk8s"
	finalizer = "k8s.bfenetworks.com/delete-protection"
	annotKey  = "k8s.bfenetworks.com/productpool-result"
)

func poolName(portName string) string {
	return product + ".k8s_" + ns + "_" + svcName + "_" + portName + "_" + cluster
}

func serviceSpec(specs ...testutil.PortSpec) testutil.ServiceSpec {
	return testutil.ServiceSpec{
		Name:      svcName,
		Namespace: ns,
		UID:       types.UID("aabbccdd-0000-4000-8000-000000000030"),
		Labels:    map[string]string{"bfe-product": product},
		Ports:     specs,
		PodIPs:    []string{"10.244.0.99"},
	}
}

func setEnv(t *testing.T, spec testutil.ServiceSpec) *testutil.Env {
	t.Helper()
	env := testutil.NewEnv(t, testutil.DefaultOpts(),
		testutil.BuildService(spec), testutil.BuildEndpoints(spec))
	env.Namespace = ns
	env.SvcName = svcName
	env.Product = product
	env.Cluster = cluster
	return env
}

// TC-01: 命名规则 + 实例组装（hostname=ip, weight=1, ports.Default=endpointPort）
func TestService_NamingAndInstances(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := setEnv(t, spec)

	want := poolName("http")
	if err := env.ReconcileUntil(t, func() bool { return env.MockALB.ProductPoolExists(product, want) }, 5*time.Second); err != nil {
		t.Fatalf("reconcile until pool present: %v", err)
	}

	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, want))
	if len(up.Instances) != 1 {
		t.Fatalf("expected 1 instance, got %d (%s)", len(up.Instances), env.MockALB.GetProductPool(product, want))
	}
	ins := up.Instances[0]
	if ins.Hostname != "10.244.0.99" || ins.IP != "10.244.0.99" {
		t.Fatalf("hostname/ip should be pod ip, got %+v", ins)
	}
	if ins.Weight != 1 {
		t.Fatalf("weight should be 1, got %d", ins.Weight)
	}
	if ins.Ports["Default"] != 80 {
		t.Fatalf("ports.Default should be 80 (endpoint port), got %v", ins.Ports)
	}
}

// TC-02: 多端口 -> 每个命名端口一个池
func TestService_MultiPort(t *testing.T) {
	spec := serviceSpec(
		testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80},
		testutil.PortSpec{Name: "grpc", Port: 9090, TargetPort: 9090},
	)
	env := setEnv(t, spec)

	p1, p2 := poolName("http"), poolName("grpc")
	if err := env.ReconcileUntil(t, func() bool {
		return env.MockALB.ProductPoolExists(product, p1) && env.MockALB.ProductPoolExists(product, p2)
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until both pools present: %v", err)
	}
	if got := env.MockALB.ProductPools(product); len(got) != 2 {
		t.Fatalf("expected 2 pools, got %v", got)
	}
}

// TC-03: 端口未命名跳过
func TestService_UnnamedPortSkipped(t *testing.T) {
	spec := serviceSpec(
		testutil.PortSpec{Name: "", Port: 8080, TargetPort: 80},
		testutil.PortSpec{Name: "named", Port: 8081, TargetPort: 81},
	)
	env := setEnv(t, spec)

	if err := env.ReconcileUntil(t, func() bool {
		return env.MockALB.ProductPoolExists(product, poolName("named"))
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until named pool present: %v", err)
	}
	if got := env.MockALB.ProductPools(product); len(got) != 1 {
		t.Fatalf("unnamed port should be skipped, expected 1 pool, got %v", got)
	}
}

// TC-04: Endpoints 无实例 -> 不调用 ALB，但 annotation 记录池名
func TestService_EmptyEndpoints(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	spec.PodIPs = nil
	epObj := testutil.BuildEndpoints(spec)
	epObj.Subsets = nil

	env := testutil.NewEnv(t, testutil.DefaultOpts(), testutil.BuildService(spec), epObj)
	env.Namespace = ns
	env.SvcName = svcName
	env.Product = product

	want := poolName("http")
	if err := env.ReconcileUntil(t, func() bool {
		s := env.GetService(t)
		return s.Annotations != nil && s.Annotations[annotKey] != ""
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until annotation present: %v", err)
	}
	if env.MockALB.ProductPoolExists(product, want) {
		t.Fatalf("empty endpoints should NOT create a pool in ALB")
	}
	svc := env.GetService(t)
	testutil.AssertAnnotationContains(t, svc, annotKey, want)
}

// TC-05: 多 Pod -> 多实例
func TestService_MultiPodInstances(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	spec.PodIPs = []string{"10.244.0.1", "10.244.0.2"}
	env := setEnv(t, spec)

	want := poolName("http")
	if err := env.ReconcileUntil(t, func() bool { return env.MockALB.ProductPoolExists(product, want) }, 5*time.Second); err != nil {
		t.Fatalf("reconcile until pool present: %v", err)
	}
	up := testutil.ParseUpsertBody(t, env.MockALB.GetProductPool(product, want))
	if len(up.Instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(up.Instances))
	}
}

// TC-06: productpool-result 注解写回
func TestService_WritesResultAnnotation(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := setEnv(t, spec)

	if err := env.ReconcileUntil(t, func() bool {
		s := env.GetService(t)
		return s.Annotations != nil && s.Annotations[annotKey] != ""
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until annotation present: %v", err)
	}
	testutil.AssertAnnotationContains(t, env.GetService(t), annotKey, poolName("http"))
}

// TC-07: diff 收敛 —— 端口从 2 个减到 1 个，旧池被删除
func TestService_DiffConvergeRemovesPool(t *testing.T) {
	spec := serviceSpec(
		testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80},
		testutil.PortSpec{Name: "grpc", Port: 9090, TargetPort: 9090},
	)
	env := setEnv(t, spec)

	p1, p2 := poolName("http"), poolName("grpc")
	if err := env.ReconcileUntil(t, func() bool {
		return env.MockALB.ProductPoolExists(product, p1) && env.MockALB.ProductPoolExists(product, p2)
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until both pools present: %v", err)
	}

	// update svc: drop grpc port
	svcObj := env.GetService(t)
	newSvc := testutil.BuildService(testutil.ServiceSpec{
		Name: svcName, Namespace: ns, UID: svcObj.UID, Labels: svcObj.Labels,
		Ports: []testutil.PortSpec{{Name: "http", Port: 8080, TargetPort: 80}},
		PodIPs: []string{"10.244.0.99"},
	})
	newSvc.Annotations = svcObj.Annotations
	newSvc.Finalizers = svcObj.Finalizers
	newSvc.ResourceVersion = svcObj.ResourceVersion
	if err := env.Client.Update(context.Background(), newSvc); err != nil {
		t.Fatalf("update svc: %v", err)
	}

	if err := env.ReconcileUntil(t, func() bool {
		return env.MockALB.ProductPoolExists(product, p1) && !env.MockALB.ProductPoolExists(product, p2)
	}, 8*time.Second); err != nil {
		t.Fatalf("diff converge: grpc pool should be removed, got pools=%v", env.MockALB.ProductPools(product))
	}
}

// TC-08: 首次 reconcile 自动添加 finalizer
func TestService_AddsFinalizer(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := setEnv(t, spec)

	if err := env.ReconcileUntil(t, func() bool {
		return testutil.HasFinalizer(env.GetService(t), finalizer)
	}, 5*time.Second); err != nil {
		t.Fatalf("reconcile until finalizer present: %v", err)
	}
}

// TC-09: 删除路径 —— 删除 Service 后池与 result cm 被清理
func TestService_DeletePath(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := setEnv(t, spec)

	want := poolName("http")
	if err := env.ReconcileUntil(t, func() bool { return env.MockALB.ProductPoolExists(product, want) }, 5*time.Second); err != nil {
		t.Fatalf("reconcile until pool present: %v", err)
	}

	env.DeleteService(t)

	if err := env.ReconcileOnce(t); err != nil {
		t.Fatalf("delete reconcile: %v", err)
	}

	if env.MockALB.ProductPoolExists(product, want) {
		t.Fatalf("pool should be deleted after service delete")
	}
	if env.ServiceExists() {
		t.Fatalf("service should be removed after finalizer removal")
	}
	if env.ConfigMapExists(svcName + ".result") {
		t.Fatalf("result configmap should be cleaned up")
	}
}

// TC-10: 成功路径 result ConfigMap
func TestService_ResultCM(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := setEnv(t, spec)

	if err := env.ReconcileUntil(t, func() bool { return env.ConfigMapExists(svcName + ".result") }, 5*time.Second); err != nil {
		t.Fatalf("reconcile until result cm present: %v", err)
	}

	cm := env.GetConfigMap(t, svcName+".result")
	testutil.AssertCMDataFieldEquals(t, cm, "result", "Succ")
	testutil.AssertLabelEquals(t, cm, "bfe-cm-result", "yes")
	testutil.AssertLabelEquals(t, cm, "bfe-result-type", "service")
	if _, ok := cm.Data["timestamp"]; !ok || cm.Data["timestamp"] == "" {
		t.Fatalf("result cm should have timestamp, data=%v", cm.Data)
	}
}

// TC-11: 失败路径 result ConfigMap（ALB 不可达）
func TestService_ResultCMFailPath(t *testing.T) {
	spec := serviceSpec(testutil.PortSpec{Name: "http", Port: 8080, TargetPort: 80})
	env := testutil.NewDownALBEnv(t, testutil.DefaultOpts(),
		testutil.BuildService(spec), testutil.BuildEndpoints(spec))
	env.Namespace = ns
	env.SvcName = svcName
	env.Product = product

	if err := env.ReconcileUntil(t, func() bool { return env.ConfigMapExists(svcName + ".result") }, 5*time.Second); err != nil {
		t.Fatalf("reconcile until result cm present: %v", err)
	}

	cm := env.GetConfigMap(t, svcName+".result")
	if cm.Data["result"] == "Succ" {
		t.Fatalf("fail path should not write Succ, data=%v", cm.Data)
	}
	testutil.AssertLabelEquals(t, cm, "bfe-cm-result", "yes")
}
