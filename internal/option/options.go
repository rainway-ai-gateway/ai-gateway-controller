// Copyright (c) 2026 The BFE Authors.
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

package option

import (
	"fmt"
	"strings"

	"github.com/yf-networks/ai-gateway-controller/internal/option/externalLB"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultProductName         = "AI_product" //""
	DefaultClusterName         = "testk8s"    //"testk8s"
	DefaultIlBApiName          = ""           //"default"
	DefaultEnableRsPool        = true
	DefaultEnableInferencePool = true
	DefaultEnableAiPoolPath    = false

	MetricsBindAddress     = ":9080"
	HealthProbeBindAddress = ":9081"
	PProfAddress           = ""
	ReconcileRate          = 10
	ReconcileBucket        = 100

	ReadinessEndpointName = "/readyz"
	LivenessEndpointName  = "/healthz"
	UnreadyDuration       = 30
)

type Options struct {
	ClusterName      string
	ProductName      string
	EnableAiPoolPath bool

	EnableRsPool        bool //L4 or L7
	EnableInferencePool bool

	ExternalLB *externalLB.Options

	RetryIntervalUnitForErrS int

	ForceRmFinalizer bool

	Namespaces       string
	NamespaceList    []string
	SkipNilSvcDelete bool

	MetricsAddr           string
	HealthProbeAddr       string
	ReadinessEndpointName string
	UnreadyDuration       int
	LivenessEndpointName  string
	PProfAddr             string
	ReconcileRate         int
	ReconcileBucket       int
}

var (
	Opts *Options
)

func NewOptions() *Options {
	return &Options{
		ClusterName:         DefaultClusterName,
		ProductName:         DefaultProductName,
		EnableRsPool:        DefaultEnableRsPool,
		EnableInferencePool: DefaultEnableInferencePool,
		EnableAiPoolPath:    DefaultEnableAiPoolPath,

		Namespaces:            corev1.NamespaceAll,
		MetricsAddr:           MetricsBindAddress,
		HealthProbeAddr:       HealthProbeBindAddress,
		ReadinessEndpointName: ReadinessEndpointName,
		UnreadyDuration:       UnreadyDuration,
		LivenessEndpointName:  LivenessEndpointName,
		PProfAddr:             PProfAddress,
		ReconcileRate:         ReconcileRate,
		ReconcileBucket:       ReconcileBucket,

		ExternalLB: externalLB.NewOptions(),

		RetryIntervalUnitForErrS: 15,

		ForceRmFinalizer: false,

		SkipNilSvcDelete: true,
	}
}

var (
	log = ctrl.Log.WithName("controllers")
)

func SetOptions(option *Options) error {

	if option.UnreadyDuration <= 0 {
		return fmt.Errorf("invalid command line argument unready-duration, should > 0")
	}

	if option.ReconcileRate <= 0 {
		return fmt.Errorf("invalid command line argument reconcile-rate, should > 0")
	}

	if option.ReconcileBucket <= 0 {
		return fmt.Errorf("invalid command line argument reconcile-bucket, should > 0")
	}

	if err := option.ExternalLB.Check(); err != nil {
		return err
	}

	Opts = option
	Opts.NamespaceList = strings.Split(Opts.Namespaces, ",")

	Opts.ProductName = strings.TrimSpace(Opts.ProductName)
	if len(Opts.ProductName) <= 0 && Opts.EnableInferencePool {
		return fmt.Errorf("please set bfe-product-name when enable-inference-pool")
	}

	Opts.ClusterName = strings.TrimSpace(Opts.ClusterName)
	if len(Opts.ClusterName) <= 0 {
		return fmt.Errorf("please set k8s-cluster-name or use default(%s)", DefaultClusterName)
	}

	return nil
}
