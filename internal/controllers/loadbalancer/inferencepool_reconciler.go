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
	"context"
	"fmt"
	"sync"

	openapi "github.com/yf-networks/ai-gateway-controller/internal/alb"
	"github.com/yf-networks/ai-gateway-controller/internal/controllers/filter"
	"github.com/yf-networks/ai-gateway-controller/internal/datastore"
	"github.com/yf-networks/ai-gateway-controller/internal/option"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	inferenceApi "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	// "sigs.k8s.io/gateway-api-inference-extension/apix/v1alpha2"
	// "sigs.k8s.io/gateway-api-inference-extension/pkg/common"
	// logutil "sigs.k8s.io/gateway-api-inference-extension/pkg/common/util/logging"
	// "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/datalayer"
	// "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/datastore"
	// pooltuil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/pool"
)

// InferencePoolReconciler utilizes the controller runtime to reconcile Instance Gateway resources
// This implementation is just used for reading & maintaining data sync. The Gateway implementation
// will have the proper controller that will create/manage objects on behalf of the server inferencePool.
type InferencePoolReconciler struct {
	client.Reader
	inferPoolDict *datastore.InferPoolDict
	ExternalLB    *openapi.AlbProvider
	//PoolGKNN  common.GKNN

	mu sync.Mutex
}

func AddInferencePoolReconciler(mgr manager.Manager, inferPoolDict *datastore.InferPoolDict) (*InferencePoolReconciler, error) {
	reconciler := &InferencePoolReconciler{
		Reader:        mgr.GetClient(),
		inferPoolDict: inferPoolDict,
		ExternalLB:    openapi.NewAlbProvider(option.Opts.ExternalLB),
		// recorder: mgr.GetEventRecorderFor("ai-gateway-controller"),
	}

	if err := reconciler.SetupWithManager(mgr); err != nil {
		return nil, fmt.Errorf("unable to create service controller for loadbalancer: %s", err)
	}

	return reconciler, nil
}

func (c *InferencePoolReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var err error
	logger := util.K8sCLogger

	goId := getGoroutineID()
	logger.Info("Reconciling InferencePool", "goId", goId, "name", req.Name, "ns", req.Namespace)

	err = c.reconcileImpl(ctx, "From Reconciling InferencePool", req.Namespace, req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (c *InferencePoolReconciler) reconcileImpl(ctx context.Context, prompt, inferPoolNs, inferPoolName string) error {
	var poolNames []string
	var err error
	logger := util.K8sCLogger

	c.mu.Lock()
	defer c.mu.Unlock()

	pool := &inferenceApi.InferencePool{}
	err = c.Get(ctx, client.ObjectKey{
		Namespace: inferPoolNs,
		Name:      inferPoolName,
	}, pool)
	isDel := needDelete(pool, err)
	if !isDel && err != nil {
		logger.Info("reconcileImpl, get object failed but not del, skip", "inferPoolName", inferPoolName)
		return nil
	}

	product := option.Opts.ProductName

	goId := getGoroutineID()
	promptMsg := fmt.Sprintf("reconcileImpl(goId:%d) %s", goId, prompt)
	logger.Info(promptMsg, "name", pool.GetName(), "ns", pool.GetNamespace(), "isDel", isDel)

	if isDel {
		poolNames, err = c.ExternalLB.DeleteInferPoolProductPool(ctx, product, inferPoolName, inferPoolNs, option.Opts.ClusterName)
		logger.V(1).Info(promptMsg, "isDel", isDel, "name", inferPoolName, "ns", inferPoolNs, "poolNames", poolNames, "err", err)

		store, _ := c.inferPoolDict.Get(inferPoolName, inferPoolNs)
		if store != nil {
			poolReq := store.GetInferPoolData()
			logger.Info(promptMsg, "isDel", isDel, "poolReq", poolReq)
			c.inferPoolDict.Delete(store)
		}
		return err
	}

	//no del
	store := c.inferPoolDict.GetOrCreate(ctx, c.Reader, pool)
	if err = store.InferPoolUpdate(ctx, c.Reader, pool); err != nil {
		return err
	}

	poolReq := store.GetInferPoolData()
	poolNames, err = c.ExternalLB.EnsureInferPoolProductPool(ctx, product, poolReq, option.Opts.ClusterName)
	logger.V(1).Info(promptMsg, "isDel", isDel, "poolReq", poolReq, "poolNames", poolNames, "err", err)
	return err
}

func (c *InferencePoolReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&inferenceApi.InferencePool{}, builder.WithPredicates(filter.NamespaceFilter(), filter.EppXLabelFilter())).
		Complete(c)
}
