# ai-gateway-controller 总体设计文档

## 1. 项目定位

`ai-gateway-controller` 是一个基于 [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) 的 Kubernetes 控制器，负责监听集群内的普通 AI `Service`（及其同名 `Endpoints`），将符合条件的后端实例经 InnerAPI 全量上报到 ai-gateway-api 的「K8s 实例池（`k8s_pools`）」，由 api 侧 fan-out 同步到引用该池的 Provider 镜像与 cluster 派生池，实现大模型推理服务流量的无缝接入与管理。

## 2. 整体架构

```
┌──────────────────────────── Kubernetes 集群 ────────────────────────────┐
│                                                                          │
│   ┌──────────────┐                          ┌──────────────┐             │
│   │   Service    │                          │  Endpoints   │             │
│   │  (AI 服务)    │                          │  (后端实例)   │             │
│   └──────┬───────┘                          └──────┬───────┘             │
│          │ watch                                  │ watch                 │
└──────────┼────────────────────────────────────────┼─────────────────────┘
           │                                        │
           ▼                                        ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    ai-gateway-controller (controller-runtime)             │
│                                                                          │
│  ┌────────────────────────────────────────┐                              │
│  │           ServiceReconciler            │                              │
│  │      (Service/Endpoints → k8s 池)       │                              │
│  └───────────────────┬────────────────────┘                              │
│                      │                                                    │
│                      ▼                                                    │
│  ┌──────────────────────────────┐   ┌──────────────────────────┐          │
│  │  AlbProvider / InnerApiClient │   │  (无状态, 无本地缓存)      │          │
│  └───────────────┬──────────────┘   └──────────────────────────┘          │
└──────────────────┼───────────────────────────────────────────────────────┘
                   │ HTTP (inner-api/v1)
                   ▼
    ┌──────────────────────────────────────────┐
    │           ai-gateway-api (BFE/ALB)        │
    │  k8s_pools → provider 镜像 → cluster 派生池 │
    └──────────────────────────────────────────┘
```

## 3. 模块划分

| 模块 | 目录/文件 | 职责 |
| --- | --- | --- |
| 启动入口 | `cmd/ai-gateway-controller/` | 解析命令行参数、初始化 Scheme、启动控制器管理器 |
| 控制器装配 | `internal/controllers/start.go` | 创建 manager，按开关装配控制器与健康检查 |
| Service 发现 | `internal/controllers/loadbalancer/service_controller.go` | 监听 Service/Endpoints，向 k8s_pools 上报实例快照 |
| 过滤器 | `internal/controllers/filter/` | 命名空间过滤、Service 标签过滤 |
| ALB 客户端 | `internal/alb/albProvider.go`、`internal/alb/innerApiClient.go` | 封装对 ai-gateway-api InnerAPI `/k8s_pools` 的调用 |
| InnerAPI 数据结构 | `internal/alb/apis/k8s_pool/k8s_pool.go` | 镜像 k8s_pools 的 `Instance`/`PoolEntry`/`PoolListEntry` |
| 配置 | `internal/option/`、`internal/option/externalLB/` | 全局运行参数（含 externalLB 连接参数） |
| 健康检查 | `internal/controllers/readiness/` | readiness/liveness 探针处理（`ready.go` 状态机） |
| 日志工具 | `internal/util/util.go` | 命名日志器 `K8sCLogger` / `ApiLogger` / `HdlLogger` |

## 4. 核心数据流

1. **Service 数据流**：
   `Service/Endpoints 事件` → 过滤器（命名空间 + `ai-gateway-api-name` 标签）→ `ServiceReconciler.Reconcile` → `AlbProvider.EnsureK8sPool` → `InnerApiClient` `PUT /inner-api/v1/k8s_pools/{name}/instances`；端口改名/删除时按池名差集 `DELETE` 多余池。

## 5. 关键开关与配置

