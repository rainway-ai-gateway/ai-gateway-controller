// Copyright (c) 2026 The BFE Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package poolutil

import (
	"github.com/yf-networks/ai-gateway-controller/internal/datalayer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	inferenceApi "sigs.k8s.io/gateway-api-inference-extension/api/v1"
)

func InferencePoolToEndpointPool(inferencePool *inferenceApi.InferencePool) *datalayer.EndpointPool {
	if inferencePool == nil {
		return nil
	}
	targetPorts := make([]int, 0, len(inferencePool.Spec.TargetPorts))
	for _, p := range inferencePool.Spec.TargetPorts {
		targetPorts = append(targetPorts, int(p.Number))
	}
	selector := make(map[string]string, len(inferencePool.Spec.Selector.MatchLabels))
	for k, v := range inferencePool.Spec.Selector.MatchLabels {
		selector[string(k)] = string(v)
	}
	endpointPool := &datalayer.EndpointPool{
		Selector:    selector,
		TargetPorts: targetPorts,
		Namespace:   inferencePool.Namespace,
		Name:        inferencePool.Name,
	}
	return endpointPool
}

func EndpointPoolToInferencePool(endpointPool *datalayer.EndpointPool) *inferenceApi.InferencePool {
	targetPorts := make([]inferenceApi.Port, 0, len(endpointPool.TargetPorts))
	for _, p := range endpointPool.TargetPorts {
		targetPorts = append(targetPorts, inferenceApi.Port{Number: inferenceApi.PortNumber(p)})
	}
	labels := make(map[inferenceApi.LabelKey]inferenceApi.LabelValue, len(endpointPool.Selector))
	for k, v := range endpointPool.Selector {
		labels[inferenceApi.LabelKey(k)] = inferenceApi.LabelValue(v)
	}

	inferencePool := &inferenceApi.InferencePool{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "inference.networking.k8s.io/v1",
			Kind:       "InferencePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      endpointPool.Name,
			Namespace: endpointPool.Namespace,
		},
		Spec: inferenceApi.InferencePoolSpec{
			Selector:    inferenceApi.LabelSelector{MatchLabels: labels},
			TargetPorts: targetPorts,
		},
	}
	return inferencePool
}

func IsPodReady(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			if condition.Status == corev1.ConditionTrue {
				return true
			}
			break
		}
	}
	return false
}
