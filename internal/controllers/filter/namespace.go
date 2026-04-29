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

package filter

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/yf-networks/ai-gateway-controller/internal/option"
	"github.com/yf-networks/ai-gateway-controller/internal/util"
)

func isYingfeiTargetNs(obj client.Object) bool {
	if len(option.Opts.NamespaceList) == 1 {
		if option.Opts.NamespaceList[0] == corev1.NamespaceAll || option.Opts.NamespaceList[0] == "*" {
			return true
		}
	}
	for _, ns := range option.Opts.NamespaceList {
		if ns == obj.GetNamespace() {
			return true
		}
	}
	return false
}

func NamespaceFilter() predicate.Funcs {
	funcs := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		bret := isYingfeiTargetNs(obj)
		util.K8sCLogger.V(1).Info("NamespaceFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		if bret {
			util.K8sCLogger.Info("NamespaceFilter", "kind", obj.GetObjectKind().GroupVersionKind(), "ns", obj.GetNamespace(), "name", obj.GetName(), "bret", bret)
		}

		return bret
	})

	return funcs
}
