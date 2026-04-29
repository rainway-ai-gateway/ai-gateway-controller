// Copyright (c) 2021 The BFE Authors.
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

package loadbalancer

import (
	"regexp"
	"runtime"
	"strconv"

	util "github.com/yf-networks/ai-gateway-controller/internal/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EventReason = ""

	OPTypeDelete = "delete"
	OPTypeUpdate = "update"
)

var goroutineRegex = regexp.MustCompile(`goroutine (\d+)`)

func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	match := goroutineRegex.FindStringSubmatch(string(buf[:n]))
	if len(match) < 2 {
		return 0
	}
	id, _ := strconv.ParseUint(match[1], 10, 64)
	return id
}

// func needDelete(svc *corev1.Service, getObjErr error) bool {
// 	if getObjErr != nil {
// 		if apierrors.IsNotFound(getObjErr) {
// 			util.K8sCLogger.Info("Service not found")
// 			return true
// 		}
// 	}

// 	if svc != nil && svc.DeletionTimestamp != nil && !svc.DeletionTimestamp.IsZero() {
// 		return true
// 	}
// 	return false
// }

func needDelete(obj client.Object, getObjErr error) bool {
	if getObjErr != nil {
		if apierrors.IsNotFound(getObjErr) {
			util.K8sCLogger.Info("K8s Object not found", "name", obj.GetName(), "ns", obj.GetNamespace())
			return true
		}
	}

	if obj != nil && obj.GetDeletionTimestamp() != nil && !obj.GetDeletionTimestamp().IsZero() {
		return true
	}

	return false
}
