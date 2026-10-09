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

# 启动（-ai-gateway-api-name 为必填项，缺失会启动失败）
ai-gateway-controller -ai-gateway-api-name=<name>
```

## 3. 参数总览

### 3.1 帮助 / 版本

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-help` / `-h` | bool | `false` | 打印全部参数默认值并退出 |
| `-version` / `-v` | bool | `false` | 打印版本、Go 版本、git commit 并退出 |

### 3.2 ALB（ai-gateway-api）连接参数

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-ai-gateway-api-addr` | string | `http://172.18.1.200:30001` | ai-gateway-api 的 HTTP 地址（必填，需按实际环境修改） |
| `-ai-gateway-api-token` | string | 内置占位值（见代码） | 访问 ai-gateway-api 的鉴权 Token（必填，需按实际环境修改；需具备 `k8s_pools` 域 Update/Delete，可选 Read） |

> 校验规则（`internal/option/externalLB/options.go`）：`ai-gateway-api-addr` 与 `ai-gateway-api-token` 均不能为空。

### 3.3 服务发现 / 集群参数

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-ai-gateway-api-name` | string | `""` | 需要被发现的 Service 必须携带标签 `ai-gateway-api-name`，其值须与此一致；设为 `*` 表示匹配任意带该标签的 Service（必填） |
| `-k8s-cluster-name` | string | `testk8s` | 集群名，参与 k8s 池命名；不可为空 |

### 3.4 命名空间过滤

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-namespace` / `-n` | string | 空（等价 `*`） | 监听命名空间列表，逗号分隔；`*` 或空表示监听全部命名空间 |

### 3.5 服务发现相关

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-retry-interval-unit-sec` | int | `-1` | 处理出错时的重试间隔（秒）。`>0` 时按该间隔延迟重入队；`<=0` 时不设置显式延迟（由 controller-runtime 自行退避重试） |
| `-force-rm-finalizer` | bool | `false` | 即使删除 k8s 池失败也强制移除 Service 上的 finalizer |
| `-skip-nil-svc-delete` | bool | `true` | 当 Service 已被删除（Get 返回 NotFound）时跳过删除处理（依赖 finalizer 保证清理） |

### 3.6 监控 / 健康检查 / 调试

| 参数 | 类型 | 默认值 | 说明 |
| --- | --- | --- | --- |
| `-metrics-bind-address` | string | `:9080` | metrics 端点监听地址 |
| `-health-probe-bind-address` | string | `:9081` | 健康检查端点监听地址 |
| `-readiness-endpoint-name` | string | `/readyz` | readiness 探针路径 |
| `-liveness-endpoint-name` | string | `/healthz` | liveness 探针路径 |
| `-unready-duration` | int | `30` | 参数须 `>0`；当前版本仅做校验，尚未接入"启动后延迟就绪"逻辑（预留） |
| `-pprof-address` | string | `""` | 启用 pprof 的监听地址（如 `:6060`）；为空则不启用 |
| `-reconcile-rate` | int | `10` | 参数须 `>0`；当前版本仅做校验，尚未据此配置限流器（预留） |
| `-reconcile-bucket` | int | `100` | 参数须 `>0`；当前版本仅做校验，尚未据此配置限流器（预留） |

> 就绪逻辑（`internal/controllers/readiness/ready.go`）：manager 启动成功后立即 `SetReady`，`/readyz` 返回成功；进程退出时 `SetUnready`。因此当前就绪状态由进程生命周期驱动，与 `-unready-duration` 无关。

### 3.7 日志参数（由 controller-runtime zap 提供）

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
  -ai-gateway-api-addr=http://172.19.1.222:8183/ \
  -ai-gateway-api-token='Token <your-token>' \
  -k8s-cluster-name=szyf \
  -ai-gateway-api-name=rainway-ai-gatway-demo \
  -namespace=rainway-ai-gatway \
  -zap-log-level=debug \
  -zap-encoder=console
```

### 4.2 监听多个命名空间

```bash
./ai-gateway-controller \
  -namespace=ns-a,ns-b,ns-c \
  -ai-gateway-api-name=rainway-ai-gatway-demo \
  -ai-gateway-api-addr=http://172.19.1.222:8183/ \
  -ai-gateway-api-token='Token <your-token>'
```

### 4.3 Kubernetes Deployment 部署

以 `examples/deploy/yf-ai-gateway-controller.yaml` 为例，参数通过容器的 `args` 传入（Token 请替换为实际值）：

```yaml
containers:
  - name: bfe-ai-gateway-controller
    image: 172.18.1.244:5000/bfenetworks/ai-gateway-controller:latest
    command: [ "/ai-gateway-controller" ]
    args:
      - '-ai-gateway-api-name=rainway-ai-gatway-demo'
      - '-ai-gateway-api-addr=http://172.19.1.222:8183/'
      - '-ai-gateway-api-token=Token <your-token>'
      - '-k8s-cluster-name=szyf'
      - '-namespace=rainway-ai-gatway'
      - '-zap-log-level=debug'
```

## 5. 注意事项

1. **必填参数**：`-ai-gateway-api-name`、`-ai-gateway-api-addr`、`-ai-gateway-api-token`、`-k8s-cluster-name` 均不可为空（`internal/option/options.go` 的 `SetOptions` 与 `internal/option/externalLB/options.go` 的 `Check` 校验）；缺失会导致启动失败。
2. **默认 ALB 地址/Token** 为代码内置占位值，实际部署时必须覆盖；Token 需具备 `k8s_pools` 域的 Update/Delete（PUT/DELETE 池，可选 Read），否则调用返回 402。
3. **`-version` / `-help`** 会在参数解析后立即退出，不启动控制器。
4. **服务发现**依赖 `-ai-gateway-api-name` 与 Service 的 `ai-gateway-api-name` 标签匹配（`*` 表示任意），并需落在 `-namespace` 范围内；写入通道为 InnerAPI `PUT /inner-api/v1/k8s_pools/{name}/instances`。详见 `design-docs/sys-design/service-discovery-design.md` 与 `design-docs/api-define/service-yaml-api-define.md`。
5. **`-retry-interval-unit-sec` 默认关闭**（`-1`）：处理出错时返回错误交由 controller-runtime 退避重试；需显式设为 `>0` 才额外做固定间隔 `RequeueAfter`。
6. **RBAC**：默认部署使用 `cluster-admin`（见 `examples/deploy/yf-ai-gateway-controller-sa.yaml`），生产环境可按需收敛权限。
