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

package openapi

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/yf-networks/ai-gateway-controller/internal/alb/apis/k8s_pool"
	"github.com/yf-networks/ai-gateway-controller/internal/option/externalLB"
	util "github.com/yf-networks/ai-gateway-controller/internal/util"
	v1 "k8s.io/api/core/v1"
)

// K8sPoolNameMaxLength mirrors the k8s_pools name validation (k8s-pools.md §2).
const K8sPoolNameMaxLength = 64

type AlbProvider struct {
	options *externalLB.Options

	// innerClient is the k8s_pools (InnerAPI) client used by Service discovery.
	innerClient K8sPoolClient
}

func NewAlbProvider(opts *externalLB.Options) *AlbProvider {
	return &AlbProvider{
		options: opts,
		innerClient: NewInnerApiClient(
			opts.ApiServerAddr,
			opts.Token,
			opts.Timeout),
	}
}

// getK8sPoolInstances extracts the discovered instances for the named port from
// the Endpoints (Ready addresses only) as k8s_pool instances. Instances are
// deduplicated by (addr, port) to satisfy the k8s_pools uniqueness rule.
func getK8sPoolInstances(ep *v1.Endpoints, portName string) []*k8s_pool.Instance {
	instances := make([]*k8s_pool.Instance, 0)
	seen := make(map[string]struct{})

	for _, subset := range ep.Subsets {
		for _, p := range subset.Ports {
			if p.Name == portName {
				for _, addr := range subset.Addresses {
					key := fmt.Sprintf("%s|%d", addr.IP, int(p.Port))
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					instances = append(instances, &k8s_pool.Instance{
						Addr: addr.IP,
						Port: int(p.Port),
					})
				}
				break
			}
		}
	}

	return instances
}

// EnsureK8sPool reports each named port's instance snapshot to its k8s_pool via
// the InnerAPI (full replacement, idempotent upsert), and returns the pool names
// successfully reported. An empty snapshot is still reported so a drained pool
// becomes "zero instances" instead of keeping stale members.
func (p *AlbProvider) EnsureK8sPool(ctx context.Context, service *v1.Service,
	ep *v1.Endpoints, clusterName string) ([]string, error) {

	namespace := service.GetNamespace()
	name := service.GetName()
	poolNames := make([]string, 0, len(service.Spec.Ports))

	for _, port := range service.Spec.Ports {
		portName := port.Name
		if portName == "" {
			continue
		}

		pool, err := k8sPoolName(namespace, name, portName, clusterName)
		if err != nil {
			return poolNames, err
		}

		servers := getK8sPoolInstances(ep, portName)
		if len(servers) == 0 {
			util.HdlLogger.Info("k8s pool instance is empty, report zero instances", "poolname", pool)
		}

		if _, _, err := p.innerClient.ReplaceK8sPoolInstances(ctx, pool, servers); err != nil {
			util.HdlLogger.Error(err, "failed to replace k8s pool instances",
				"poolname", pool, "instances", len(servers))
			return poolNames, err
		}
		util.HdlLogger.Info("replace k8s pool instances succ", "poolname", pool, "instances", len(servers))
		poolNames = append(poolNames, pool)
	}
	return poolNames, nil
}

// DeleteK8sPoolByList deletes the given k8s pools (404 treated as success) and
// returns the pools actually deleted.
func (p *AlbProvider) DeleteK8sPoolByList(ctx context.Context, poolNames []string) ([]string, error) {
	deleted := make([]string, 0, len(poolNames))

	for _, pool := range poolNames {
		if err := p.innerClient.DeleteK8sPool(ctx, pool); err != nil {
			util.HdlLogger.Error(err, "delete k8s pool", "poolname", pool)
			return deleted, err
		}
		util.HdlLogger.Info("delete k8s pool succ", "poolname", pool)
		deleted = append(deleted, pool)
	}

	return deleted, nil
}

// k8sPoolName derives the deterministic k8s_pool name for a Service port:
//
//	k8s_<namespace>_<service-name>_<port-name>[_<cluster-name>]
//
// If the result exceeds the k8s_pools name limit it is folded to a readable
// prefix plus an 8-char hash suffix so it stays unique and valid.
func k8sPoolName(namespace, name, portName, clusterName string) (string, error) {
	identity := namespace + "|" + name + "|" + portName + "|" + clusterName

	pool := fmt.Sprintf("k8s_%s_%s_%s", namespace, name, portName)
	if clusterName != "" {
		pool = fmt.Sprintf("%s_%s", pool, clusterName)
	}

	if len(pool) > K8sPoolNameMaxLength {
		pool = fmt.Sprintf("%s_%s", pool[:K8sPoolNameMaxLength-9], shortHash8(identity))
	}

	if err := validK8sPoolName(pool); err != nil {
		return "", fmt.Errorf("invalid k8s pool name %q: %s", pool, err)
	}
	return pool, nil
}

func shortHash8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)[:8]
}

// validK8sPoolName mirrors iprovider.K8sPoolName: 1-64 chars, only letters,
// digits, '_', '-', '.', and no leading/trailing '.', '-', '_'.
func validK8sPoolName(s string) error {
	if len(s) < 1 || len(s) > K8sPoolNameMaxLength {
		return fmt.Errorf("length must be between 1 and %d", K8sPoolNameMaxLength)
	}

	first, last := s[0], s[len(s)-1]
	if first == '.' || first == '-' || first == '_' {
		return fmt.Errorf("must not start with '.', '-' or '_'")
	}
	if last == '.' || last == '-' || last == '_' {
		return fmt.Errorf("must not end with '.', '-' or '_'")
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-' || c == '.':
		default:
			return fmt.Errorf("contains invalid character %q", string(c))
		}
	}
	return nil
}
