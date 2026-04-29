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

package datastore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"github.com/yf-networks/ai-gateway-controller/internal/datalayer"
	"github.com/yf-networks/ai-gateway-controller/internal/poolutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	util "github.com/yf-networks/ai-gateway-controller/internal/util"
	"sigs.k8s.io/controller-runtime/pkg/client"

	inferenceApi "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	podutil "sigs.k8s.io/gateway-api-inference-extension/pkg/epp/util/pod"
)

var (
	errPoolNotSynced = errors.New("InferencePool is not initialized in data store")
)

func getInferPoolKey(inferPoolName string, inferPoolNs string) string {
	return fmt.Sprintf("%s_%s", inferPoolNs, inferPoolName)
}

type LLMPooEPPServiceEntry struct {
	ServiceName string
	Ns          string
	Port        int
}

func (obj *LLMPooEPPServiceEntry) GetDomain() string {
	return fmt.Sprintf("%s.%s.svc", obj.ServiceName, obj.Ns)
}

func (obj *LLMPooEPPServiceEntry) GetPort() int {
	return int(obj.Port)
}

func (obj *LLMPooEPPServiceEntry) GetUrl() string {
	return fmt.Sprintf("http://%s.%s.svc:%d", obj.ServiceName, obj.Ns, obj.Port)
}

type InferPoolRS struct {
	Hostname string
	IP       string
	Port     int
}

type InferPoolEPP struct {
	Hostname string
	IP       string
	Port     int
}

type InferPoolData struct {
	InferPoolName string
	InferPoolNs   string

	EppEntry LLMPooEPPServiceEntry
	RsPool   []*InferPoolRS
	EppPool  []*InferPoolEPP //TODO
}

type InferPoolStore struct {
	InferPoolName string
	InferPoolNs   string

	//epp k8s service info
	k8sServiceName string
	k8sServicePort int32

	// mu is used to synchronize access to pool, objectives, and rewrites.
	mu     sync.RWMutex
	rsPool *datalayer.EndpointPool //rsPool tag

	// key: epName(podName + ns + idx), value:endpoint info
	rsPods *sync.Map

	eppPool []*InferPoolEPP //extra epp pool
}

type DatastoreOption func(*InferPoolStore)

func WithEndpointPool(pool *datalayer.EndpointPool) DatastoreOption {
	return func(d *InferPoolStore) {
		d.rsPool = pool
	}
}

func NewInferPoolStore(inferPoolName string, inferPoolNs string, opts ...DatastoreOption) *InferPoolStore {
	// Initialize with defaults
	store := &InferPoolStore{
		InferPoolName: inferPoolName,
		InferPoolNs:   inferPoolNs,
		rsPool:        nil,
		mu:            sync.RWMutex{},
		rsPods:        &sync.Map{},
	}
	util.HdlLogger.Info("NewInferPoolStore", "inferPoolName", inferPoolName, "inferPoolNs", inferPoolNs)

	// Apply options
	for _, opt := range opts {
		opt(store)
	}

	return store
}

func (ds *InferPoolStore) GetInferPoolFullName() string {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return fmt.Sprintf("%s-%s", ds.InferPoolNs, ds.InferPoolName)
}

func (ds *InferPoolStore) Clear() {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.rsPool = nil

	// stop all pods go routines before clearing the pods map.
	ds.rsPods.Range(func(_, v any) bool {
		//ds.epf.ReleaseEndpoint(v.(backendmetrics.PodMetrics))
		return true
	})
	ds.rsPods.Clear()
}

func (ds *InferPoolStore) InferPoolUpdate(ctx context.Context, reader client.Reader, inferPool *inferenceApi.InferencePool) error {
	if inferPool == nil {
		return nil
	}

	stkeyName := getInferPoolKey(ds.InferPoolName, ds.InferPoolNs)

	var endpointPool *datalayer.EndpointPool
	endpointPool = poolutil.InferencePoolToEndpointPool(inferPool)
	if endpointPool == nil {
		ds.Clear()
		util.HdlLogger.Info("InferPoolUpdate, endpointPool is nil", "storekey", stkeyName)
		return nil
	}
	util.HdlLogger.Info("InferPoolUpdate", "storekey", stkeyName, "endpointPool", endpointPool)

	eppInfo := inferPool.Spec.EndpointPickerRef
	var k8sServiceName string = string(eppInfo.Name)
	var k8sServicePort int32 = int32(eppInfo.Port.Number)

	//util.HdlLogger.Info("InferPoolUpdate", "storekey", stkeyName, "k8sServiceName", k8sServiceName, "k8sServicePort", k8sServicePort)

	logger := util.HdlLogger

	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.k8sServicePort = k8sServicePort
	if ds.k8sServiceName != k8sServiceName {
		ds.k8sServiceName = k8sServiceName
		//update eppPool in epp service controller
		ds.eppPool = nil
	}

	oldEndpointPool := ds.rsPool
	ds.rsPool = endpointPool
	//if oldEndpointPool == nil || !labels.Equals(oldEndpointPool.Selector, endpointPool.Selector) {
	if oldEndpointPool == nil || !endpointPool.Equal(oldEndpointPool) {
		logger.Info("Updating endpoints", "selector", endpointPool.Selector)
		// A full resync is required to address two cases:
		// 1) At startup, the pod events may get processed before the pool is synced with the datastore,
		//    and hence they will not be added to the store since pool selector is not known yet
		// 2) If the selector on the pool was updated, then we will not get any pod events, and so we need
		//    to resync the whole pool: remove pods in the store that don't match the new selector and add
		//    the ones that may have existed already to the store.
		if err := ds.podResyncAll(ctx, reader); err != nil {
			return fmt.Errorf("failed to update pods according to the pool selector - %w", err)
		}
	}

	return nil
}

