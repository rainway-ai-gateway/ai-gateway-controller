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

package filter

import (
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	RainwayAIGatewayAnnotationPrefix = "k8s.bfenetworks.com/"
	RainwayAIGatewayAPIName          = "ai-gateway-api-name"
)

func isYingfeiRSService(service client.Object) bool {
	sname := service.GetName()
	labels := service.GetLabels()
	if labels != nil {
		apiName, ok := labels[RainwayAIGatewayAPIName]
		if ok {
			if option.Opts.ApiName == apiName || option.Opts.ApiName == "*" {
				return true
			}
			util.HdlLogger.V(1).Info("ai-gateway-api-name label does not match Opts.ApiName skip", "sname", sname)
		}
		util.HdlLogger.V(1).Info("ai-gateway-api-name label does not present. skip", "sname", sname)
		return false
	}
	return false
}

func isYingfeiTargetService(service client.Object) bool {
	if isYingfeiRSService(service) {
		return true
	}

	return false
}

func LabelFilter() predicate.Funcs {
	funcs := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		bret := isYingfeiTargetService(obj)
		if bret {
			util.K8sCLogger.Info("LabelFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		} else {
			util.K8sCLogger.V(1).Info("LabelFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		}
		return bret
	})

	return funcs
}
