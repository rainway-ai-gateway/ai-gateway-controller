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

package loadbalancer

import (
	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/datastore"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewTestReconciler builds a ServiceReconciler for in-process integration tests.
// It is equivalent to newServiceReconciler but injects a fake client and a fake
// event recorder, so tests can drive Reconcile without a controller manager.
func NewTestReconciler(c client.Client, scheme *runtime.Scheme, lb *openapi.AlbProvider, recorder record.EventRecorder) *ServiceReconciler {
	return &ServiceReconciler{
		ExternalLB: lb,
		Client:     c,
		Scheme:     scheme,
		recorder:   recorder,
	}
}

// NewTestInferencePoolReconciler builds an InferencePoolReconciler for in-process
// integration tests. It is equivalent to the reconciler built by
// AddInferencePoolReconciler, but injects a fake client and a fresh InferPoolDict.
func NewTestInferencePoolReconciler(c client.Reader, dict *datastore.InferPoolDict, lb *openapi.AlbProvider) *InferencePoolReconciler {
	return &InferencePoolReconciler{
		Reader:        c,
		inferPoolDict: dict,
		ExternalLB:    lb,
	}
}