func (ds *InferPoolStore) PoolGet() (*datalayer.EndpointPool, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	if !ds.poolHasSyncedImpl() {
		return nil, errPoolNotSynced
	}
	return ds.rsPool, nil
}

func (ds *InferPoolStore) PoolHasSynced() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.poolHasSyncedImpl()
}

func (ds *InferPoolStore) poolHasSyncedImpl() bool {
	return ds.rsPool != nil
}

func (ds *InferPoolStore) PoolLabelsMatch(podLabels map[string]string) bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	if ds.rsPool == nil {
		return false
	}
	poolSelector := labels.SelectorFromSet(ds.rsPool.Selector)
	podSet := labels.Set(podLabels)
	return poolSelector.Matches(podSet)
}

func (ds *InferPoolStore) RsPodMatch(rsPodName string, rsPodNs string) bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	isMatched := false

	ds.rsPods.Range(func(k, v any) bool {
		ep := v.(datalayer.Endpoint)
		epMD := ep.GetMetadata()
		if epMD.PodName == rsPodName && epMD.PodNs == rsPodNs {
			tmpep := ep.(*datalayer.ModelServer)
			util.HdlLogger.Info("match rspod from store", "podName", rsPodName, "ep", tmpep.GetMetadata())
			isMatched = true
			return false
		}
		return true
	})
	return isMatched
}

func (ds *InferPoolStore) PodUpdateOrAddIfNotExist(pod *corev1.Pod) bool {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	return ds.podUpdateOrAddIfNotExistImpl(pod)
}

func (ds *InferPoolStore) podUpdateOrAddIfNotExistImpl(pod *corev1.Pod) bool {
	// if ds.rsPool == nil {
	// 	return true
	// }

	labels := make(map[string]string, len(pod.GetLabels()))
	for key, value := range pod.GetLabels() {
		labels[key] = value
	}

	pods := []*datalayer.EndpointMetadata{}
	for idx, port := range ds.rsPool.TargetPorts {
		pods = append(pods,
			&datalayer.EndpointMetadata{
				//This can be a bug in epp
				NamespacedName: types.NamespacedName{
					Name: pod.Name + "-rank-" + strconv.Itoa(idx),
					//Name:      pod.Name + "-" + strconv.Itoa(port),
					Namespace: pod.Namespace,
				},
				PodName: pod.Name,
				PodNs:   pod.Namespace,
				Address: pod.Status.PodIP,
				Port:    port,
				Labels:  labels,
			})
	}

	result := true
	for _, endpointMetadata := range pods {
		var ep datalayer.Endpoint
		existing, ok := ds.rsPods.Load(endpointMetadata.NamespacedName)
		util.HdlLogger.Info("add&update endpoint from store", "podName", endpointMetadata.PodName, "ep", endpointMetadata, "ok", ok)

		if !ok {
			ep = datalayer.NewEndpoint(endpointMetadata)
			ds.rsPods.Store(endpointMetadata.NamespacedName, ep)
			result = false
		} else {
			ep = existing.(*datalayer.ModelServer)
		}
		// Update endpoint properties if anything changed.
		ep.UpdateMetadata(endpointMetadata)
	}
	return result
}

func (ds *InferPoolStore) PodDelete(podName string, podNs string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.podDeleteImpl(podName, podNs)
}

func (ds *InferPoolStore) podDeleteImpl(podName string, podNs string) {

	ds.rsPods.Range(func(k, v any) bool {
		ep := v.(datalayer.Endpoint)
		if ep.GetMetadata().PodName == podName {
			tmpep := ep.(*datalayer.ModelServer)
			util.HdlLogger.Info("remvoe endpoint from store", "podName", podName, "ep", tmpep.GetMetadata())
			ds.rsPods.Delete(k)
		}
		return true
	})
}

