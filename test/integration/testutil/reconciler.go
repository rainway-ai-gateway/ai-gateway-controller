package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/controllers/loadbalancer"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/internal/option/externalLB"

	corev1 "k8s.io/api/core/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Env holds the in-process test environment for the Service reconciler.
type Env struct {
	Rec       *loadbalancer.ServiceReconciler
	Client    client.Client
	Scheme    *apiruntime.Scheme
	MockALB   *ALBMock
	Namespace string
	SvcName   string
	Cluster   string
	Opts      *option.Options
	origOpts  *option.Options
}

func buildScheme() *apiruntime.Scheme {
	scheme := apiruntime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	return scheme
}

func buildFakeClient(scheme *apiruntime.Scheme, initObjs ...client.Object) client.Client {
	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(initObjs) > 0 {
		builder = builder.WithObjects(initObjs...)
	}
	return builder.Build()
}

func newAlbProvider(t *testing.T, opts *option.Options, addr string) *openapi.AlbProvider {
	t.Helper()
	if opts.ExternalLB == nil {
		opts.ExternalLB = externalLB.NewOptions()
	}
	// The AlbProvider InnerAPI client is built from opts.ExternalLB.ApiServerAddr.
	opts.ExternalLB.ApiServerAddr = addr
	if opts.ExternalLB.Token == "" {
		opts.ExternalLB.Token = "Token testtoken"
	}
	if opts.ExternalLB.Timeout == 0 {
		opts.ExternalLB.Timeout = 3000
	}
	return openapi.NewAlbProvider(opts.ExternalLB)
}

// NewEnv builds an in-process Service reconciler test environment:
//   - a fake k8s client (mocking Service/Endpoints/ConfigMap data)
//   - a httptest ALB mock (mocking the ai-gateway-api dependency)
//   - option.Opts is installed from opts and restored on cleanup.
func NewEnv(t *testing.T, opts *option.Options, initObjs ...client.Object) *Env {
	t.Helper()
	origOpts := option.Opts

	env := &Env{
		MockALB:   NewALBMock(),
		Namespace: "yxy-it-test",
		SvcName:   "it-svc",
		Cluster:   "testk8s",
		Opts:      opts,
		origOpts:  origOpts,
	}
	env.Scheme = buildScheme()
	env.Client = buildFakeClient(env.Scheme, initObjs...)

	lb := newAlbProvider(t, opts, env.MockALB.URL())
	option.Opts = opts
	env.Rec = loadbalancer.NewTestReconciler(env.Client, env.Scheme, lb, newFakeRecorder())

	t.Cleanup(func() {
		option.Opts = origOpts
		env.MockALB.Close()
	})
	return env
}

// newFakeRecorder returns a FakeRecorder whose Events channel is continuously
// drained, so emitEvent never blocks even under many reconcile iterations.
func newFakeRecorder() *record.FakeRecorder {
	rec := record.NewFakeRecorder(100)
	go func() {
		for range rec.Events {
		}
	}()
	return rec
}

// NewDownALBEnv is like NewEnv but the ALB api is unreachable
// (used to drive fail-path assertions for the result ConfigMap).
func NewDownALBEnv(t *testing.T, opts *option.Options, initObjs ...client.Object) *Env {
	t.Helper()
	origOpts := option.Opts

	env := &Env{
		Namespace: "yxy-it-test",
		SvcName:   "it-svc",
		Cluster:   "testk8s",
		Opts:      opts,
		origOpts:  origOpts,
	}
	env.Scheme = buildScheme()
	env.Client = buildFakeClient(env.Scheme, initObjs...)

	const downAddr = "http://127.0.0.1:1"
	lb := newAlbProvider(t, opts, downAddr)
	option.Opts = opts
	env.Rec = loadbalancer.NewTestReconciler(env.Client, env.Scheme, lb, newFakeRecorder())

	t.Cleanup(func() {
		option.Opts = origOpts
	})
	return env
}

// ReconcileOnce invokes Service Reconcile exactly once.
func (e *Env) ReconcileOnce(t *testing.T) error {
	t.Helper()
	_, err := e.Rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: e.Namespace, Name: e.SvcName},
	})
	return err
}

// ReconcileUntil calls Service Reconcile repeatedly until cond returns true or
// the timeout expires (the first pass only adds the finalizer and returns).
func (e *Env) ReconcileUntil(t *testing.T, cond func() bool, timeout time.Duration) error {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, _ = e.Rec.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: e.Namespace, Name: e.SvcName},
		})
		if cond() {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("condition not met within %v", timeout)
}

// GetService fetches the Service from the fake client.
func (e *Env) GetService(t *testing.T) *corev1.Service {
	t.Helper()
	svc := &corev1.Service{}
	if err := e.Client.Get(context.Background(), client.ObjectKey{Namespace: e.Namespace, Name: e.SvcName}, svc); err != nil {
		t.Fatalf("get service: %v", err)
	}
	return svc
}

// GetConfigMap fetches a ConfigMap from the fake client, failing if absent.
func (e *Env) GetConfigMap(t *testing.T, name string) *corev1.ConfigMap {
	t.Helper()
	cm := &corev1.ConfigMap{}
	if err := e.Client.Get(context.Background(), client.ObjectKey{Namespace: e.Namespace, Name: name}, cm); err != nil {
		t.Fatalf("get configmap %s: %v", name, err)
	}
	return cm
}

// ConfigMapExists reports whether a ConfigMap exists (without failing the test).
func (e *Env) ConfigMapExists(name string) bool {
	cm := &corev1.ConfigMap{}
	err := e.Client.Get(context.Background(), client.ObjectKey{Namespace: e.Namespace, Name: name}, cm)
	return err == nil
}

// ServiceExists reports whether the Service exists.
func (e *Env) ServiceExists() bool {
	svc := &corev1.Service{}
	err := e.Client.Get(context.Background(), client.ObjectKey{Namespace: e.Namespace, Name: e.SvcName}, svc)
	return err == nil
}

// DeleteService deletes the Service. Because the controller adds a finalizer,
// the fake client only sets a deletion timestamp (mirroring real k8s); the
// object is removed once the reconciler drops the finalizer.
func (e *Env) DeleteService(t *testing.T) {
	t.Helper()
	svc := e.GetService(t)
	if err := e.Client.Delete(context.Background(), svc); err != nil {
		t.Fatalf("delete service: %v", err)
	}
}
