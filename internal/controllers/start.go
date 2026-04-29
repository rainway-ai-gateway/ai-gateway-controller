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

package controllers

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"syscall"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/yf-networks/ai-gateway-controller/internal/controllers/loadbalancer"
	"github.com/yf-networks/ai-gateway-controller/internal/controllers/readiness"
	"github.com/yf-networks/ai-gateway-controller/internal/datastore"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
)

var (
	log = ctrl.Log.WithName("controllers")
)

func Start(scheme *runtime.Scheme) error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("unable to get client config: %s", err)
	}

	mgr, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: option.Opts.MetricsAddr},
		HealthProbeBindAddress: option.Opts.HealthProbeAddr,
		LivenessEndpointName:   option.Opts.LivenessEndpointName,
		ReadinessEndpointName:  option.Opts.ReadinessEndpointName,
	})
	if err != nil {
		return fmt.Errorf("unable to start controller manager: %s", err)
	}

	ctx := ctrl.SetupSignalHandler()

	if err := startExternalLB(mgr); err != nil {
		return err
	}

	if err := mgr.AddHealthzCheck(option.Opts.LivenessEndpointName, healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %s", err)
	}
	if err := mgr.AddReadyzCheck(option.Opts.ReadinessEndpointName, readiness.Checker); err != nil {
		return fmt.Errorf("unable to set up ready check: %s", err)
	}

	startPProfListener()

	log.Info("starting manager")
	readiness.SetReady(readiness.EventRunning)

	if err := mgr.Start(ctx); err != nil {
		readiness.SetUnready(readiness.EventRunning)
		return fmt.Errorf("fail to run manager: %s", err)
	}
	readiness.SetUnready(readiness.EventRunning)
	log.Info("exit manager")

	return nil
}

func startExternalLB(mgr manager.Manager) error {
	//ctx, cancel := context.WithCancel(context.Background())
	if option.Opts.EnableRsPool {
		log.Info("add L7 ServiceController")
		if err := loadbalancer.AddServiceController(mgr); err != nil {
			return err
		}
	}

	if option.Opts.EnableInferencePool {
		log.Info("add InferencePool Controller")
		dsDict := datastore.NewInferPoolDict()
		rootReconciler, err := loadbalancer.AddInferencePoolReconciler(mgr, dsDict)
		if err != nil {
			return err
		}

		if err := loadbalancer.AddPodReconciler(mgr, rootReconciler, dsDict); err != nil {
			return err
		}
	}

	return nil
}

func startPProfListener() {
	if len(option.Opts.PProfAddr) <= 0 {
		return
	}

	go func() {
		if err := http.ListenAndServe(option.Opts.PProfAddr, nil); err != nil {
			log.Error(err, "fail to start listener for pprof")
			raise(syscall.SIGTERM)
		}
	}()
}

func raise(sig os.Signal) error {
	p, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return p.Signal(sig)
}
