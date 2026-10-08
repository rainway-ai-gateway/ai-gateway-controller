# ai-gateway-controller 集成测试框架方案（Mock 化）

> 更新日期：2026-10-08
> 文档类型：测试设计方案
> 目标读者：后端开发工程师、测试工程师
> 关联：`design-docs/sys-design/`（overview / service-discovery-design）、`design-docs/api-define/service-yaml-api-define.md`

---

## 一、概述

### 1.1 背景与目标

`ai-gateway-controller` 是 k8s controller，依赖两套外部系统：

1. k8s（Service / Endpoints / ConfigMap）；
2. ai-gateway-api（外部负载均衡 ALB 的 InnerAPI）。

本测试框架采用**纯本地、离线、mock 化**的组件级集成测试：

- **不运行真实 k8s**：用 controller-runtime 的 **fake client** 在内存中 mock k8s 数据，直接驱动
  `ServiceReconciler.Reconcile` 在进程内执行。
- **mock ai-gateway-api 依赖**：用 `httptest` 起内存版 InnerAPI（`/k8s_pools`）mock，并记录访问日志供断言。
- **秒级反馈**：无网络、无镜像、无子进程依赖，单个 case 毫秒级完成。
- **组织方式借鉴 `service-controller/test`**：`docs/`（方案）+ `testutil/`（工具）+
  `tests/{feature}/`（用例 + design.md）+ `scripts/`（脚本）+ `command/`（飞轮命令）。

### 1.2 覆盖范围

| 特性 | 实现设计 | 用例 |
|---|---|---|
| 普通 Service 发现（k8s_pools 上报） | `design-docs/sys-design/service-discovery-design.md` | `tests/service_discovery/` |
| 准入过滤（LabelFilter / NamespaceFilter） | `design-docs/sys-design/service-discovery-design.md` §2 | `tests/filter/` |

---

## 二、目录结构

```
ai-gateway-controller/test/
├── docs/
│   └── README.md                          # 本方案文档
├── integration/                           # ★ 本框架（进程内 mock 集成测试，属主 module）
│   ├── testutil/                          # 测试工具包（可被 tests/ 复用）
│   │   ├── alb_mock.go                    # httptest InnerAPI k8s_pools mock（内存 store + access log）
│   │   ├── reconciler.go                  # 构造 Service reconciler（fake client + mock ALB + option.Opts）
│   │   ├── k8s_fake.go                    # fake Service/Endpoints 构造（BuildService/BuildEndpoints）
│   │   ├── asserts.go                     # 断言辅助（result cm / annotation / k8s pool 实例数组）
│   │   └── options.go                     # option.Opts 默认值 / 设置与恢复
│   ├── tests/
│   │   ├── filter/                        # 准入过滤（8 用例）
│   │   │   ├── design.md
│   │   │   └── filter_test.go
│   │   └── service_discovery/             # 普通 Service 发现（11 用例）
│   │       ├── design.md
│   │       └── service_discovery_test.go
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
   │ ① 构造 fake Service/Endpoints（内存，模拟 k8s 数据；result ConfigMap 由 reconciler 产生）
   │ ② option.Opts 配置（ApiName / ClusterName / ExternalLB.ApiServerAddr ...）
   ▼
 testutil.Env
   │  Client     = controller-runtime fake client（k8s 数据 mock）
   │  ExternalLB = openapi.AlbProvider ──► httptest InnerAPI mock（k8s_pools, 含 access log）
   ▼
ServiceReconciler.Reconcile
    │  （完整逻辑：ensurePool / ensureK8sPool / deletePool / result cm）
    ▼
  断言：fake client 中的 ConfigMap/annotation + mock ALB 的池 store / access log
```

### 3.2 关键组件

| 组件 | 文件 | 职责 |
|---|---|---|
| `ALBMock` | `testutil/alb_mock.go` | httptest 服务器，实现 InnerAPI：`PUT /inner-api/v1/k8s_pools/{n}/instances`、`GET`/`DELETE /inner-api/v1/k8s_pools/{n}`、`GET /inner-api/v1/k8s_pools`；含内存池 store 与访问日志 |
| `NewEnv` / `NewDownALBEnv` | `testutil/reconciler.go` | 构造 `ServiceReconciler`：注册 scheme、fake client、mock ALB 地址、`option.Opts`；`ReconcileOnce` / `ReconcileUntil` 驱动 |
| `k8s_fake.go` | 构造/seed | `BuildService` / `BuildEndpoints`：Service/Endpoints 内存对象工厂 |
| `options.go` | 配置 | `DefaultOpts()` 默认参数；`SetOpts` 安装/恢复全局 `option.Opts` |
| `asserts.go` | 断言 | 读取 result cm、annotation、finalizer，解析 k8s pool 实例数组 |

