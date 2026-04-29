# ai-gateway-controller

English | [中文](./README-CN.md)

`ai-gateway-controller` is a Kubernetes controller designed to implement the following features:
- Inference pool discovery and automatic synchronization to BFE
- Discovery of regular AI Kubernetes Services (not dependent on inference pool) and automatic synchronization to BFE

This controller continuously monitors changes to the above resources in the Kubernetes cluster, automatically registering eligible services into BFE configurations to achieve seamless integration and management of large model inference service traffic.

## Quick Start

### Prerequisites

- Kubernetes cluster (v1.18+)
- Properly configured kubectl
- [Yingfei ai gateway api](https://github.com/yf-networks/ai-gateway-api) deployed and accessible

### Deploy the Controller

```bash
# Clone the repository
git clone https://github.com/yf-networks/ai-gateway-controller.git
cd ai-gateway-controller

# Apply deployment manifests
kubectl apply -f ./examples/deploy/yf-ai-gateway-controller-sa.yaml
kubectl apply -f ./examples/deploy/yf-ai-gateway-controller.yaml
```

### Verify Deployment

```bash
kubectl get deployment bfe-ai-gateway-controller
kubectl get pods 

```

## Configuration Guide

### Controller Configuration
Please refer to [./examples/deploy/yf-ai-gateway-controller.yaml](./examples/deploy/yf-ai-gateway-controller.yaml).

Notes:
- You can modify the image source according to your actual scenario
- Please modify ai-gateway-api-addr based on the address of your ai gateway api
- Please modify ai-gateway-api-token based on the token configuration of your ai gateway api
  - On the ai gateway api, you can obtain the Token through `User Manage / Token`

### Inference Pool Discovery
Deploy
```
$ kubectl apply -f ./examples/inferencepool/inference-pool.yaml
```
Please refer to [./examples/inferencepool/inference-pool.yaml](./examples/inferencepool/inference-pool.yaml) for details.


### Regular AI Kubernetes Service Discovery
Deploy
```
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml
```
Please refer to [./examples/l7service/whoami_airs.yaml](./examples/l7service/whoami_airs.yaml) for details.

Notes:
- Add `bfe-product` to labels, its value should be `AI_product`
- The port name in ports must be specified

## Monitoring & Operations

### Health Checks

The controller provides standard Kubernetes health check endpoints:

- **Readiness Check**: `GET /ready` - Checks if the controller is ready to handle requests
- **Liveness Check**: `GET /healthz` - Checks if the controller is running healthily


## Building the Project

### Build Requirements

- Go 1.24+
- Docker

### Build Commands
```bash
# Build binary
make build

# Build Docker image (current architecture)
make docker

# Push docker image (multi-platform: linux/amd64 & linux/arm64)
make docker-push REGISTRY=ghcr.io/your-org
```

Notes:
- You may need to configure GOPROXY for smooth building. eg:

```
GO111MODULE=on GOPROXY=https://goproxy.cn,direct go mod download
```

## Usage Examples

### Prerequisites

#### Configure examples/ai-gateway-controller-endpoints.yaml
- Service address: `http://172.18.1.244:8183`
- Token: `Token xCFZgmV02dzD3lWTlRvN'`
  - You need to create a `token` in advance on the control plane [dashboard](https://github.com/yf-networks/ai-gateway-web)
- Monitored k8s namespace: `open-bfe-demo`
- k8s cluster name: `szyf`
- Image address. Please refer to [ai-gateway-controller image](https://github.com/yf-networks/ai-gateway-controller/pkgs/container/ai-gateway-controller)

Notes:
- Please modify the values of the above configurations according to your actual environment.
- After starting the ai gateway api, the product line `AI_product` has been automatically created

### Deploy ai-gateway-controller

```
# Deploy ai-gateway-controller
$kubectl apply -f ./examples/deploy/yf-ai-gateway-controller-sa.yaml
$kubectl apply -f ./examples/deploy/yf-ai-gateway-controller.yaml

# Check deployment status
$kubectl get pods
NAME                                     READY   STATUS                   RESTARTS       AGE
bfe-ai-gateway-controller-7bb75b9b54-sl27s     1/1     Running                  0                64m

# View logs
$kubectl logs bfe-ai-gateway-controller-7bb75b9b54-sl27s
...
```

### Deploy Inference Pool AI Inference Service
Please refer to [./examples/inferencepool/inference-pool.yaml](./examples/inferencepool/inference-pool.yaml) for details.

Notes:
- No need to modify existing Inference Pool

```
# Deploy Inference pool
$ kubectl apply -f ./examples/inferencepool/epp-rbac.yaml
$ kubectl apply -f ./examples/inferencepool/service-accounts.yaml
$ kubectl apply -f ./examples/inferencepool/vllm-sim.yaml
$ kubectl apply -f ./examples/inferencepool/epp-deployments.yaml
$ kubectl apply -f ./examples/inferencepool/epp-services.yaml
$ kubectl apply -f ./examples/inferencepool/inference-pool.yaml
 
# Delete Inference pool
$ kubectl delete -f ./examples/inferencepool/inference-pool.yaml
$ kubectl delete -f ./examples/inferencepool/epp-services.yaml
$ kubectl delete -f ./examples/inferencepool/epp-deployments.yaml
$ kubectl delete -f ./examples/inferencepool/vllm-sim.yaml
$ kubectl delete -f ./examples/inferencepool/service-accounts.yaml
$ kubectl delete -f ./examples/inferencepool/epp-rbac.yaml
```

### Deploy Regular AI Inference Service
Please refer to [./examples/l7service/whoami_airs.yaml](./examples/l7service/whoami_airs.yaml) for details.

Notes:
- Add `bfe-product` to labels, its value should be `AI_product`
- The port name in ports must be specified

```
# Deploy regular AI inference service
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml

# Delete regular AI inference service
$ kubectl apply -f ./examples/l7service/whoami_airs.yaml
```