func (ds *InferPoolStore) podResyncAll(ctx context.Context, reader client.Reader) error {
	logger := util.HdlLogger

	ds.rsPods.Clear()

	podList := &corev1.PodList{}
	if err := reader.List(ctx, podList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(ds.rsPool.Selector),
		Namespace:     ds.rsPool.Namespace,
	}); err != nil {
		return fmt.Errorf("failed to list pods - %w", err)
	}

	activePods := sets.New[string]()
	for _, pod := range podList.Items {
		if !podutil.IsPodReady(&pod) {
			continue
		}
		namespacedName := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
		activePods.Insert(pod.Name)
		if !ds.podUpdateOrAddIfNotExistImpl(&pod) {
			logger.Info("Pod added", "name", namespacedName)
		} else {
			logger.Info("Pod already exists", "name", namespacedName)
		}
	}

	// Remove pods that don't belong to the pool or not ready any more.
	ds.rsPods.Range(func(k, v any) bool {
		ep := v.(datalayer.Endpoint)
		epMD := ep.GetMetadata()
		if !activePods.Has(epMD.PodName) {
			logger.Info("Removing pod", "pod", epMD.PodName)
			ds.podDeleteImpl(epMD.PodName, epMD.PodNs)
		}
		return true
	})

	return nil
}

func (ds *InferPoolStore) UpdateEppPool(eppPool []*InferPoolEPP) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.eppPool = eppPool
}

func (ds *InferPoolStore) DeleteEppPool() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.eppPool = nil
}

func (ds *InferPoolStore) GetInferPoolData() *InferPoolData {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	ret := &InferPoolData{}

	ret.InferPoolName = ds.InferPoolName
	ret.InferPoolNs = ds.InferPoolNs

	ret.EppEntry.ServiceName = ds.k8sServiceName
	ret.EppEntry.Ns = ds.InferPoolNs
	ret.EppEntry.Port = int(ds.k8sServicePort)

	ds.rsPods.Range(func(k, v any) bool {
		ep := v.(datalayer.Endpoint)
		epMD := ep.GetMetadata()
		hostName := fmt.Sprintf("%s:%d", epMD.PodName, epMD.Port)
		inst := &InferPoolRS{
			Hostname: hostName,
			IP:       epMD.Address,
			Port:     epMD.Port,
		}
		ret.RsPool = append(ret.RsPool, inst)
		return true
	})

	tmpEppPool := make([]*InferPoolEPP, 0, len(ds.eppPool))
	for _, item := range ds.eppPool {
		if item == nil {
			continue
		}
		tmpEppPool = append(tmpEppPool, item)
	}
	ret.EppPool = tmpEppPool

	return ret
}

type InferPoolDict struct {
	mu            sync.Mutex
	inferPoolDict map[string]*InferPoolStore
}

func NewInferPoolDict() *InferPoolDict {
	ret := &InferPoolDict{}
	ret.inferPoolDict = make(map[string]*InferPoolStore)

	return ret
}

func (obj *InferPoolDict) Get(inferPoolName string, inferPoolNs string) (*InferPoolStore, bool) {
	key := getInferPoolKey(inferPoolName, inferPoolNs)
	obj.mu.Lock()
	defer obj.mu.Unlock()
	store, exists := obj.inferPoolDict[key]
	return store, exists
}

func (obj *InferPoolDict) GetOrCreate(ctx context.Context, reader client.Reader, inferPool *inferenceApi.InferencePool) *InferPoolStore {
	key := getInferPoolKey(inferPool.Name, inferPool.Namespace)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	if store, exists := obj.inferPoolDict[key]; exists {
		util.HdlLogger.Info("GetOrCreate(Get)", "key", key)
		return store
	}

	store := NewInferPoolStore(inferPool.Name, inferPool.Namespace)
	obj.inferPoolDict[key] = store
	util.HdlLogger.Info("GetOrCreate(Create)", "key", key)

	return store
}

func (obj *InferPoolDict) DeleteByNamespaceName(inferPoolName string, inferPoolNs string) bool {
	key := getInferPoolKey(inferPoolName, inferPoolNs)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	if _, exists := obj.inferPoolDict[key]; exists {
		delete(obj.inferPoolDict, key)
		return true
	}
	return false
}

func (obj *InferPoolDict) Delete(store *InferPoolStore) {
	if store == nil {
		return
	}

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		if v == store {
			delete(obj.inferPoolDict, k)
		}
	}
}

