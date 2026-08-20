# ai-gateway-controller 总体设计文档

## 1. 项目定位

`ai-gateway-controller` 是一个基于 [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) 的 Kubernetes 控制器，负责持续监听集群内的 AI 推理相关资源，并将符合条件的资源自动注册到外部负载均衡（BFE/ALB）的「产品实例池（product instance pool）」中，实现大模型推理服务流量的无缝接入与管理。

控制器支持两类资源的发现与同步：

1. **普通 AI K8s Service 发现**：监听携带 `bfe-product` 标签的普通 `Service`，将后端实例注册为 L7 产品实例池。
2. **InferencePool 发现**：监听 `inference.networking.k8s.io/v1` 的 `InferencePool`，结合后端推理 Pod 与 EPP（Endpoint Picker）Service，注册为 EPP 类型产品实例池。

## 2. 整体架构

```
┌──────────────────────────── Kubernetes 集群 ────────────────────────────┐
│                                                                          │
│   ┌──────────────┐    ┌──────────────────┐    ┌──────────────┐           │
│   │   Service    │    │   InferencePool   │    │     Pod      │           │
│   │  (普通L7服务) │    │ (inference.net)   │    │ (推理后端)    │           │
│   └──────┬───────┘    └────────┬─────────┘    └──────┬───────┘           │
│          │ watch               │ watch                │ watch             │
└──────────┼─────────────────────┼──────────────────────┼───────────────────┘
           │                     │                      │
           ▼                     ▼                      ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                    ai-gateway-controller (controller-runtime)             │
│                                                                          │
│  ┌────────────────┐   ┌─────────────────────┐   ┌────────────────┐       │
│  │ ServiceReconciler│  │ InferencePoolReconciler│ │  PodReconciler  │       │
│  │  (rs pool)      │   │  (epp pool)           │  │  (端点同步)      │       │
│  └───────┬────────┘   └──────────┬───────────┘   └───────┬────────┘       │
│          │                       │                        │               │
│          │            ┌──────────▼───────────┐            │               │
│          │            │  InferPoolDict/Store │◄───────────┘               │
│          │            │  (内存数据存储)       │                            │
│          │            └──────────┬───────────┘                            │
│          │                       │                                        │
│          ▼                       ▼                                        │
│  ┌─────────────────────────────────────────────────────────────┐          │
│  │                    AlbProvider / OpenApiClient               │          │
│  └───────────────────────────────┬─────────────────────────────┘          │
└──────────────────────────────────┼─────────────────────────────────────────┘
                                   │ HTTP (open-api/v1)
                                   ▼
                    ┌─────────────────────────────┐
                    │  ai-gateway-api (BFE/ALB)   │
                    │  产品实例池 (product pool)    │
                    └─────────────────────────────┘
```

## 3. 模块划分

| 模块 | 目录/文件 | 职责 |
| --- | --- | --- |
| 启动入口 | `cmd/ai-gateway-controller/` | 解析命令行参数、初始化 Scheme、启动控制器管理器 |
| 控制器装配 | `internal/controllers/start.go` | 创建 manager，按开关装配各控制器与健康检查 |
| Service 发现 | `internal/controllers/loadbalancer/service_controller.go` | 监听 Service/Endpoints，同步普通 L7 服务到产品池 |
| InferencePool 发现 | `internal/controllers/loadbalancer/inferencepool_reconciler.go` | 监听 InferencePool，同步 EPP 产品池 |
| Pod 同步 | `internal/controllers/loadbalancer/pod_reconciler.go` | 监听 Pod 变化，增量更新推理端点并触发 InferencePool 重调 |
| 过滤器 | `internal/controllers/filter/` | 命名空间过滤、Service 标签过滤、InferencePool 过滤 |
| 数据存储 | `internal/datastore/datastore.go` | 维护 InferencePool → 端点 的内存映射 |
| 数据层 | `internal/datalayer/` | 定义 `EndpointPool`/`Endpoint` 等核心数据结构 |
| 工具 | `internal/poolutil/poolutil.go` | InferencePool 与 EndpointPool 互转、Pod 就绪判断 |
| ALB 客户端 | `internal/alb/` | 封装对 ai-gateway-api 的 HTTP 调用（产品池 CRUD） |
| 配置 | `internal/option/` | 全局运行参数（含 externalLB 连接参数） |
| 健康检查 | `internal/controllers/readiness/` | readiness/liveness 探针处理 |

## 4. 核心数据流

1. **Service 数据流**：
   `Service/Endpoints 事件` → 过滤器（命名空间 + `bfe-product` 标签）→ `ServiceReconciler.Reconcile` → `EnsureProductPool` → `OpenApiClient` 创建/更新/删除产品池。

2. **InferencePool 数据流**：
   `InferencePool 事件` → 过滤器（命名空间）→ `InferencePoolReconciler.Reconcile` → 更新 `InferPoolStore` → 拉取匹配的 Ready Pod 端点 → `EnsureInferPoolProductPool` → `OpenApiClient` 创建/更新 EPP 产品池。

3. **Pod 增量数据流**：
   `Pod 事件` → `PodReconciler` 按标签匹配 → 增量更新 `InferPoolStore` 端点 → 触发对应 `InferencePoolReconciler` 重新同步。

## 5. 关键开关与配置

| 配置项 | 启动参数 | 默认值 | 说明 |
| --- | --- | --- | --- |
| 服务发现开关 | `--enable-rs-pool` | `true` | 是否启用普通 Service 发现 |
| 推理池发现开关 | `--enable-inference-pool` | `true` | 是否启用 InferencePool 发现 |
| 产品名 | `--bfe-product-name` | `AI_product` | 产品池归属产品，Service 标签需与之匹配 |
| 集群名 | `--k8s-cluster-name` | `testk8s` | 参与产品池命名 |
| 监听命名空间 | `--namespace` / `-n` | `*` | 逗号分隔，`*` 表示全部 |
| ALB 地址 | `--ai-gateway-api-addr` | — | ai-gateway-api 的 HTTP 地址 |
| ALB Token | `--ai-gateway-api-token` | — | 访问 ai-gateway-api 的鉴权 Token |

## 6. 产品实例池命名约定

产品池名由控制器统一生成，格式：

```
<product>.k8s_<namespace>_<resource-name>_<port-name>[_<cluster-name>]
```

- 普通 Service：`port-name` 为 `spec.ports[].name`，每个端口一个池。
- InferencePool：`port-name` 固定为 `inferp`，每个 InferencePool 一个池。

## 7. 可观测性与运维

- 日志：基于 controller-runtime zap 日志，`--zap-log-level` 控制级别。
- 健康检查：`/healthz`（liveness）、`/readyz`（readiness），默认绑定 `:9081`。
- 指标：metrics 默认绑定 `:9080`。
- 事件：ServiceReconciler 通过 EventRecorder 在 Service 上记录成功/失败事件。

## 8. 相关设计文档

- [K8s Service 发现设计](service-discovery-design.md)
- [K8s InferencePool 发现设计](inferencepool-discovery-design.md)
