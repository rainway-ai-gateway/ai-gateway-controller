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

package loadbalancer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/controllers/filter"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"
)

const (
	RainwayAIGatewayAnnotationPrefix = filter.RainwayAIGatewayAnnotationPrefix
	FinalizerName                    = RainwayAIGatewayAnnotationPrefix + "delete-protection"
	K8sPoolResultAnnotationKey       = RainwayAIGatewayAnnotationPrefix + "k8spool-result"
)

func AddServiceController(mgr manager.Manager) error {
	reconciler := newServiceReconciler(mgr)
	if err := reconciler.setupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create service controller for loadbalancer: %s", err)
	}

	return nil
}

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	ExternalLB *openapi.AlbProvider

	client.Client
	Scheme   *runtime.Scheme
	recorder record.EventRecorder
}

func newServiceReconciler(mgr manager.Manager) *ServiceReconciler {
	return &ServiceReconciler{
		ExternalLB: openapi.NewAlbProvider(option.Opts.ExternalLB),
		Client:     mgr.GetClient(),
		Scheme:     mgr.GetScheme(),
		recorder:   mgr.GetEventRecorderFor("ai-gateway-controller"),
	}
}

func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	svc := &corev1.Service{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: req.Namespace,
		Name:      req.Name,
	}, svc)

	isdel := needDelete(svc, err)
	util.K8sCLogger.Info("reconciling service", "namespace", req.Namespace, "name", req.Name, "isdel", isdel)

	if isdel && err != nil && option.Opts.SkipNilSvcDelete {
		//since we use finalizer, in this case(svc is empty), we can skip it
		util.K8sCLogger.Info("reconciling service, skip nil delete", "namespace", req.Namespace, "name", req.Name, "isdel", isdel)
		return ctrl.Result{}, nil
	}

	op := OPTypeDelete
	if !isdel {
		//newly create service, add finalizer firstly
		if !hasFinalizer(svc, FinalizerName) {
			err = r.addFinalizer(ctx, svc)
			if err == nil {
				util.K8sCLogger.Info("reconciling service succ to add finalizer", "namespace", req.Namespace, "name", req.Name, "isdel", isdel)
				//will handle it in next msg
				return ctrl.Result{}, err
			}
			util.K8sCLogger.Info("reconciling service failed to add finalizer", "namespace", req.Namespace, "name", req.Name, "isdel", isdel)
			return ctrl.Result{Requeue: true, RequeueAfter: 30 * time.Second}, err
		}
		op = OPTypeUpdate
		err = r.ensurePool(ctx, req.Namespace, req.Name, svc)
	} else {
		err = r.deletePool(ctx, svc)
		if err == nil || option.Opts.ForceRmFinalizer {
			r.removeFinalizer(ctx, svc)
		}
	}

	r.emitEvent(svc, err, req.Namespace, req.Name, op)
	r.handleResultConfigmap(ctx, req.Namespace, req.Name, err, op)

	if err != nil {
		if option.Opts.RetryIntervalUnitForErrS > 0 {
			util.HdlLogger.Error(err, "reconciling error, will retry...")
			return ctrl.Result{RequeueAfter: time.Duration(option.Opts.RetryIntervalUnitForErrS) * time.Second}, nil
		}
	}

	return ctrl.Result{}, err
}

func (r *ServiceReconciler) ensurePool(ctx context.Context, namespace string, name string, service *corev1.Service) error {
	ep := &corev1.Endpoints{}
	err := r.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, ep)
	if err != nil {
		return err
	}

	err = r.ensureK8sPool(ctx, service, ep)

	return err
}

func (r *ServiceReconciler) ensureK8sPool(ctx context.Context, service *corev1.Service, ep *corev1.Endpoints) error {
	// Report each named port's instance snapshot to its k8s pool (idempotent,
	// including empty snapshots). poolNames are the pools successfully reported.
	poolNames, err1 := r.ExternalLB.EnsureK8sPool(ctx, service, ep, option.Opts.ClusterName)

	// Diff against the previously reported pool names and delete the ones no
	// longer produced (port renamed/removed), keeping pools whose deletion
	// failed so they are retried on the next reconcile.
	annotation := service.Annotations[K8sPoolResultAnnotationKey]
	if annotation != "" {
		oldpools, err := extractPoolNames(annotation)
		if err == nil {
			diff := diffPoolNames(oldpools, poolNames)
			if err1 == nil {
				delnames, err := r.ExternalLB.DeleteK8sPoolByList(ctx, diff)
				if err != nil {
					util.HdlLogger.Error(err, "del diff k8s pools")
				}
				diff = diffPoolNames(diff, delnames)
			}
			poolNames = append(poolNames, diff...)
		}
	}
	r.addAnnotationByPoolNames(ctx, service, poolNames, K8sPoolResultAnnotationKey)

	return err1
}

