# ai-gateway-controller 集成测试框架方案（Mock 化）

> 更新日期：2026-08-19
> 文档类型：测试设计方案
> 目标读者：后端开发工程师、测试工程师
> 关联：`design-docs/sys-design/`（overview / service-discovery-design / inferencepool-discovery-design）

---

## 一、概述

### 1.1 背景与目标

`ai-gateway-controller` 是 k8s controller，依赖两套外部系统：

1. k8s（Service / Endpoints / ConfigMap / Pod / InferencePool）；
2. ai-gateway-api（外部负载均衡 ALB 的开放接口）。

本测试框架采用**纯本地、离线、mock 化**的组件级集成测试：

- **不运行真实 k8s**：用 controller-runtime 的 **fake client** 在内存中 mock k8s 数据，直接驱动
  `ServiceReconciler.Reconcile` / `InferencePoolReconciler.Reconcile` 在进程内执行。
- **mock ai-gateway-api 依赖**：用 `httptest` 起内存版 ALB 开放接口 mock，并记录访问日志供断言。
- **秒级反馈**：无网络、无镜像、无子进程依赖，单个 case 毫秒级完成。
- **组织方式借鉴 `service-controller/test`**：`docs/`（方案）+ `testutil/`（工具）+
  `tests/{feature}/`（用例 + design.md）+ `scripts/`（脚本）+ `command/`（飞轮命令）。

### 1.2 覆盖范围

| 特性 | 实现设计 | 用例 |
|---|---|---|
| 普通 Service 发现（L7 product pool） | `design-docs/sys-design/service-discovery-design.md` | `tests/service_discovery/` |
| InferencePool 发现（EPP pool） | `design-docs/sys-design/inferencepool-discovery-design.md` | `tests/inferencepool_discovery/` |
| 准入过滤（LabelFilter / NamespaceFilter） | `design-docs/sys-design/service-discovery-design.md` §2 | `tests/filter/` |

---

## 二、目录结构

```
ai-gateway-controller/test/
├── docs/
│   └── README.md                          # 本方案文档
├── integration/                           # ★ 本框架（进程内 mock 集成测试，属主 module）
│   ├── testutil/                          # 测试工具包（可被 tests/ 复用）
│   │   ├── alb_mock.go                    # httptest ALB mock（内存 store + access log）
│   │   ├── reconciler.go                  # 构造 Service/InferencePool reconciler（fake client + mock ALB + option.Opts）
│   │   ├── k8s_fake.go                    # fake Service/Endpoints/ConfigMap/Pod/InferencePool 构造
│   │   ├── asserts.go                     # 断言辅助（result cm / annotation / upsert body）
│   │   └── options.go                     # option.Opts 默认值 / 设置与恢复
│   ├── tests/
│   │   ├── filter/                        # 准入过滤（8 用例）
│   │   │   ├── design.md
│   │   │   └── filter_test.go
│   │   ├── service_discovery/             # 普通 Service 发现（11 用例）
│   │   │   ├── design.md
│   │   │   └── service_discovery_test.go
│   │   └── inferencepool_discovery/       # InferencePool 发现（5 用例）
│   │       ├── design.md
│   │       └── inferencepool_discovery_test.go
│   └── artifacts/                         # 飞轮迭代日志（gitignore）
├── scripts/
│   ├── run_tests.sh                       # 一键 vet + 运行用例
│   └── clean.sh                           # 清理运行时产物
└── command/
    └── run-int-tests.md                   # 飞轮驱动命令
```

> 说明：本框架用例直接放在主 module 下（`test/integration/`），从而可引用 `internal/` 包
> （`internal/controllers/loadbalancer`、`internal/option`、`internal/alb`），这是 Go internal 包的约束。

---

## 三、架构设计

### 3.1 进程内驱动示意图

```
 测试用例 (tests/{feature}/*_test.go)
   │ ① 构造 fake Service/Endpoints/ConfigMap/Pod/InferencePool（内存，模拟 k8s 数据）
   │ ② option.Opts 配置（ProductName / ClusterName / ExternalLB.ApiServerAddr ...）
   ▼
 testutil.Env / InferencePoolEnv
   │  Client     = controller-runtime fake client（k8s 数据 mock）
   │  ExternalLB = openapi.AlbProvider ──► httptest ALB mock（ai-gateway-api mock, 含 access log）
   ▼
 ServiceReconciler.Reconcile / InferencePoolReconciler.Reconcile
   │  （完整逻辑：finalizer / ensurePool / pod resync / epp pool / result cm）
   ▼
 断言：fake client 中的 ConfigMap/annotation + mock ALB 的池 store / access log
```

### 3.2 关键组件

