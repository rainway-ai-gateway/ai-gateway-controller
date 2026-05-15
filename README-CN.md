# ai-gateway-controller

中文 | [English](./README.md)

`ai-gateway-controller` 是一个 Kubernetes 控制器，用于实现如下功能:
- Inference pool 发现，并自动同步到 BFE
- 普通AI k8s Service (未依赖inference pool)发现，并自动同步到 BFE

该控制器持续监控k8s集群中的上述资源资源变化，自动将符合条件的服务注册到 BFE 配置中，实现大模型推理服务流量的无缝接入与管理。


## 快速开始

### 前提条件

- Kubernetes 集群 (v1.18+)
- kubectl 配置正确
- [Yingfei ai gateway api](https://github.com/yf-networks/ai-gateway-api)已部署并可访问

### 部署控制器

```bash
# 克隆仓库
git clone https://github.com/yf-networks/ai-gateway-controller.git
cd ai-gateway-controller

# 应用部署清单
kubectl apply -f ./examples/deploy/yf-ai-gateway-controller-sa.yaml
kubectl apply -f ./examples/deploy/yf-ai-gateway-controller.yaml
```

### 验证部署

```bash
kubectl get deployment bfe-ai-gateway-controller
kubectl get pods 

```

## 配置说明

### 控制器配置
请参考 [./examples/deploy/yf-ai-gateway-controller.yaml](./examples/deploy/yf-ai-gateway-controller.yaml).

注意点：
- 可以根据实际场景，修改image的源
- 请根据ai gateway api的地址，修改ai-gateway-api-addr
- 请根据ai gateway api的token配置，修改ai-gateway-api-token
  - 在ai gateway api上，可通过如下方式获得Token `User Manage / Token`

### Inference Pool发现
发布
```
$ kubectl apply -f ./examples/inferencepool/inference-pool.yaml
```
具体内容请参考 [./examples/inferencepool/inference-pool.yaml](./examples/inferencepool/inference-pool.yaml).


### 普通AI k8s Service发现
发布
```
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml
```
具体内容请参考 [./examples/l7service/whoami_airs.yaml](./examples/l7service/whoami_airs.yaml).

注意点：
- labels中增加 `bfe-product`, 其值应为 `AI_product`
- ports中的port name必须指定

## 监控与运维

### 健康检查

控制器提供标准的 Kubernetes 健康检查端点：

- **Readiness 检查**：`GET /ready` - 检查控制器是否准备好处理请求
- **Liveness 检查**：`GET /healthz` - 检查控制器是否健康运行


## 构建项目

### 构建要求

- Go 1.24+
- Docker

### 构建命令
```bash
# 构建二进制
make build

# 构建 Docker 镜像 (当前架构)
make docker

# 推送 docker 镜像（跨平台：linux/amd64 & linux/arm64）
make docker-push REGISTRY=ghcr.io/your-org
```

注意:
- 为了顺利构建，可能需要配置 GOPROXY. eg:

```
GO111MODULE=on GOPROXY=https://goproxy.cn,direct go mod download
```

## 具体使用例子

### 前置条件

#### 配置 examples/ai-gateway-controller-endpoints.yaml
- 服务地址: `http://172.18.1.244:8183`
- Token为: `Token xCFZgmV02dzD3lWTlRvN'`
  - 需要预先在控制面[dashboard](https://github.com/yf-networks/ai-gateway-web)上创建`token`
- 监听的k8s namespace: `open-bfe-demo`
- k8s的集群名为: `szyf`
- 镜像地址. 请参考 [ai-gateway-controller image](https://github.com/yf-networks/ai-gateway-controller/pkgs/container/ai-gateway-controller)

注意:
- 请根据您的实际环境，修改上述配置的值。
- 启动 ai gateway api 后，已自动创建产品线 `AI_product`

### 部署ai-gateway-controller

```
# 部署ai-gateway-controller
$kubectl apply -f ./examples/deploy/yf-ai-gateway-controller-sa.yaml
$kubectl apply -f ./examples/deploy/yf-ai-gateway-controller.yaml

# 查看部署状态
$kubectl get pods
NAME                                     READY   STATUS                   RESTARTS       AGE
bfe-ai-gateway-controller-7bb75b9b54-sl27s     1/1     Running                  0                64m

# 查看日志
$kubectl logs bfe-ai-gateway-controller-7bb75b9b54-sl27s
...
```

### 部署Inference Pool AI推理服务
具体内容请参考 [./examples/inferencepool/inference-pool.yaml](./examples/inferencepool/inference-pool.yaml).

注意点：
- 不需要修改已有的Inference Pool

```
# 发布Inference pool
$ kubectl apply -f ./examples/inferencepool/epp-rbac.yaml
$ kubectl apply -f ./examples/inferencepool/service-accounts.yaml
$ kubectl apply -f ./examples/inferencepool/vllm-sim.yaml
$ kubectl apply -f ./examples/inferencepool/epp-deployments.yaml
$ kubectl apply -f ./examples/inferencepool/epp-services.yaml
$ kubectl apply -f ./examples/inferencepool/inference-pool.yaml
 
# 删除Inference pool
$ kubectl delete -f ./examples/inferencepool/inference-pool.yaml
$ kubectl delete -f ./examples/inferencepool/epp-services.yaml
$ kubectl delete -f ./examples/inferencepool/epp-deployments.yaml
$ kubectl delete -f ./examples/inferencepool/vllm-sim.yaml
$ kubectl delete -f ./examples/inferencepool/service-accounts.yaml
$ kubectl delete -f ./examples/inferencepool/epp-rbac.yaml
```

### 部署普通AI推理服务
具体内容请参考 [./examples/l7service/whoami_airs.yaml](./examples/l7service/whoami_airs.yaml).

注意点：
- labels中增加 `bfe-product`, 其值应为 `AI_product`
- ports中的port name必须指定

```
# 发布普通AI推理服务
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml

# 删除普通AI推理服务
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml
```

