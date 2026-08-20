# InferencePool YAML 接口定义（ai-gateway-controller）

本文档描述 ai-gateway-controller 对 Kubernetes `InferencePool` 资源的格式要求，即控制器会消费的 InferencePool YAML 输入契约。

逻辑处理代码：
- `internal/controllers/loadbalancer/inferencepool_reconciler.go`
- `internal/poolutil/poolutil.go`（`InferencePoolToEndpointPool`）
- `internal/datastore/datastore.go`

## 1. 概述

`InferencePoolReconciler` 监听 `inference.networking.k8s.io/v1` 的 `InferencePool` 资源。控制器根据 InferencePool 的 `selector` 选出后端推理 Pod，并结合 `endpointPickerRef` 指向的 EPP（Endpoint Picker）Service，向外部负载均衡（ALB）创建/更新一个「EPP 类型的产品实例池」。

> 说明：本项目没有对 InferencePool 添加额外的自定义字段要求，遵循标准 `gateway-api-inference-extension` v1 的 InferencePool 定义。

## 2. 生效前提

| 条件 | 说明 |
| --- | --- |
| 功能开关 | 控制器启动参数 `--enable-inference-pool` 为 `true`（默认 `true`） |
| 命名空间 | InferencePool 的 `metadata.namespace` 属于控制器监听的命名空间列表（启动参数 `--namespace`，默认 `*`） |
| 产品名 | 控制器启动参数 `--bfe-product-name` 必须非空（默认 `AI_product`），用于生成产品池名 |

InferencePool **不要求任何标签**（`EppXLabelFilter` 恒返回 `true`，见 `internal/controllers/filter/label.go:92`），命名空间内的所有 InferencePool 都会被处理。

## 3. 字段要求

### 3.1 基本信息

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `apiVersion` | 是 | 固定为 `inference.networking.k8s.io/v1` |
| `kind` | 是 | 固定为 `InferencePool` |
| `metadata.name` | 是 | InferencePool 名称，参与池命名 |
| `metadata.namespace` | 是 | 所属命名空间，参与池命名，且必须属于监听范围 |

### 3.2 `spec.selector`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `spec.selector.matchLabels` | 是 | 用于选择后端推理 Pod 的标签集合（`key: value`），匹配逻辑为 AND（Pod 必须包含全部标签）。至少 1 个、最多 64 个 |

- 仅匹配**同命名空间**内的 Pod；
- 仅 `Ready` 的 Pod 会被纳入（`poolutil.IsPodReady`，要求 Pod 未处于删除中且 `PodReady` 条件为 `True`）；
- 实例地址取 Pod 的 `status.podIP`。

### 3.3 `spec.targetPorts`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `spec.targetPorts[].number` | 是 | 后端推理 Pod 暴露的端口号（1~65535）。每个端口被当作一个独立 endpoint（`podIP:number`） |

- 至少 1 个、最多 8 个端口；
- 端口号必须互不重复；
- 一个 Pod 会为**每个** targetPort 各生成一个 endpoint。

### 3.4 `spec.endpointPickerRef`

指向 EPP（Endpoint Picker）扩展及其关联 Service 的引用。

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `spec.endpointPickerRef.name` | 是 | EPP 的 Kubernetes Service 名称 |
| `spec.endpointPickerRef.kind` | 否 | 默认 `Service`。本项目实际按 Service 处理，建议显式填写 `Service` |
| `spec.endpointPickerRef.group` | 否 | 默认空（Core API group） |
| `spec.endpointPickerRef.port.number` | 是 | EPP Service 的**服务端口号**（非 targetPort）。控制器据此构造 EPP 的访问地址 `<service-name>.<namespace>.svc:<port>` |
| `spec.endpointPickerRef.failureMode` | 否 | 默认 `FailClose` |

### 3.5 产品实例池命名规则（内部行为，供理解）

控制器为该 InferencePool 生成一个 EPP 类型产品实例池，命名规则：

