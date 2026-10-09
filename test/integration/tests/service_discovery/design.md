# service_discovery —— 普通 Service 发现 集成测试用例设计

> 实现设计契约：`design-docs/sys-design/service-discovery-design.md`
> 执行：`sh test/scripts/run_tests.sh service_discovery`
> 被测对象：进程内 `ServiceReconciler.Reconcile`（K8s 实例池 k8s_pools 上报路径）

## 一、测试目标

验证普通 Service 发现的核心行为：

1. **命名规则**：`k8s_<namespace>_<name>_<portname>[_<clusterName>]`。
2. **实例组装**：`addr=pod ip`、`port=endpointPort`、`weight` 省略（API 默认 100）；多 Pod → 多实例。
3. **多端口**：每个命名端口上报一个池。
4. **端口未命名跳过**：`port.name == ""` 跳过该端口。
5. **空 Endpoints**：上报空数组（零实例），池条目仍创建，annotation 记录池名。
6. **结果 annotation**：`k8s.bfenetworks.com/k8spool-result` 写回（JSON 字符串数组）。
7. **diff 收敛**：端口减少后，不再期望的池被删除。
8. **finalizer**：首次 reconcile 自动添加 `k8s.bfenetworks.com/delete-protection`。
9. **删除路径**：删除 Service 后 finalizer 被移除，k8s 池与 result cm 被清理。
10. **result ConfigMap**：成功路径 `result=Succ`；失败路径（ALB 不可达）`result=错误信息`。

## 二、测试环境

| 项 | 值 |
|---|---|
| k8s | controller-runtime fake client（Service/Endpoints/ConfigMap） |
| alb api | `testutil.ALBMock`（`/inner-api/v1/k8s_pools` store） |
| options | `DefaultOpts()`，`apiName=rainway-ai-gatway-demo`、`cluster=testk8s` |
| Service | `ai-gateway-api-name=rainway-ai-gatway-demo` |

## 三、测试用例

| # | case | 输入 | 断言 |
|---|---|---|---|
| 1 | `naming-and-instances` | svc `http`(8080→80)，1 Pod `10.244.0.99` | 池 `k8s_yxy-it-service_it-svc_http_testk8s`；实例 addr=ip、port=80 |
| 2 | `multi-port` | 端口 http/grpc | 上报 2 个池 |
| 3 | `unnamed-port-skipped` | 端口1 无 name，端口2 有 name | 只创建 1 个池 |
| 4 | `empty-endpoints` | 无 Endpoints 实例 | 创建空池（0 实例），annotation 含池名 |
| 5 | `multi-pod-instances` | 2 个 Pod IP | 池实例数=2 |
| 6 | `writes-result-annotation` | 普通 reconcile | svc 注解 `k8s.bfenetworks.com/k8spool-result` 含池名 |
| 7 | `diff-converge-removes-pool` | 先 2 端口创建 2 池，再改 1 端口 | 旧池被删除，剩 1 池 |
| 8 | `adds-finalizer` | 首次 reconcile | svc 带 `k8s.bfenetworks.com/delete-protection` |
| 9 | `delete-path` | 创建池后删除 Service | 池删除、Service 移除、result cm 清理 |
| 10 | `result-cm` | 成功 reconcile | result cm `data.result=Succ`、labels 正确 |
| 11 | `result-cm-fail-path` | ALB 不可达 | result cm `data.result != Succ` |

## 四、代码位置索引

| 位置 | 用途 |
|---|---|
| `internal/alb/albProvider.go` | `EnsureK8sPool` / `DeleteK8sPoolByList` / `getK8sPoolInstances` / `k8sPoolName` |
| `internal/alb/innerApiClient.go` | `InnerApiClient`（k8s_pools InnerAPI 调用、404 幂等） |
| `internal/controllers/loadbalancer/service_controller.go` | `Reconcile` / `ensurePool` / `ensureK8sPool` / `deletePool` / `handleResultConfigmap` |
| `test/integration/testutil/alb_mock.go` | k8s_pools InnerAPI mock（池 store + access log） |
| `test/integration/testutil/reconciler.go` | `NewEnv` / `NewDownALBEnv` / `ReconcileUntil` |
| `test/integration/tests/service_discovery/service_discovery_test.go` | TC-01 ~ TC-11 |
