// Copyright (c) 2025 The BFE Authors.
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

package main

import (
	"flag"

	"github.com/yf-networks/ai-gateway-controller/internal/option"
)

var (
	help        bool
	showVersion bool

	opts *option.Options = option.NewOptions()
)

func initFlags() {
	flag.BoolVar(&help, "help", false, "Show help.")
	flag.BoolVar(&help, "h", false, "Show help.")

	flag.BoolVar(&showVersion, "version", false, "Show version of bfe-ingress-controller.")
	flag.BoolVar(&showVersion, "v", false, "Show version of bfe-ingress-controller.")

	flag.BoolVar(&opts.EnableRsPool, "enable-rs-pool", opts.EnableRsPool, "enable l7 pool")
	flag.BoolVar(&opts.EnableInferencePool, "enable-inference-pool", opts.EnableInferencePool, "enable inference pool")

	flag.StringVar(&opts.ExternalLB.ApiServerAddr, "bfe-api-addr", opts.ExternalLB.ApiServerAddr, "Address of ALB api server")
	flag.StringVar(&opts.ExternalLB.Token, "bfe-api-token", opts.ExternalLB.Token, "access token of ALB api server")

	flag.StringVar(&opts.ClusterName, "k8s-cluster-name", opts.ClusterName, "k8s cluster name")
	flag.StringVar(&opts.ProductName, "bfe-product-name", opts.ProductName, "default product name")

	flag.IntVar(&opts.RetryIntervalUnitForErrS, "retry-interval-unit-sec", -1, "retry interval second(<=0, means use default retry interval)")
	flag.BoolVar(&opts.ForceRmFinalizer, "force-rm-finalizer", false, "will remove finalizer even deleting failed")

	flag.StringVar(&opts.Namespaces, "namespace", opts.Namespaces, "Namespaces to watch, delimited by ',', '*' for all.")
	flag.StringVar(&opts.Namespaces, "n", opts.Namespaces, "Namespaces to watch, delimited by ',', '*' for all.")
	flag.BoolVar(&opts.SkipNilSvcDelete, "skip-nil-svc-delete", true, "is skip nil service delete")

	flag.StringVar(&opts.MetricsAddr, "metrics-bind-address", opts.MetricsAddr, "The address the metric endpoint binds to.")
	flag.StringVar(&opts.HealthProbeAddr, "health-probe-bind-address", opts.HealthProbeAddr, "The address the probe endpoint binds to.")
	flag.StringVar(&opts.ReadinessEndpointName, "readiness-endpoint-name", opts.ReadinessEndpointName, "Readiness probe endpoint name")
	flag.StringVar(&opts.LivenessEndpointName, "liveness-endpoint-name", opts.LivenessEndpointName, "Liveness probe endpoint name")
	flag.StringVar(&opts.PProfAddr, "pprof-address", opts.PProfAddr, "The address to enable pprof, for example :6060.")
	flag.IntVar(&opts.UnreadyDuration, "unready-duration", opts.UnreadyDuration, "keep unready when starting for a period of time, in second")
	flag.IntVar(&opts.ReconcileRate, "reconcile-rate", opts.ReconcileRate, "Set rate limit in processing reconcile request (per second).")
	flag.IntVar(&opts.ReconcileBucket, "reconcile-bucket", opts.ReconcileBucket, "Set ratelimiter bucket size for reconcile request.")

}
