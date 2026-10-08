# Service YAML 接口定义（ai-gateway-controller）

本文档描述 ai-gateway-controller 对 Kubernetes `Service` 资源的格式要求，即控制器会消费的 Service YAML 输入契约。

逻辑处理代码：`internal/controllers/loadbalancer/service_controller.go`

## 1. 概述

`ai-gateway-controlle` 控制器监听 `Service` 与 `Endpoints` 资源。当 Service 满足过滤条件后，控制器会为 Service 的每个「具名端口」，经 InnerAPI 全量上报一个 K8s 实例池（`k8s_pools`）的实例快照；引用该池的 Provider 由 ai-gateway-api 侧 fan-out 同步。

> 说明：本文档只描述 Service 的输入格式要求。

## 2. 生效前提

Service 要被控制器接管，必须同时满足以下条件：

| 条件 | 说明 |
| --- | --- |
| 命名空间 | Service 的 `metadata.namespace` 属于控制器监听的命名空间列表（启动参数 `--namespace`，默认 `*` 即所有命名空间） |
| API 名称标签 | Service 必须携带标签 `ai-gateway-api-name`，且其值必须与控制器启动参数 `--ai-gateway-api-name` 一致；`--ai-gateway-api-name=*` 表示匹配任意带该标签的 Service |

以上条件由 `filter.NamespaceFilter()` 与 `filter.LabelFilter()` 完成，逻辑见 `internal/controllers/filter/label.go`（`isYingfeiRSService`）。

## 3. 字段要求

### 3.1 `metadata.labels`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `ai-gateway-api-name` | 是 | API 名称，字符串。**必须**与控制器启动参数 `--ai-gateway-api-name` 完全一致（或该参数为 `*`）。不满足则 Service 被忽略 |

其余标签无要求，可自由添加。

### 3.2 `metadata.namespace`

必须位于控制器监听的命名空间列表内。默认监听所有命名空间（`*`）。

### 3.3 `spec.selector`

标准的 Pod 选择器。控制器并不直接使用 `spec.selector`，而是读取与该 Service 同名的 `Endpoints` 对象（由 Kubernetes 根据 `spec.selector` 自动生成）。因此：

- Service 必须设置有效的 `spec.selector`，使 Kubernetes 能生成同名 `Endpoints`；
- 若 Service 没有匹配到任何 Ready Pod，则对应的 `Endpoints` 中不含实例，控制器会上报空池（零实例）。

### 3.4 `spec.ports`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `ports[].name` | 是 | 端口名，**不能为空**。每个非空 `name` 对应一个 K8s 实例池；`name` 为空（或不填）的端口会被跳过，不生成池 |
| `ports[].port` | 是 | Service 端口（对外暴露端口），标准 K8s 语义 |
| `ports[].targetPort` | 是 | 目标端口，用于匹配后端 Pod 的容器端口 |

`spec.type`、`clusterIP` 等字段不影响控制器逻辑，按标准 K8s Service 填写即可。

### 3.5 对应 `Endpoints`（自动生成）

控制器按 Service 的 `namespace` + `name` 读取同名 `Endpoints`，并从其中提取后端实例：

- 遍历 `Endpoints.subsets[]`，仅取 `subset.ports[]` 中 `name` 等于 Service `spec.ports[].name` 的端口；
- 取该 subset 的 `addresses[].ip` 作为实例地址 `addr`（**不使用** `notReadyAddresses`）；
- 实例端口 `port` 取 `subset.ports[].port`（即 targetPort 的实际端口号）；
- 实例按 `(addr, port)` 去重；`weight` 不显式设置，由 API 置默认 `100`。

### 3.6 K8s 实例池命名规则（内部行为，供理解）

为每个具名端口生成一个 K8s 实例池，命名规则：

```
k8s_<namespace>_<service-name>_<port-name>[_<cluster-name>]
```

- `<cluster-name>`：控制器启动参数 `--k8s-cluster-name`（默认 `testk8s`），为空时不追加该段；
- 结果超过 64 字符时折叠为可读前缀 + 8 位哈希：`pool[:55] + "_" + sha256("<ns>|<svc>|<port>|<cluster>")[:8]`；并校验合法性（1-64 字符，仅字母/数字/`_`/`-`/`.`，首尾不得为 `.`/`-`/`_`）；

其中 `<port-name>` 即 `spec.ports[].name`，因此**端口名是池名的一部分，一旦创建后不宜随意改名**（改名会导致旧的池被删除、新的池被创建）。

该池名即 Provider 侧 `instance_source=k8s_pool` 时填写的 `k8s_pool_name`，两者必须完全一致。

## 4. 控制器写入的附加字段（非用户填写）

以下字段由控制器在 Service 上自动维护，用户无需手动填写：

