# ai-gateway-controller 命令行使用文档

本文档根据代码实现整理 `ai-gateway-controller` 的全部命令行参数及其用法。

代码依据：
- `cmd/ai-gateway-controller/main.go`
- `cmd/ai-gateway-controller/flags.go`
- `internal/option/options.go`
- `internal/option/externalLB/options.go`

## 1. 概述

`ai-gateway-controller` 是一个 Kubernetes 控制器（基于 controller-runtime），以单进程方式运行，通过命令行参数控制其行为。运行方式有两种：

1. 直接在 Kubernetes 集群内运行二进制（通过 Deployment 部署）；
2. 本地调试运行（需具备集群访问权限，读取默认 kubeconfig）。

构建方式：

```bash
make build
# 产物: output/ai-gateway-controller
```

## 2. 基本用法

```bash
# 显示帮助
ai-gateway-controller -h

# 显示版本信息
ai-gateway-controller -version

# 最小启动（使用默认参数）
ai-gateway-controller
```

## 3. 参数总览

### 3.1 帮助 / 版本

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-help` / `-h` | bool | `false` | 打印全部参数默认值并退出 |
| `-version` / `-v` | bool | `false` | 打印版本、Go 版本、git commit 并退出 |

### 3.2 功能开关

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-enable-rs-pool` | bool | `true` | 是否启用普通 Service（L7 池）发现 |
| `-enable-inference-pool` | bool | `true` | 是否启用 InferencePool（EPP 池）发现 |

### 3.3 ALB（ai-gateway-api）连接参数

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-ai-gateway-api-addr` | string | `http://172.18.1.200:30001` | ai-gateway-api 的 HTTP 地址（必填，需按实际环境修改） |
| `-ai-gateway-api-token` | string | `Token f740c3040fed4bd7e97c` | 访问 ai-gateway-api 的鉴权 Token（必填，需按实际环境修改） |

> 校验规则（`internal/option/externalLB/options.go`）：`ai-gateway-api-addr` 与 `ai-gateway-api-token` 均不能为空。

### 3.4 产品 / 集群参数

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-bfe-product-name` | string | `AI_product` | 产品名。产品池归属该产品；普通 Service 的 `bfe-product` 标签须与此一致；启用 InferencePool 时不可为空 |
| `-k8s-cluster-name` | string | `testk8s` | 集群名，参与产品池命名；不可为空 |

### 3.5 命名空间过滤

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-namespace` / `-n` | string | `*` | 监听命名空间列表，逗号分隔；`*` 或空表示监听全部命名空间 |

### 3.6 服务发现相关

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-retry-interval-unit-sec` | int | `-1` | 处理出错时的重试间隔（秒）。`>0` 时按该间隔延迟重入队；`<=0` 时不设置显式延迟（由 controller-runtime 自行退避重试） |
| `-force-rm-finalizer` | bool | `false` | 即使删除产品池失败也强制移除 Service 上的 finalizer |
| `-skip-nil-svc-delete` | bool | `true` | 当 Service 已被删除（Get 返回 NotFound）时跳过删除处理（依赖 finalizer 保证清理） |

### 3.7 监控 / 健康检查 / 调试

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-metrics-bind-address` | string | `:9080` | metrics 端点监听地址 |
| `-health-probe-bind-address` | string | `:9081` | 健康检查端点监听地址 |
| `-readiness-endpoint-name` | string | `/readyz` | readiness 探针路径 |
| `-liveness-endpoint-name` | string | `/healthz` | liveness 探针路径 |
| `-unready-duration` | int | `30` | 启动后保持 unready 的时长（秒），须 `>0` |
| `-pprof-address` | string | `""` | 启用 pprof 的监听地址（如 `:6060`）；为空则不启用 |
| `-reconcile-rate` | int | `10` | 处理 reconcile 请求的速率限制（每秒），须 `>0` |
| `-reconcile-bucket` | int | `100` | reconcile 限流器的令牌桶大小，须 `>0` |

### 3.8 日志参数（由 controller-runtime zap 提供）

通过 `zap.Options.BindFlags` 注册，常见参数：

| 参数 | 说明 |
| --- | --- |
| `-zap-log-level` | 日志级别，如 `debug` / `info` / `error` |
| `-zap-devel` | 开发模式日志（更易读的格式） |
| `-zap-encoder` | 日志编码器，如 `console` / `json` |
| `-zap-stacktrace-level` | 打印堆栈的最低级别 |
| `-zap-time-encoding` | 时间格式编码 |

## 4. 使用示例

### 4.1 直接运行二进制（完整参数示例）

```bash
./ai-gateway-controller \
  -ai-gateway-api-addr=http://172.18.1.196:8183 \
  -ai-gateway-api-token='Token 4M_qRi1xYfjGtFiuHWrr' \
  -k8s-cluster-name=szyf \
  -bfe-product-name=AI_product \
  -namespace=open-bfe-demo \
  -enable-rs-pool=true \
  -enable-inference-pool=true \
  -zap-log-level=debug \
  -zap-encoder=console
```

### 4.2 仅启用 Service 发现（关闭 InferencePool 发现）

```bash
./ai-gateway-controller \
  -enable-rs-pool=true \
  -enable-inference-pool=false \
  -ai-gateway-api-addr=http://172.18.1.196:8183 \
  -ai-gateway-api-token='Token xxx' \
  -k8s-cluster-name=szyf
```

### 4.3 监听多个命名空间

```bash
./ai-gateway-controller \
  -namespace=ns-a,ns-b,ns-c \
  -ai-gateway-api-addr=http://172.18.1.196:8183 \
  -ai-gateway-api-token='Token xxx'
```

### 4.4 Kubernetes Deployment 部署

以 `examples/deploy/yf-ai-gateway-controller.yaml` 为例，参数通过容器的 `args` 传入：

```yaml
containers:
  - name: bfe-ai-gateway-controller
    image: ghcr.io/bfenetworks/ai-gateway-controller:x86_64-latest
    command: [ "/ai-gateway-controller" ]
    args:
      - '-ai-gateway-api-addr=http://172.18.1.196:8183'
      - '-ai-gateway-api-token=Token 4M_qRi1xYfjGtFiuHWrr'
      - '-k8s-cluster-name=szyf'
      - '-namespace=open-bfe-demo'
      - '-zap-log-level=debug'
```

## 5. 注意事项

1. **必填参数**：`-ai-gateway-api-addr`、`-ai-gateway-api-token`、`-k8s-cluster-name` 不可为空；启用 `-enable-inference-pool` 时 `-bfe-product-name` 不可为空（`internal/option/options.go` 的 `SetOptions` 校验）。
2. **默认 ALB 地址/Token** 为代码内置的占位值，实际部署时必须覆盖。
3. **`-version` / `-help`** 会在参数解析后立即退出，不启动控制器。
4. **服务发现**依赖 `-bfe-product-name` 与 Service 的 `bfe-product` 标签匹配，详见《K8s Service 发现设计文档》。
5. **RBAC**：默认部署使用 `cluster-admin`（见 `examples/deploy/yf-ai-gateway-controller-sa.yaml`），生产环境可按需收敛权限。
