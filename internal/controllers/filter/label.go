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
	BfenetworksAnnotationPrefix = "k8s.bfenetworks.com/"
)

func isYingfeiRSService(service client.Object) bool {
	sname := service.GetName()
	labels := service.GetLabels()
	if labels != nil {
		lprodName, ipok := labels["bfe-product"]
		if ipok {
			if option.Opts.ProductName != lprodName {
				util.HdlLogger.V(1).Info("bfe-product label does not match Opts.ProductName skip", "sname", sname)
				return false
			} else {
				return true
			}
		}
		util.HdlLogger.V(1).Info("bfe-product label does not present. skip", "sname", sname)
		return false
	}
	return false
}

func isYingfeiTargetService(service client.Object) bool {

	if option.Opts.EnableRsPool {
		if isYingfeiRSService(service) {
			return true
		}
	}

	return false
}

func LabelFilter() predicate.Funcs {
	funcs := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		// labels := obj.GetLabels()
		// fmt.Printf("EppXLabelFilter: %s\n, labels:%+v", obj, labels)
		// if labels == nil {
		// 	return false
		// }

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

func isYingfeiEppXTargetObj(obj client.Object) bool {
	return true
}

func EppXLabelFilter() predicate.Funcs {
	funcs := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		bret := isYingfeiEppXTargetObj(obj)
		if bret {
			util.K8sCLogger.Info("EppXLabelFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		} else {
			util.K8sCLogger.V(1).Info("EppXLabelFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		}
		return bret
	})

	return funcs
}