| 字段 | 说明 |
| --- | --- |
| `metadata.finalizers` | 添加 `k8s.bfenetworks.com/delete-protection`，用于在删除 Service 时清理对应 k8s 池 |
| `metadata.annotations["k8s.bfenetworks.com/k8spool-result"]` | 记录本次上报的 k8s 池名列表（JSON 字符串数组），用于后续比对与删除 |

## 5. 使用示例

仓库内提供了可直接套用的完整示例，其命名空间、标签与集群名与部署示例 `examples/deploy/yf-ai-gateway-controller.yaml` 保持一致：

| 文件 | 用途 |
| --- | --- |
| `examples/l7service/prerequisite.yaml` | 创建命名空间 `rainway-ai-gatway` |
| `examples/l7service/whoami_airs.yaml` | Deployment + 双端口 Service（有后端实例） |
| `examples/l7service/whoami_airs_empty.yaml` | 无匹配 Pod 的 Service（验证空池上报） |

匹配前提（与上述部署参数一一对应）：

- 控制器 `--namespace=rainway-ai-gatway`；
- 控制器 `--ai-gateway-api-name=rainway-ai-gatway-demo`；
- 控制器 `--k8s-cluster-name=szyf`。

### 5.1 完整示例（Deployment + 双端口 Service）

对应 `examples/l7service/whoami_airs.yaml`：

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whoami-airs
  namespace: rainway-ai-gatway
  labels:
    app.kubernetes.io/name: whoami-airs
    app.kubernetes.io/instance: whoami-airs
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: whoami-airs
      app.kubernetes.io/instance: whoami-airs
  template:
    metadata:
      labels:
        app.kubernetes.io/name: whoami-airs
        app.kubernetes.io/instance: whoami-airs
    spec:
      containers:
        - name: whoami
          imagePullPolicy: IfNotPresent
          image: traefik/whoami
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: whoami-airs
  namespace: rainway-ai-gatway
  labels:
    ai-gateway-api-name: rainway-ai-gatway-demo   # 必填，且须与 --ai-gateway-api-name 一致
spec:
  ports:
    - name: http0                                 # 池: k8s_rainway-ai-gatway_whoami-airs_http0_szyf
      port: 8080
      targetPort: 80
    - name: http1                                 # 池: k8s_rainway-ai-gatway_whoami-airs_http1_szyf
      port: 8081
      targetPort: 80
  selector:
    app.kubernetes.io/name: whoami-airs
```

控制器会为 `http0` / `http1` 各上报一个池，实例均为 Ready Pod 的 `<pod-ip>:80`（targetPort 的实际端口）。

### 5.2 空池示例（无匹配 Pod）

对应 `examples/l7service/whoami_airs_empty.yaml`：`spec.selector` 指向不存在的标签，同名 Endpoints 无 Ready 地址，控制器仍按端口上报 `[]`（零实例），池条目照常创建：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: whoami-airs-empty
  namespace: rainway-ai-gatway
  labels:
    ai-gateway-api-name: rainway-ai-gatway-demo
spec:
  ports:
    - name: http                                  # 池: k8s_rainway-ai-gatway_whoami-airs-empty_http_szyf（零实例）
      port: 8080
      targetPort: 80
  selector:
    app.kubernetes.io/name: whoami-airs-empty
```

### 5.3 Provider 引用（ai-gateway-api 侧，非本仓库资源）

要让流量到达后端，还需在 ai-gateway-api 侧创建引用同名池的 Provider，其 `k8s_pool_name` 必须与 §3.6 生成的池名完全一致：

```json
{
  "name": "whoami-airs-provider",
  "instance_source": "k8s_pool",
  "k8s_pool_name": "k8s_rainway-ai-gatway_whoami-airs_http0_szyf"
}
```

Provider 可先于池创建（池不存在等价于零实例）；池建立或实例变化后，ai-gateway-api 会在同一事务内刷新 Provider 的 `k8s_instance_pool` 镜像并联动 cluster 派生池。

## 6. 注意事项

1. **`ai-gateway-api-name` 标签缺失或值不匹配**，Service 会被完全忽略，不会产生任何动作。
2. **端口必须有 `name`**：`spec.ports[].name` 为空则跳过该端口。
3. **无后端实例时上报空池**：当 Service 没有 Ready Pod（同名 Endpoints 中无 `addresses`）时，该端口上报空数组（零实例），池条目仍会创建；"池不存在" 与 "池存在但零实例" 对下游等价。
4. **删除 Service**：控制器通过 finalizer `k8s.bfenetworks.com/delete-protection` 拦截删除，先删除对应 k8s 池，再移除 finalizer。
5. 服务端口名（`ports[].name`）参与池命名，生产环境应保持稳定；命名空间/服务名/端口名任一变化都会导致池名变化（旧池删除、新池创建）。
6. **命名空间与标签缺一不可**：示例中命名空间为 `rainway-ai-gatway`、标签值为 `rainway-ai-gatway-demo`，需同时落在控制器的 `--namespace` 与 `--ai-gateway-api-name` 范围内才会被接管。
