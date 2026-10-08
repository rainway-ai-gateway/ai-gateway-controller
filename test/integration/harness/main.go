// Command harness drives the ai-gateway-controller ServiceReconciler with a
// mocked Kubernetes (controller-runtime fake client) so cross-component
// integration tests can verify the controller's writes against a REAL
// ai-gateway-api process, without a real k8s cluster.
//
// It reads a JSON plan (set Service/Endpoints, run Reconcile N times, delete
// Service), executes it against the in-process reconcile code, and prints a
// JSON report of the observed mocked-k8s state. The caller asserts the
// ai-gateway-api side itself (via /inner-api/v1/k8s_pools).
//
// This binary lives in the controller module because it needs the internal
// reconciler packages; the integration-test repo only builds and runs it.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/controllers/loadbalancer"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/internal/option/externalLB"
	"github.com/yf-networks/ai-gateway-controller/test/integration/testutil"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Plan is the harness input.
type Plan struct {
	Namespace   string `json:"namespace"`
	Service     string `json:"service"`
	UID         string `json:"uid"`
	APIName     string `json:"api_name"`
	ClusterName string `json:"cluster_name"`
	APIServer   string `json:"api_server"`
	Token       string `json:"token"`
	Steps       []Step `json:"steps"`
}

// Step is one operation in the plan.
//
//	op=set              create/update the mocked Service + Endpoints
//	op=reconcile        invoke Reconcile "times" times
//	op=delete_service   delete the mocked Service (finalizer aware)
type Step struct {
	Op    string `json:"op"`
	Ports []Port `json:"ports,omitempty"`
	// Endpoints is not omitempty: an explicit empty list means "no Ready
	// addresses" (zero instances), distinct from an omitted (default) list.
	Endpoints []string `json:"endpoints"`
	Times     int      `json:"times,omitempty"`
}

// Port describes one named Service port and its endpoint target port.
type Port struct {
	Name       string `json:"name"`
	Port       int32  `json:"port"`
	TargetPort int32  `json:"target_port"`
}

// Report is the harness output.
type Report struct {
	ReconcileErrors []string                     `json:"reconcile_errors"`
	ServiceExists   bool                         `json:"service_exists"`
	Finalizers      []string                     `json:"finalizers"`
	Annotations     map[string]string            `json:"annotations"`
	ConfigMaps      map[string]map[string]string `json:"configmaps"`
}

func main() {
	planPath := flag.String("plan", "", "path to the JSON plan file (\"-\" for stdin)")
	flag.Parse()

	if *planPath == "" {
		fmt.Fprintln(os.Stderr, "missing -plan")
		os.Exit(2)
	}

	var raw []byte
	var err error
	if *planPath == "-" {
		raw, err = readAll(os.Stdin)
	} else {
		raw, err = os.ReadFile(*planPath)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "read plan: %v\n", err)
		os.Exit(2)
	}

	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		fmt.Fprintf(os.Stderr, "parse plan: %v\n", err)
		os.Exit(2)
	}

	report, err := run(plan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "run plan: %v\n", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))
}

func readAll(f *os.File) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := f.Read(tmp)
		buf = append(buf, tmp[:n]...)
		if err != nil {
			break
		}
	}
	return buf, nil
}