func (obj *InferPoolDict) GetByRsPodLabelsMatch(pod *corev1.Pod) (map[string]*InferPoolStore, map[string]*InferPoolStore) {
	ret := make(map[string]*InferPoolStore)
	retDel := make(map[string]*InferPoolStore)
	podLabels := pod.GetLabels()

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		isMatched := v.PoolLabelsMatch(podLabels)
		if isMatched {
			ret[k] = v
		} else {
			isMatched := v.RsPodMatch(pod.Name, pod.Namespace)
			if isMatched {
				retDel[k] = v
			}
		}
	}

	return ret, retDel
}

func (obj *InferPoolDict) UpdateByRsPodLabelsMatch(pod *corev1.Pod) (map[string]*InferPoolStore, map[string]*InferPoolStore) {
	ret := make(map[string]*InferPoolStore)
	retDel := make(map[string]*InferPoolStore)
	podLabels := pod.GetLabels()
	rsPodName := pod.Name

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, store := range obj.inferPoolDict {
		isMatched := store.PoolLabelsMatch(podLabels)
		if isMatched { //add or update
			ret[k] = store
			if !store.PoolHasSynced() {
				util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Skip since inferencePool is not available yet", "storekey", k)
				continue
			}

			util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod update", "storekey", k, "rsPodName", rsPodName)
			if !poolutil.IsPodReady(pod) {
				util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod is not ready. remove")
				store.PodDelete(pod.Name, pod.Namespace)
			} else {
				if store.PodUpdateOrAddIfNotExist(pod) {
					util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod added")
				} else {
					util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod already exists")
				}
			}
		} else { //del
			isMatched := store.RsPodMatch(pod.Name, pod.Namespace)
			if isMatched {
				retDel[k] = store
				store.PodDelete(pod.Name, pod.Namespace)
				util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod del", "rsPodName", rsPodName)
			}
		}
	}

	return ret, retDel
}

func (obj *InferPoolDict) RsPodLabelsMatch(podLabels map[string]string) bool {
	isMatched := false
	obj.mu.Lock()
	defer obj.mu.Unlock()

	for _, v := range obj.inferPoolDict {
		if v.PoolLabelsMatch(podLabels) {
			isMatched = true
			break
		}
	}

	return isMatched
}

func (obj *InferPoolDict) GetByRsPodDeleted(rsPodName string, rsPodNs string) map[string]*InferPoolStore {
	ret := make(map[string]*InferPoolStore)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		isMatched := v.RsPodMatch(rsPodName, rsPodNs)
		if isMatched {
			ret[k] = v
		}
	}

	return ret
}

func (obj *InferPoolDict) UpdateByRsPodDeleted(rsPodName string, rsPodNs string) map[string]*InferPoolStore {
	ret := make(map[string]*InferPoolStore)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		isMatched := v.RsPodMatch(rsPodName, rsPodNs)
		if isMatched {
			v.PodDelete(rsPodName, rsPodNs)
			ret[k] = v
			util.HdlLogger.Info("UpdateByRsPodLabelsMatch, Pod del", "rsPodName", rsPodName)
		}
	}

	return ret
}

func (obj *InferPoolDict) List() map[string]*InferPoolStore {
	obj.mu.Lock()
	defer obj.mu.Unlock()

	result := make(map[string]*InferPoolStore, len(obj.inferPoolDict))
	for k, v := range obj.inferPoolDict {
		result[k] = v
	}
	return result
}

func (obj *InferPoolDict) GetByInferPoolEppService(eppServiceName, eppServiceNs string) map[string]*InferPoolStore {
	ret := make(map[string]*InferPoolStore)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		if v.k8sServiceName == eppServiceName && v.InferPoolNs == eppServiceNs {
			ret[k] = v
		}
	}
	return ret
}

func (obj *InferPoolDict) UpdateEppPool(eppServiceName, eppServiceNs string, eppPool []*InferPoolEPP) map[string]*InferPoolStore {
	ret := make(map[string]*InferPoolStore)
	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		if v.k8sServiceName == eppServiceName && v.InferPoolNs == eppServiceNs {
			ret[k] = v
			v.UpdateEppPool(eppPool)
		}
	}
	return ret
}

func (obj *InferPoolDict) DeleteEppPool(eppServiceName, eppServiceNs string) map[string]*InferPoolStore {
	ret := make(map[string]*InferPoolStore)

	obj.mu.Lock()
	defer obj.mu.Unlock()

	for k, v := range obj.inferPoolDict {
		if v.k8sServiceName == eppServiceName && v.InferPoolNs == eppServiceNs {
			v.DeleteEppPool()
			ret[k] = v
		}
	}
	return ret
}
