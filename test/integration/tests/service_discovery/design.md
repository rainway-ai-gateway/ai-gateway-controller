# service_discovery —— 普通 Service 发现 集成测试用例设计

> 实现设计契约：`design-docs/sys-design/service-discovery-design.md`
> 执行：`sh test/scripts/run_tests.sh service_discovery`
> 被测对象：进程内 `ServiceReconciler.Reconcile`（L7 product instance pool 路径）

## 一、测试目标

验证普通 Service 发现的核心行为：

1. **命名规则**：`<product>.k8s_<namespace>_<name>_<portname>[_<clusterName>]`。
2. **实例组装**：`hostname=ip`、`ip`、`weight=1`、`ports{Default: endpointPort}`；多 Pod → 多实例。
3. **多端口**：每个命名端口创建一个池。
4. **端口未命名跳过**：`port.name == ""` 跳过该端口。
5. **空 Endpoints**：不调用 ALB，但 annotation 记录池名。
6. **结果 annotation**：`productpool-result` 写回。
7. **diff 收敛**：端口减少后，不再期望的池被删除。
8. **finalizer**：首次 reconcile 自动添加 `k8s.bfenetworks.com/delete-protection`。
9. **删除路径**：删除 Service 后产品池与 result cm 被清理。
10. **result ConfigMap**：成功路径 `result=Succ`；失败路径（ALB 不可达）`result=错误信息`。

## 二、测试环境

| 项 | 值 |
|---|---|
| k8s | controller-runtime fake client（Service/Endpoints/ConfigMap） |
| alb api | `testutil.ALBMock`（instance-pools store） |
| options | `DefaultOpts()`，`product=AI_product`、`cluster=testk8s` |
| Service | `bfe-product=AI_product` |

## 三、测试用例

| # | case | 输入 | 断言 |
|---|---|---|---|
| 1 | `naming-and-instances` | svc `http`(8080→80)，1 Pod `10.244.0.99` | 池 `AI_product.k8s_yxy-it-service_it-svc_http_testk8s`；实例 hostname=ip、weight=1、ports.Default=80 |
| 2 | `multi-port` | 端口 http/grpc | 创建 2 个池 |
| 3 | `unnamed-port-skipped` | 端口1 无 name，端口2 有 name | 只创建 1 个池 |
| 4 | `empty-endpoints` | 无 Endpoints 实例 | 不创建池，但 annotation 含池名 |
| 5 | `multi-pod-instances` | 2 个 Pod IP | 池实例数=2 |
| 6 | `writes-result-annotation` | 普通 reconcile | svc 注解 `k8s.bfenetworks.com/productpool-result` 含池名 |
| 7 | `diff-converge-removes-pool` | 先 2 端口创建 2 池，再改 1 端口 | 旧池被删除，剩 1 池 |
| 8 | `adds-finalizer` | 首次 reconcile | svc 带 `k8s.bfenetworks.com/delete-protection` |
| 9 | `delete-path` | 创建池后删除 Service | 池删除、Service 移除、result cm 清理 |
| 10 | `result-cm` | 成功 reconcile | result cm `data.result=Succ`、labels 正确 |
| 11 | `result-cm-fail-path` | ALB 不可达 | result cm `data.result != Succ` |

## 四、代码位置索引

| 位置 | 用途 |
|---|---|
| `internal/alb/albProvider.go:95-145` | EnsureProductPool |
| `internal/alb/albProvider.go:72-93` | getInstances |
| `internal/alb/albProvider.go:164-169` | poolName |
| `internal/controllers/loadbalancer/service_controller.go:138-187` | ensurePool / ensureProductPool |
| `internal/controllers/loadbalancer/service_controller.go:189-207` | deletePool |
| `internal/controllers/loadbalancer/service_controller.go:209-219` | addFinalizer |
| `internal/controllers/loadbalancer/service_controller.go:300-346` | handleResultConfigmap |