func run(plan Plan) (*Report, error) {
	ctx := context.Background()

	opts := option.NewOptions()
	opts.ClusterName = plan.ClusterName
	opts.ApiName = plan.APIName
	opts.NamespaceList = []string{"*"}
	opts.ExternalLB = externalLB.NewOptions()
	opts.ExternalLB.ApiServerAddr = plan.APIServer
	opts.ExternalLB.Token = plan.Token
	opts.ExternalLB.Timeout = 3000
	option.Opts = opts

	scheme := apiruntime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return nil, err
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	recorder := record.NewFakeRecorder(1024)
	go func() {
		for range recorder.Events {
		}
	}()

	lb := openapi.NewAlbProvider(opts.ExternalLB)
	rec := loadbalancer.NewTestReconciler(fakeClient, scheme, lb, recorder)

	key := client.ObjectKey{Namespace: plan.Namespace, Name: plan.Service}

	report := &Report{
		Annotations: map[string]string{},
		ConfigMaps:  map[string]map[string]string{},
	}

	for _, step := range plan.Steps {
		switch step.Op {
		case "set":
			spec := testutil.ServiceSpec{
				Name:      plan.Service,
				Namespace: plan.Namespace,
				UID:       types.UID(plan.UID),
				Labels:    map[string]string{"ai-gateway-api-name": plan.APIName},
				Ports:     toPortSpecs(step.Ports),
				PodIPs:    step.Endpoints,
			}
			if err := upsertService(ctx, fakeClient, key, testutil.BuildService(spec)); err != nil {
				return nil, fmt.Errorf("set service: %w", err)
			}
			endpoints := testutil.BuildEndpoints(spec)
			// An explicitly empty endpoints list means "no Ready addresses"
			// (zero instances); testutil.BuildEndpoints otherwise defaults to
			// a placeholder address.
			if step.Endpoints != nil && len(step.Endpoints) == 0 {
				endpoints.Subsets = nil
			}
			if err := upsertEndpoints(ctx, fakeClient, key, endpoints); err != nil {
				return nil, fmt.Errorf("set endpoints: %w", err)
			}
		case "reconcile":
			times := step.Times
			if times <= 0 {
				times = 1
			}
			for i := 0; i < times; i++ {
				_, err := rec.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				if err != nil {
					report.ReconcileErrors = append(report.ReconcileErrors, err.Error())
				}
			}
		case "delete_service":
			svc := &corev1.Service{}
			if err := fakeClient.Get(ctx, key, svc); err != nil {
				if !apierrors.IsNotFound(err) {
					return nil, fmt.Errorf("get service for delete: %w", err)
				}
				continue
			}
			if err := fakeClient.Delete(ctx, svc); err != nil {
				return nil, fmt.Errorf("delete service: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown step op %q", step.Op)
		}
	}

	// Observe final mocked-k8s state.
	if err := observe(ctx, fakeClient, key, report); err != nil {
		return nil, err
	}
	return report, nil
}

func observe(ctx context.Context, c client.Client, key client.ObjectKey, report *Report) error {
	svc := &corev1.Service{}
	err := c.Get(ctx, key, svc)
	switch {
	case err == nil:
		report.ServiceExists = true
		report.Finalizers = svc.Finalizers
		for k, v := range svc.Annotations {
			report.Annotations[k] = v
		}
	case apierrors.IsNotFound(err):
		report.ServiceExists = false
	default:
		return fmt.Errorf("observe service: %w", err)
	}

	// Result ConfigMap written by the reconciler (<service>.result).
	cm := &corev1.ConfigMap{}
	cmKey := client.ObjectKey{Namespace: key.Namespace, Name: key.Name + ".result"}
	if err := c.Get(ctx, cmKey, cm); err == nil {
		report.ConfigMaps[cm.Name] = cm.Data
	}
	return nil
}

func upsertService(ctx context.Context, c client.Client, key client.ObjectKey, desired *corev1.Service) error {
	existing := &corev1.Service{}
	err := c.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return c.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	existing.Spec.Ports = desired.Spec.Ports
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	for k, v := range desired.Labels {
		existing.Labels[k] = v
	}
	return c.Update(ctx, existing)
}

func upsertEndpoints(ctx context.Context, c client.Client, key client.ObjectKey, desired *corev1.Endpoints) error {
	existing := &corev1.Endpoints{}
	err := c.Get(ctx, key, existing)
	if apierrors.IsNotFound(err) {
		return c.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	existing.Subsets = desired.Subsets
	return c.Update(ctx, existing)
}

func toPortSpecs(ports []Port) []testutil.PortSpec {
	out := make([]testutil.PortSpec, 0, len(ports))
	for _, p := range ports {
		out = append(out, testutil.PortSpec{Name: p.Name, Port: p.Port, TargetPort: p.TargetPort})
	}
	return out
}