```
<product>.k8s_<namespace>_<inferencepool-name>_inferp[_<cluster-name>]
```

- `<product>`：控制器启动参数 `--bfe-product-name`；
- 固定端口名段为 `inferp`；
- `<cluster-name>`：控制器启动参数 `--k8s-cluster-name`（默认 `testk8s`），为空时不追加该段。

该产品池内部属性（由控制器生成，用户无需关心）：

- `type` = `IP`，`role` = `EPP`；
- 实例列表：由 `selector` 选中的 Ready Pod，实例 IP 为 `pod.status.podIP`，端口为 `targetPorts[].number`，权重固定为 `1`；
- EPP Server：`domain` = `<endpointPickerRef.name>.<namespace>.svc`，`port` = `endpointPickerRef.port.number`，endpoints 为 EPP 实例列表（当前实现中为空/预留）。

## 4. 使用示例

### 4.1 最小示例

```yaml
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: vllm-sim-inference-pool
  namespace: open-bfe-demo
spec:
  selector:
    matchLabels:
      app: vllm-sim-inference-pool
  endpointPickerRef:
    name: vllm-sim-endpoint-picker
    kind: Service
    port:
      number: 9002
  targetPorts:
    - number: 8200
```

### 4.2 多 targetPort 示例

```yaml
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: multi-rank-pool
  namespace: open-bfe-demo
spec:
  selector:
    matchLabels:
      app: llm-server
  endpointPickerRef:
    name: llm-endpoint-picker
    kind: Service
    port:
      number: 9002
  targetPorts:
    - number: 8200
    - number: 8201
```

### 4.3 完整示例（InferencePool + 后端 Deployment + EPP Service）

后端推理 Pod（`spec.template.metadata.labels` 必须能被 InferencePool 的 `selector.matchLabels` 匹配）：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: vllm-sim
  namespace: open-bfe-demo
  labels:
    app: vllm-sim-inference-pool
spec:
  replicas: 1
  selector:
    matchLabels:
      app: vllm-sim-inference-pool
  template:
    metadata:
      labels:
        app: vllm-sim-inference-pool
    spec:
      containers:
        - name: vllm
          image: ghcr.io/llm-d/llm-d-inference-sim:latest
          args:
            - "--port=8200"
          ports:
            - name: http
              containerPort: 8200
              protocol: TCP
---
# EPP 的 Service，endpointPickerRef 引用它
apiVersion: v1
kind: Service
metadata:
  name: vllm-sim-endpoint-picker
  namespace: open-bfe-demo
spec:
  selector:
    app: vllm-sim-endpoint-picker
  ports:
    - name: default
      protocol: TCP
      port: 9002
      targetPort: 9002
      appProtocol: http2
  type: ClusterIP
---
# InferencePool 本体
apiVersion: inference.networking.k8s.io/v1
kind: InferencePool
metadata:
  name: vllm-sim-inference-pool
  namespace: open-bfe-demo
spec:
  selector:
    matchLabels:
      app: vllm-sim-inference-pool
  endpointPickerRef:
    name: vllm-sim-endpoint-picker
    kind: Service
    port:
      number: 9002
  targetPorts:
    - number: 8200
```

## 5. 注意事项

1. **`endpointPickerRef.port.number` 必填**：控制器读取 `EndpointPickerRef.Port.Number` 构造 EPP 地址（`internal/datastore/datastore.go:165`），缺失会导致池的 EPP Server 端口为 0。
2. **`selector` 匹配的是 Pod 标签**：后端 Pod 的 `labels` 必须包含 `spec.selector.matchLabels` 中的所有键值对；跨命名空间不生效。
3. **仅 Ready Pod 生效**：未 Ready 或正在删除的 Pod 会被排除，不会作为实例写入产品池。
4. **删除 InferencePool**：控制器会删除对应的 EPP 类型产品池，并清理本地数据存储中的记录。
5. InferencePool 无 finalizer、无结果 ConfigMap，控制器的处理对用户基本透明。
