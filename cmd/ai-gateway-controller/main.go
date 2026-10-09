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

package main

import (
	"flag"
	"fmt"
	rt "runtime"

	uzap "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/yf-networks/ai-gateway-controller/internal/controllers"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	initFlags()
}

var (
	version string
	commit  string
)

func main() {
	zapOpts := zap.Options{
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	zapOpts.ZapOpts = append(zapOpts.ZapOpts, uzap.AddCaller())
	zapOpts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&zapOpts)))

	if help {
		flag.PrintDefaults()
		return
	}
	if showVersion {
		fmt.Printf("ai-gateway-controller version: %s\n", version)
		fmt.Printf("go version: %s\n", rt.Version())
		fmt.Printf("git commit: %s\n", commit)
		return
	}

	err := option.SetOptions(opts)
	if err != nil {
		setupLog.Error(err, "fail to start controllers")
		return
	}

	setupLog.Info("starting ai-gateway-controller")
	if err := controllers.Start(scheme); err != nil {
		setupLog.Error(err, "fail to start controllers")
	}
	setupLog.Info("ai-gateway-controller exit")
}