| 组件 | 文件 | 职责 |
|---|---|---|
| `ALBMock` | `testutil/alb_mock.go` | httptest 服务器，实现 `/open-api/v1/products/{p}/instance-pools[/{n}]` 与 `/ai-pools[/{n}]` 的 CRUD + 内存 store + 访问日志 |
| `NewEnv` / `NewDownALBEnv` | `testutil/reconciler.go` | 构造 `ServiceReconciler`：注册 scheme、fake client、mock ALB 地址、`option.Opts` |
| `NewInferencePoolEnv` | `testutil/reconciler.go` | 构造 `InferencePoolReconciler` + `InferPoolDict` |
| `k8s_fake.go` | 构造/seed | Service/Endpoints/ConfigMap/Pod/InferencePool 的内存对象工厂 |
| `asserts.go` | 断言 | 读取 result cm、annotation、解析 upsert body |

### 3.3 关键实现要点

1. **ALB 地址必须注入全局 `option.Opts.ExternalLB.ApiServerAddr`**：`OpenApiClient.doReq` 的请求 URL 来自
   `option.Opts.ExternalLB.ApiServerAddr`（见 `internal/alb/openApiClient.go:191`），而非 client 的 `remote` 字段。
   因此测试须将 `option.Opts.ExternalLB.ApiServerAddr` 指向 httptest mock 的 URL。
2. **Service Reconcile 需调用多次**：首次调用会先 `addFinalizer` 并提前返回
   （`service_controller.go:104-115`），第二次调用才真正 `ensurePool` / 写 result cm。
   测试用 `ReconcileUntil` 循环驱动。InferencePool Reconcile 无 finalizer，单次调用即可完成。
3. **`option.Opts` 全局态**：用例设置后必须 `defer`/`t.Cleanup` 恢复，且同包用例**禁止 `t.Parallel()`**。
4. **fake client 支持**：`Get/List/Create/Update/Patch(MergeFrom)/Delete` 均可（controller-runtime fake client），
   且会模拟 finalizer 语义——删除带 finalizer 的 Service 只置 `DeletionTimestamp`，待 finalizer 移除后真正删除。
5. **fail 路径**：将 `option.Opts.ExternalLB.ApiServerAddr` 指向不可达地址（`http://127.0.0.1:1`）即可模拟 ALB 不可达，
   断言 result cm 的 `data.result != "Succ"`。
6. **为让用例可驱动 Reconcile**，在 `internal/controllers/loadbalancer/testing.go` 新增了仅测试用构造器
   `NewTestReconciler`（注入 fake client / fake recorder）与 `NewTestInferencePoolReconciler`
   （注入 fake client / InferPoolDict），等价于生产构造器但无需 manager。

---

## 四、飞轮闭环（设计 → 用例 → 实现 → 重跑）

### 4.1 闭环流程

```
写/改 tests/{feature}/design.md + *_test.go（用例契约）
   │
   ▼
go build ./...  +  go vet ./...          （编译/静态自检）
   │
   ▼
sh test/scripts/run_tests.sh        （vet + go test ./test/integration/...）
   │
   ▼
全部通过 ──► 成功退出
   │
   ▼ 有失败
飞轮诊断（最多 10 轮）:
  1. 读失败 case: design.md / 代码 file:line / 断言 expected vs actual
  2. 决策:
     - 实际行为合理、断言过严/写错  ──► 改 *_test.go（记录理由）
     - 代码与设计不符              ──► 改 internal/（最小改动）
     - design 与用例冲突           ──► 暂停，请用户裁决
  3. 每轮变更与日志写入 test/integration/artifacts/flywheel-iter-<N>.log
  4. 重建 → 重跑 → 下一轮
  达 10 轮仍未通过 ──► 停止，向用户报告剩余失败 + 全部变更/日志，请人工排查
```

### 4.2 安全边界

- 只改 `internal/`、`cmd/`、`test/integration/`、`design-docs/`；不动 `deploy yaml`、`Dockerfile`、`build/*`、`go.mod`、`VERSION`。
- 不自动 `git commit`/`push`。
- 所有飞轮变更与日志保留在 `test/integration/artifacts/`，供人工排查。

### 4.3 运行

```bash
source /home/yeyunxi/setting_go124
cd /home/yeyunxi/yingfei/opensouce/ai-gateway-controller
sh test/scripts/run_tests.sh        # 全部用例
# 或单特性: sh test/scripts/run_tests.sh service_discovery
# 或单用例: go test -v -run TestService_DeletePath ./test/integration/tests/service_discovery/
```

---

## 五、代码位置索引

| 位置 | 用途 |
|---|---|
| `test/integration/testutil/alb_mock.go` | httptest ALB mock + access log |
| `test/integration/testutil/reconciler.go` | `NewEnv` / `NewDownALBEnv` / `NewInferencePoolEnv` |
| `test/integration/testutil/k8s_fake.go` | Service/Endpoints/Pod/InferencePool 构造 |
| `test/integration/testutil/asserts.go` | result cm / annotation / upsert body 断言 |
| `test/integration/tests/{feature}/*_test.go` | 各特性用例 |
| `internal/controllers/loadbalancer/testing.go` | `NewTestReconciler` / `NewTestInferencePoolReconciler`（测试用） |

---

**文档结束**
