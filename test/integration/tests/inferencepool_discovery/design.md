# inferencepool_discovery —— InferencePool 发现 集成测试用例设计

> 实现设计契约：`design-docs/sys-design/inferencepool-discovery-design.md`
> 执行：`sh test/scripts/run_tests.sh inferencepool_discovery`
> 被测对象：进程内 `InferencePoolReconciler.Reconcile`（EPP product pool 路径）

## 一、测试目标

验证 InferencePool 发现的核心行为：

1. **命名规则**：`<product>.k8s_<namespace>_<inferencepool-name>_inferp[_<clusterName>]`。
2. **实例组装**：来源为匹配 selector 的 Ready Pod，`ip=podIP`、`ports.Default=targetPort`、`weight=1`。
3. **池属性**：`type=IP`、`role=EPP`。
4. **EPP Server**：`domain = <endpointPickerRef.name>.<ns>.svc`、`port = endpointPickerRef.port.number`。
5. **selector 过滤**：仅匹配的 Ready Pod 纳入实例（非 Ready、不匹配标签的排除）。
6. **多 targetPort**：每个 Pod 每个 targetPort 一个 endpoint。
7. **删除路径**：删除 InferencePool 后 EPP 池被清理。

## 二、测试环境

| 项 | 值 |
|---|---|
| k8s | controller-runtime fake client（InferencePool/Pod） |
| alb api | `testutil.ALBMock`（instance-pools store） |
| options | `DefaultOpts()`，`product=AI_product`、`cluster=testk8s` |
| InferencePool | `selector.matchLabels={app: vllm}`、`targetPorts=[8200]`、`endpointPickerRef={name: vllm-sim-endpoint-picker, kind: Service, port: 9002}` |

## 三、测试用例

| # | case | 输入 | 断言 |
|---|---|---|---|
| 1 | `naming-and-instances` | 1 个 Ready Pod `10.0.0.1` | 池 `AI_product.k8s_yxy-it-infer_vllm-sim-pool_inferp_testk8s`；type=IP、role=EPP、1 实例 ip=10.0.0.1 port=8200、epp_server.domain/port 正确 |
| 2 | `multi-pod-instances` | 2 个 Ready Pod | 实例数=2 |
| 3 | `selector-filters-pods` | 匹配 Ready + 匹配非 Ready + 不匹配 Ready | 实例数=1（仅匹配 Ready） |
| 4 | `multi-target-ports` | targetPorts=[8200,8201]，1 Pod | 实例数=2，端口覆盖 8200/8201 |
| 5 | `delete-path` | 创建池后删除 InferencePool | EPP 池被删除 |

## 四、代码位置索引

| 位置 | 用途 |
|---|---|
| `internal/poolutil/poolutil.go:24-43` | InferencePoolToEndpointPool |
| `internal/datastore/datastore.go:149-199` | InferPoolUpdate |
| `internal/datastore/datastore.go:323-362` | podResyncAll |
| `internal/datastore/datastore.go:378-414` | GetInferPoolData |
| `internal/alb/albProvider.go:171-240` | EnsureInferPoolProductPool |
| `internal/alb/albProvider.go:242-276` | DeleteInferPoolProductPool |
| `internal/controllers/loadbalancer/inferencepool_reconciler.go:83-131` | reconcileImpl |
