# filter —— 准入过滤 集成测试用例设计

> 实现设计契约：`design-docs/sys-design/service-discovery-design.md` §2
> 执行：`sh test/scripts/run_tests.sh filter`
> 被测对象：`filter.LabelFilter()` 与 `filter.NamespaceFilter()`（predicate.Funcs）

## 一、测试目标

验证准入过滤（LabelFilter 的 `bfe-product` 匹配 + EnableRsPool 开关）与 NamespaceFilter：

1. **bfe-product 匹配** → true。
2. **bfe-product 缺失** → false。
3. **bfe-product 值不匹配** → false。
4. **无任何 labels** → false。
5. **enable-rs-pool=false** 时即使标签匹配 → false。
6. **NamespaceFilter**：namespace 在列表内 → true；列表外 → false；`*` → 全部 true。

## 二、测试环境

| 项 | 值 |
|---|---|
| 被测 | `filter.LabelFilter().Create(event.CreateEvent{Object: svc})` / `filter.NamespaceFilter().Create(...)` |
| options | `DefaultOpts()`（`bfe-product-name=AI_product`、`enable-rs-pool=true`） |
| Service | 直接构造 `corev1.Service`（不注入默认 label，便于测缺失/不匹配场景） |

## 三、测试用例

| # | case | 输入 | 断言 |
|---|---|---|---|
| 1 | `product-match` | `bfe-product=AI_product` | true |
| 2 | `no-product` | 无 `bfe-product` | false |
| 3 | `product-mismatch` | `bfe-product=other` | false |
| 4 | `no-labels` | 无 labels | false |
| 5 | `rs-pool-disabled` | `EnableRsPool=false` + `bfe-product=AI_product` | false |
| 6 | `namespace-included` | svc ns 在列表 | true |
| 7 | `namespace-excluded` | svc ns 不在列表 | false |
| 8 | `namespace-all` | `*` | true |

## 四、代码位置索引

| 位置 | 用途 |
|---|---|
| `internal/controllers/filter/label.go:42-59` | isYingfeiRSService |
| `internal/controllers/filter/label.go:61-70` | isYingfeiTargetService |
| `internal/controllers/filter/label.go:72-90` | LabelFilter |
| `internal/controllers/filter/namespace.go:40-52` | isYingfeiTargetNs |
| `internal/controllers/filter/namespace.go:54-66` | NamespaceFilter |