### 3.3 关键实现要点

1. **ALB 地址注入**：`NewAlbProvider` 用 `opts.ExternalLB.ApiServerAddr` 构造 `InnerApiClient`（请求 URL 取 client 的
   `remote` 字段）。测试须先将 `option.Opts.ExternalLB.ApiServerAddr` 指向 httptest mock 的 URL。
2. **Service Reconcile 需调用多次**：首次调用会先 `addFinalizer` 并提前返回
   （`service_controller.go` 的 `Reconcile`），第二次调用才真正 `ensurePool` / 写 result cm。
   测试用 `ReconcileUntil` 循环驱动。
3. **`option.Opts` 全局态**：用例设置后必须 `defer`/`t.Cleanup` 恢复，且同包用例**禁止 `t.Parallel()`**。
4. **fake client 支持**：`Get/List/Create/Update/Patch(MergeFrom)/Delete` 均可（controller-runtime fake client），
   且会模拟 finalizer 语义——删除带 finalizer 的 Service 只置 `DeletionTimestamp`，待 finalizer 移除后真正删除。
5. **fail 路径**：将 `option.Opts.ExternalLB.ApiServerAddr` 指向不可达地址（`http://127.0.0.1:1`）即可模拟 ALB 不可达，
   断言 result cm 的 `data.result != "Succ"`。
6. **为让用例可驱动 Reconcile**，在 `internal/controllers/loadbalancer/testing.go` 新增了仅测试用构造器
   `NewTestReconciler`（注入 fake client / fake recorder），等价于生产构造器但无需 manager。

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
cd "$(git rev-parse --show-toplevel)"   # 仓库根（ai-gateway-controller）
sh test/scripts/run_tests.sh           # 全部用例（vet + go test ./test/integration/...）
# 或单特性: sh test/scripts/run_tests.sh service_discovery
# 或单用例: go test -v -run TestService_DeletePath ./test/integration/tests/service_discovery/
```

---

## 五、代码位置索引

| 位置 | 用途 |
|---|---|
| `test/integration/testutil/alb_mock.go` | httptest InnerAPI k8s_pools mock + access log |
| `test/integration/testutil/reconciler.go` | `NewEnv` / `NewDownALBEnv` / `ReconcileUntil` |
| `test/integration/testutil/options.go` | `DefaultOpts` / `SetOpts` |
| `test/integration/testutil/k8s_fake.go` | `BuildService` / `BuildEndpoints` 构造 |
| `test/integration/testutil/asserts.go` | result cm / annotation / finalizer / k8s pool 实例数组断言 |
| `test/integration/tests/{feature}/*_test.go` | 各特性用例 |
| `internal/controllers/loadbalancer/testing.go` | `NewTestReconciler`（测试用） |

---

## 六、跨组件集成测试驱动（harness）

控制器生产进程依赖真实 kube-apiserver（`ctrl.GetConfig()`），无法在无 k8s 环境下作为进程启动。
为了在 integration-test 仓库验证“控制器 → 真实 ai-gateway-api”的上报契约，本仓库提供测试驱动二进制：

```text
test/integration/harness/main.go   （package main）
```

- **职责**：读入 JSON 计划（`set` / `reconcile` / `delete_service`），用 controller-runtime
  **fake client** 在内存 mock Service/Endpoints，进程内驱动**真实 `ServiceReconciler`**；其
  `AlbProvider`/`InnerApiClient` 指向真实 ai-gateway-api（由 integration-test 启动）；执行完输出
  JSON 报告（reconcile 错误、Service 是否存在、finalizer、annotation、result ConfigMap）。
- **为什么放在本仓库**：需要引用 `internal/controllers/loadbalancer`、`internal/alb`、
  `internal/option` 等 internal 包；integration-test 是独立 module，无法导入。
- **边界**：内嵌的是真实的 reconciler 与 ALB 客户端，仅 k8s 数据源被替换为 fake client；因此它
  验证的是真实发现逻辑与真实上报契约，而不是 mock 逻辑。

计划/报告的字段定义与运行入口见 integration-test 仓库
`test-cases/implementation/common/controller_harness.go`；对应测试场景为 **SC43
ai-gateway-controller K8s 池发现与上报**（`test-cases/implementation/scenario-SC43-controller-k8s-pool-discovery/`）。

运行方式（由 integration-test 自动编译并执行）：

```bash
# 手动调试
go build -o /tmp/ctrl-harness ./test/integration/harness
/tmp/ctrl-harness -plan /path/to/plan.json
```

---

**文档结束**
