// Copyright(c) 2026 Beijing Yingfei Networks Technology Co.Ltd.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http: //www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

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
	DefaultClusterName = "testk8s" //"testk8s"
	DefaultIlBApiName  = ""        //""

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
	ClusterName string

	ApiName    string
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
		ClusterName: DefaultClusterName,
		ApiName:     DefaultIlBApiName,

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

	Opts.ApiName = strings.TrimSpace(Opts.ApiName)
	if len(Opts.ApiName) <= 0 {
		return fmt.Errorf("please set ai-gateway-api-name")
	}

	Opts.ClusterName = strings.TrimSpace(Opts.ClusterName)
	if len(Opts.ClusterName) <= 0 {
		return fmt.Errorf("please set k8s-cluster-name or use default(%s)", DefaultClusterName)
	}

	return nil
}
