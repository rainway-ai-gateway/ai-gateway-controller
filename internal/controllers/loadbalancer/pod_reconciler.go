// Copyright (c) 2025 The BFE Authors.
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

package loadbalancer

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/datastore"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type PodReconciler struct {
	client.Reader
	inferPoolDict       *datastore.InferPoolDict
	ExternalLB          *openapi.AlbProvider
	inferPoolReconciler *InferencePoolReconciler
}

func AddPodReconciler(mgr manager.Manager, inferPoolReconciler *InferencePoolReconciler, ds *datastore.InferPoolDict) error {
	reconciler := &PodReconciler{
		Reader:        mgr.GetClient(),
		inferPoolDict: ds,
		ExternalLB:    openapi.NewAlbProvider(option.Opts.ExternalLB),
		// recorder: mgr.GetEventRecorderFor("ai-gateway-controller"),
		inferPoolReconciler: inferPoolReconciler,
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create service controller for loadbalancer: %s", err)
	}

	return nil
}

func (c *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := util.K8sCLogger

	pod := &corev1.Pod{}
	err := c.Get(ctx, req.NamespacedName, pod)
	isDel := c.needDelete(pod, err)
	goId := getGoroutineID()
	logger.Info("Reconciling Pod", "goId", goId, "name", pod.GetName(), "ns", pod.GetNamespace(), "isDel", isDel)

	var updateErr error
	isEncounterError := false

	if isDel {
		//storeDict := c.inferPoolDict.GetByRsPodDeleted(req.Name, req.Namespace)
		storeDict := c.inferPoolDict.UpdateByRsPodDeleted(req.Name, req.Namespace)
		for k, store := range storeDict {
			logger.Info("Pod delete", "storekey", k, "pod", pod, "inferpool", store.InferPoolName)
		}

		for key, store := range storeDict {
			err1 := c.inferPoolReconciler.reconcileImpl(ctx, "From Reconciling Pod(del)", store.InferPoolNs, store.InferPoolName)
			util.K8sCLogger.V(1).Info("Reconciling pod update(del) infer pool", "namespace", req.Namespace, "name", req.Name, "isdel", isDel, "key", key, "err", err)
			if err1 != nil {
				isEncounterError = true
				updateErr = err1
			}
		}

		if isEncounterError {
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	//not del
	//storeDict, storeDictDel := c.inferPoolDict.GetByRsPodLabelsMatch(pod)
	storeDict, storeDictDel := c.inferPoolDict.UpdateByRsPodLabelsMatch(pod)
	for key, store := range storeDict {
		err1 := c.inferPoolReconciler.reconcileImpl(ctx, "From Reconciling Pod(update-update)", store.InferPoolNs, store.InferPoolName)
		util.K8sCLogger.V(1).Info("Reconciling pod update(update-update) infer pool", "namespace", req.Namespace, "name", req.Name, "isdel", isDel, "key", key, "err", err)
		if err1 != nil {
			isEncounterError = true
			updateErr = err1
		}
	}

	for key, store := range storeDictDel {
		err1 := c.inferPoolReconciler.reconcileImpl(ctx, "From Reconciling Pod(update-del)", store.InferPoolNs, store.InferPoolName)
		util.K8sCLogger.V(1).Info("Reconciling pod update(update-del) infer pool", "namespace", req.Namespace, "name", req.Name, "isdel", isDel, "key", key, "err", err)
		if err1 != nil {
			isEncounterError = true
			updateErr = err1
		}
	}

	if isEncounterError {
		return ctrl.Result{}, updateErr
	}
	return ctrl.Result{}, nil
}

func (c *PodReconciler) needDelete(pod *corev1.Pod, err error) bool {
	if err != nil {
		if apierrors.IsNotFound(err) {
			return true
		}
	}
	if pod != nil && pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero() {
		return true
	}

	return false
}

func (c *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	filter := predicate.Funcs{
		CreateFunc: func(ce event.CreateEvent) bool {
			pod := ce.Object.(*corev1.Pod)
			bret := c.inferPoolDict.RsPodLabelsMatch(pod.GetLabels())
			util.K8sCLogger.V(1).Info("PodReconciler CreateFunc RsPodLabelsMatch", "podName", pod.Name, "bret", bret)
			return bret
		},
		UpdateFunc: func(ue event.UpdateEvent) bool {
			oldPod := ue.ObjectOld.(*corev1.Pod)
			newPod := ue.ObjectNew.(*corev1.Pod)
			//return c.inferPoolDict.RsPodLabelsMatch(oldPod.GetLabels()) || c.inferPoolDict.RsPodLabelsMatch(newPod.GetLabels())
			bret := c.inferPoolDict.RsPodLabelsMatch(oldPod.GetLabels()) || c.inferPoolDict.RsPodLabelsMatch(newPod.GetLabels())
			util.K8sCLogger.V(1).Info("PodReconciler UpdateFunc RsPodLabelsMatch", "new podName", newPod.Name, "old podName", oldPod.Name, "bret", bret)
			return bret
		},
		DeleteFunc: func(de event.DeleteEvent) bool {
			pod := de.Object.(*corev1.Pod)
			bret := c.inferPoolDict.RsPodLabelsMatch(pod.GetLabels())
			util.K8sCLogger.V(1).Info("PodReconciler DeleteFunc RsPodLabelsMatch", "podName", pod.Name, "bret", bret)
			return bret
		},
		GenericFunc: func(ge event.GenericEvent) bool {
			pod := ge.Object.(*corev1.Pod)
			//return c.inferPoolDict.RsPodLabelsMatch(pod.GetLabels())
			bret := c.inferPoolDict.RsPodLabelsMatch(pod.GetLabels())
			util.K8sCLogger.V(1).Info("PodReconciler GenericFunc RsPodLabelsMatch", "podName", pod.Name, "bret", bret)
			return bret
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(filter).
		Complete(c)
}
