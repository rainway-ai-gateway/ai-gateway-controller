# Service YAML 接口定义（ai-gateway-controller）

本文档描述 ai-gateway-controller 对 Kubernetes `Service` 资源的格式要求，即控制器会消费的 Service YAML 输入契约。

逻辑处理代码：`internal/controllers/loadbalancer/service_controller.go`

## 1. 概述

`service-reconciler` 控制器监听 `Service` 与 `Endpoints` 资源。当 Service 满足过滤条件后，控制器会为 Service 的每个「具名端口」向外部负载均衡（ALB）创建/更新一个「产品实例池（product pool）」。

> 说明：本文档只描述 Service 的输入格式要求。

## 2. 生效前提

Service 要被控制器接管，必须同时满足以下条件：

| 条件 | 说明 |
| --- | --- |
| 功能开关 | 控制器启动参数 `--enable-rs-pool` 为 `true`（默认 `true`） |
| 命名空间 | Service 的 `metadata.namespace` 属于控制器监听的命名空间列表（启动参数 `--namespace`，默认 `*` 即所有命名空间） |
| 产品标签 | Service 必须携带标签 `bfe-product`，且其值必须与控制器启动参数 `--bfe-product-name` 一致（默认值 `AI_product`） |

以上条件由 `filter.NamespaceFilter()` 与 `filter.LabelFilter()` 完成，逻辑见 `internal/controllers/filter/label.go:42`（`isYingfeiRSService`）。

## 3. 字段要求

### 3.1 `metadata.labels`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `bfe-product` | 是 | 产品名，字符串。**必须**与控制器启动参数 `--bfe-product-name` 完全一致（默认 `AI_product`）。不满足则 Service 被忽略 |

其余标签无要求，可自由添加。

### 3.2 `metadata.namespace`

必须位于控制器监听的命名空间列表内。默认监听所有命名空间（`*`）。

### 3.3 `spec.selector`

标准的 Pod 选择器。控制器并不直接使用 `spec.selector`，而是读取与该 Service 同名的 `Endpoints` 对象（由 Kubernetes 根据 `spec.selector` 自动生成）。因此：

- Service 必须设置有效的 `spec.selector`，使 Kubernetes 能生成同名 `Endpoints`；
- 若 Service 没有匹配到任何 Ready Pod，则对应的 `Endpoints` 中不含实例，控制器会记录池名但跳过 ALB 的创建/更新操作。

### 3.4 `spec.ports`

| 字段 | 必填 | 取值说明 |
| --- | --- | --- |
| `ports[].name` | 是 | 端口名，**不能为空**。每个非空 `name` 对应一个产品实例池；`name` 为空（或不填）的端口会被跳过，不生成池 |
| `ports[].port` | 是 | Service 端口（对外暴露端口），标准 K8s 语义 |
| `ports[].targetPort` | 是 | 目标端口，用于匹配后端 Pod 的容器端口 |

`spec.type`、`clusterIP` 等字段不影响控制器逻辑，按标准 K8s Service 填写即可。

### 3.5 对应 `Endpoints`（自动生成）

控制器按 Service 的 `namespace` + `name` 读取同名 `Endpoints`，并从其中提取后端实例：

- 遍历 `Endpoints.subsets[]`，仅取 `subset.ports[]` 中 `name` 等于 Service `spec.ports[].name` 的端口；
- 取该 subset 的 `addresses[].ip` 作为实例 IP（**不使用** `notReadyAddresses`）；
- 实例端口取 `subset.ports[].port`（即 targetPort 的实际端口号）；
- 每个实例固定 `weight=1`，端口映射固定为 `{"Default": <port>}`。

### 3.6 产品实例池命名规则（内部行为，供理解）

为每个具名端口生成一个产品实例池，命名规则：

```
<product>.k8s_<namespace>_<service-name>_<port-name>[_<cluster-name>]
```

- `<product>`：优先取 Service 标签 `bfe-product` 的值，否则取 `--bfe-product-name`；
- `<cluster-name>`：控制器启动参数 `--k8s-cluster-name`（默认 `testk8s`），为空时不追加该段。

其中 `<port-name>` 即 `spec.ports[].name`，因此**端口名是池名的一部分，一旦创建后不宜随意改名**（改名会导致旧的池残留、新的池被创建）。

## 4. 控制器写入的附加字段（非用户填写）

以下字段由控制器在 Service 上自动维护，用户无需手动填写：

| 字段 | 说明 |
| --- | --- |
| `metadata.finalizers` | 添加 `k8s.bfenetworks.com/delete-protection`，用于在删除 Service 时清理对应产品池 |
| `metadata.annotations["k8s.bfenetworks.com/productpool-result"]` | 记录本次创建/更新的产品池列表（JSON 数组，元素为 `{Product, Poolname}`），用于后续比对与删除 |

## 5. 使用示例

### 5.1 最小示例（单个端口）

```yaml
apiVersion: v1
kind: Service
metadata:
  name: whoami-airs
  namespace: open-bfe-demo
  labels:
    bfe-product: AI_product          # 必填，且须与 --bfe-product-name 一致
spec:
  ports:
    - name: http                     # 端口名必填，对应一个产品池
      port: 8080
      targetPort: 80
  selector:
    app.kubernetes.io/name: whoami-airs
```

### 5.2 多端口示例

每个非空 `name` 都会生成一个独立产品池：

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-app
  namespace: open-bfe-demo
  labels:
    bfe-product: AI_product
spec:
  ports:
    - name: http                     # 池: AI_product.k8s_open-bfe-demo_my-app_http_testk8s
      port: 8080
      targetPort: 80
    - name: grpc                     # 池: AI_product.k8s_open-bfe-demo_my-app_grpc_testk8s
      port: 9090
      targetPort: 9090
  selector:
    app: my-app
```

### 5.3 完整示例（Deployment + Service）

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: whoami-airs
  namespace: open-bfe-demo
  labels:
    app.kubernetes.io/name: whoami-airs
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: whoami-airs
  template:
    metadata:
      labels:
        app.kubernetes.io/name: whoami-airs
    spec:
      containers:
        - name: whoami
          image: traefik/whoami
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: whoami-airs
  namespace: open-bfe-demo
  labels:
    bfe-product: AI_product
spec:
  ports:
    - name: http
      port: 8080
      targetPort: 80
  selector:
    app.kubernetes.io/name: whoami-airs
```

## 6. 注意事项

1. **`bfe-product` 标签缺失或值不匹配**，Service 会被完全忽略，不会产生任何动作。
2. **端口必须有 `name`**：`spec.ports[].name` 为空则跳过该端口。
3. **必须有后端实例**：当 Service 没有 Ready Pod（同名 Endpoints 中无 `addresses`）时，该端口只记录池名、不调用 ALB 接口。
4. **删除 Service**：控制器通过 finalizer `k8s.bfenetworks.com/delete-protection` 拦截删除，先删除对应产品池，再移除 finalizer。
5. 服务端口名（`ports[].name`）参与池命名，生产环境应保持稳定。