| 配置项 | 启动参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| API 名称 | `--ai-gateway-api-name` | `""`（必填） | 被发现的 Service 需携带同名标签且值一致；`*` 匹配任意；为空时启动报错 |
| 集群名 | `--k8s-cluster-name` | `testk8s` | 参与 k8s 池命名 |
| 监听命名空间 | `--namespace` / `-n` | 全部 | 逗号分隔；空或 `*` 表示全部 |
| ALB 地址 | `--ai-gateway-api-addr` | 内置测试默认值 | ai-gateway-api 的 HTTP 地址，部署时覆盖 |
| ALB Token | `--ai-gateway-api-token` | 内置测试默认值 | 访问 ai-gateway-api 的鉴权 Token（需 `k8s_pools` 域 Update/Delete 权限），部署时覆盖 |
| 删除保护 | `--force-rm-finalizer` | `false` | 删除 k8s 池失败时仍移除 finalizer |
| 跳过空删除 | `--skip-nil-svc-delete` | `true` | Service 已消失（NotFound）的删除事件直接跳过 |
| 出错重试间隔 | `--retry-interval-unit-sec` | `-1`（关闭） | 处理出错后 `RequeueAfter` 秒数，`>0` 才生效 |
| 指标地址 | `--metrics-bind-address` | `:9080` | metrics 监听地址 |
| 探针地址 | `--health-probe-bind-address` | `:9081` | liveness/readiness 监听地址 |
| pprof | `--pprof-address` | `""`（关闭） | 开启 pprof 时填写，如 `:6060` |

## 6. K8s 实例池命名约定

池名由控制器统一生成，格式：

```
k8s_<namespace>_<resource-name>_<port-name>[_<cluster-name>]
```

- 普通 Service：`port-name` 为 `spec.ports[].name`，每个具名端口一个池；端口名为空则跳过；
- `<cluster-name>` 为空时不追加该段；
- 结果超过 64 字符时折叠为「前缀 + 8 位哈希」：`pool[:55] + "_" + sha256(<ns>|<svc>|<port>|<cluster>)[:8]`，并校验合法性（1-64 字符，仅字母/数字/`_`/`-`/`.`，首尾不得为 `.`/`-`/`_`）。

Provider 侧以 `k8s_pool_name` 引用同名池。

关联的 Service annotation：
- `k8s.bfenetworks.com/k8spool-result`：记录该 Service 当前管理的池名列表（JSON 字符串数组），用于端口改名/删除时差集清理；
- `k8s.bfenetworks.com/delete-protection`：finalizer，保证 Service 删除前先清理 k8s 池。

## 7. InnerAPI 交互端点

控制器通过 `InnerApiClient` 访问 ai-gateway-api 的 `/inner-api/v1/k8s_pools`：

| 端点 | Method | 用途 |
| --- | --- | --- |
| `/k8s_pools/{name}/instances` | PUT | 全量替换某池实例快照（幂等 upsert，body 为裸 JSON 数组，空数组表达零实例） |
| `/k8s_pools/{name}` | DELETE | 删除池（无引用保护，404 幂等） |
| `/k8s_pools/{name}` | GET | 读取单个池（状态查询/诊断） |
| `/k8s_pools` | GET | 读取全部池列表（对账/诊断） |

## 8. 可观测性与运维

- 日志：基于 controller-runtime zap 日志，命名日志器 `k8sc` / `api` / `handling`，`--zap-log-level` 控制级别。
- 健康检查：`/healthz`（liveness）、`/readyz`（readiness），默认绑定 `:9081`；启动后经 `--unready-duration`（默认 30s）保持 unready。
- 指标：metrics 默认绑定 `:9080`。
- 事件：ServiceReconciler 通过 EventRecorder 在 Service 上记录成功/失败事件。
- 结果回写：同名 `<service-name>.result` ConfigMap 记录每次处理结果与时间戳。

## 9. 相关设计文档

- [K8s Service 发现设计](service-discovery-design.md)
