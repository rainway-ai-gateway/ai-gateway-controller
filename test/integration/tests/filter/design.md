# filter —— 准入过滤 集成测试用例设计

> 实现设计契约：`design-docs/sys-design/service-discovery-design.md` §2
> 执行：`sh test/scripts/run_tests.sh filter`
> 被测对象：`filter.LabelFilter()` 与 `filter.NamespaceFilter()`（predicate.Funcs）

## 一、测试目标

验证准入过滤（LabelFilter 的 `ai-gateway-api-name` 匹配）与 NamespaceFilter：

1. **ai-gateway-api-name 匹配** → true。
2. **ai-gateway-api-name 缺失** → false。
3. **ai-gateway-api-name 值不匹配** → false。
4. **无任何 labels** → false。
5. **`ApiName="*"`** 时任意带 `ai-gateway-api-name` 标签的 Service → true。
6. **NamespaceFilter**：namespace 在列表内 → true；列表外 → false；`*` → 全部 true。

## 二、测试环境

| 项 | 值 |
|---|---|
| 被测 | `filter.LabelFilter().Create(event.CreateEvent{Object: svc})` / `filter.NamespaceFilter().Create(...)` |
| options | `DefaultOpts()`（`ai-gateway-api-name=rainway-ai-gatway-demo`） |
| Service | 直接构造 `corev1.Service`（不注入默认 label，便于测缺失/不匹配场景） |

## 三、测试用例

| # | case | 输入 | 断言 |
|---|---|---|---|
| 1 | `api-name-match` | `ai-gateway-api-name=rainway-ai-gatway-demo` | true |
| 2 | `no-api-name` | 无 `ai-gateway-api-name` | false |
| 3 | `api-name-mismatch` | `ai-gateway-api-name=other` | false |
| 4 | `no-labels` | 无 labels | false |
| 5 | `api-name-wildcard` | `ApiName="*"` + `ai-gateway-api-name=anything` | true |
| 6 | `namespace-included` | svc ns 在列表 | true |
| 7 | `namespace-excluded` | svc ns 不在列表 | false |
| 8 | `namespace-all` | `*` | true |

## 四、代码位置索引

| 位置 | 用途 |
|---|---|
| `internal/controllers/filter/label.go` | isYingfeiRSService |
| `internal/controllers/filter/label.go` | isYingfeiTargetService |
| `internal/controllers/filter/label.go` | LabelFilter |
| `internal/controllers/filter/namespace.go` | isYingfeiTargetNs |
| `internal/controllers/filter/namespace.go` | NamespaceFilter |