func (r *ServiceReconciler) deletePool(ctx context.Context, service *corev1.Service) error {
	if service == nil {
		return nil
	}

	if service.Annotations == nil {
		return nil
	}

	annotation := service.Annotations[K8sPoolResultAnnotationKey]
	if annotation != "" {
		if poollist, err := extractPoolNames(annotation); err == nil {
			// delete the k8s pools
			_, err = r.ExternalLB.DeleteK8sPoolByList(ctx, poollist)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ServiceReconciler) addFinalizer(ctx context.Context, service *corev1.Service) error {
	patch := client.MergeFrom(service.DeepCopy())
	controllerutil.AddFinalizer(service, FinalizerName)

	if err := r.Patch(ctx, service, patch); err != nil {
		util.K8sCLogger.Info("failed to add finalizer", "FinalizerName", FinalizerName)
		return err
	}

	return nil
}

func (r *ServiceReconciler) removeFinalizer(ctx context.Context, service *corev1.Service) error {
	// remove our finalizer from the list and update it.
	if hasFinalizer(service, FinalizerName) {
		patch := client.MergeFrom(service.DeepCopy())
		controllerutil.RemoveFinalizer(service, FinalizerName)
		if err := r.Patch(ctx, service, patch); err != nil {
			util.K8sCLogger.Info("failed to remove finalizer", "FinalizerName", FinalizerName)
			return err
		}
	}
	return nil
}

func hasFinalizer(service *corev1.Service, finalizer string) bool {
	for _, item := range service.GetFinalizers() {
		if item == finalizer {
			return true
		}
	}
	return false
}

func extractPoolNames(annotation string) ([]string, error) {
	var poollist []string

	err := json.Unmarshal([]byte(annotation), &poollist)
	if err != nil {
		util.HdlLogger.Error(err, "unmarshal", "annotation", annotation)
		return nil, err
	}

	return poollist, nil
}

func diffPoolNames(oldlist, newlist []string) []string {
	diff := make([]string, 0, len(oldlist))
	for _, p := range oldlist {
		has := false
		for _, n := range newlist {
			if n == p {
				has = true
				break
			}
		}
		if !has {
			diff = append(diff, p)
		}
	}
	return diff
}

func (r *ServiceReconciler) addAnnotationByPoolNames(ctx context.Context, service *corev1.Service, pools []string, annotationKey string) error {
	patch := client.MergeFrom(service.DeepCopy())
	if service.Annotations == nil {
		service.Annotations = make(map[string]string)
	}

	jsonstr, _ := json.Marshal(pools)
	service.Annotations[annotationKey] = string(jsonstr)

	if err := r.Patch(ctx, service, patch); err != nil {
		util.K8sCLogger.Info("failed to add annotation", "key", annotationKey, "pool", string(jsonstr))
		return err
	}

	return nil
}

func (r *ServiceReconciler) handleResultConfigmap(ctx context.Context, ns string, name string, err error, op string) error {
	dstname := name + ".result"
	dst := &corev1.ConfigMap{}
	dst.ObjectMeta.Name = dstname
	dst.ObjectMeta.Namespace = ns

	cm := &corev1.ConfigMap{}
	terr := r.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      dstname,
	}, cm)

	if op == OPTypeDelete && err == nil {
		//succ to delete
		if terr == nil {
			_ = r.Delete(ctx, cm)
			util.HdlLogger.Info("delete result cm", "name", dstname)
			return nil
		}
	} else {
		dst.ObjectMeta.Labels = map[string]string{}
		dst.ObjectMeta.Labels["bfe-cm-result"] = "yes"
		dst.ObjectMeta.Labels["bfe-result-type"] = "service"
		dst.ObjectMeta.Labels["extra-msg"] = op

		dst.Data = map[string]string{}
		if err == nil {
			dst.Data["result"] = "Succ"
		} else {
			dst.Data["result"] = err.Error()
		}
		ts := time.Now()
		dst.Data["timestamp"] = ts.Format("2006-01-02 15:04:05.000")

		if terr != nil {
			terr = r.Create(ctx, dst)
		} else {
			terr = r.Update(ctx, dst)
		}

		if terr != nil {
			util.HdlLogger.Error(terr, "fail to put result cm")
		}
	}

	return nil
}

func (r *ServiceReconciler) emitEvent(object runtime.Object, err error, namespace, name string, extra string) {
	status := "OK"
	objName := namespace + "::" + name
	if err == nil {
		r.recorder.Event(object, corev1.EventTypeNormal, extra+" success For "+objName, status)
	} else {
		status = err.Error()
		r.recorder.Event(object, corev1.EventTypeWarning, extra+" failed For "+objName, status)
	}
	util.HdlLogger.Info("servie controller status", "object", objName, "op", extra, "msg", status)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) setupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}, builder.WithPredicates(filter.NamespaceFilter(), filter.LabelFilter())).
		Watches(
			&corev1.Endpoints{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(filter.NamespaceFilter(), filter.LabelFilter()),
		).
		Named("service-reconciler").
		Complete(r)
}
