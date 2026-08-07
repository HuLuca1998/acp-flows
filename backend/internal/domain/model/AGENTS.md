# AGENTS.md · backend/internal/domain/model

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](/AGENTS.md) 冲突时以本文件为准。
> 上级规则见 [`../AGENTS.md`](../AGENTS.md)。

## 负责什么

领域类型的定义，以及**只依赖该类型自身状态**的方法。

一个聚合一个文件：`work.go` `plan.go` `unit_contract.go` …
值对象与枚举跟着所属聚合走，不单独开文件。

**模型是充血的**：状态流转、不变量校验、自身派生属性都写在模型上。
`Work.Transition(to)` 属于模型，不属于 service。

> **贫血模型是失败信号。** 如果这里全是纯字段的 struct、逻辑都跑到 service 里去了，
> 说明分层做反了，停下来重新设计。

## 不负责什么

- ✗ 需要访问数据库、文件系统、网络的方法 → 那是 `app` 层用例
- ✗ 跨多个聚合的编排 → `domain/policy/`
- ✗ API 的请求/响应 DTO → 那是 `api/gen` 的生成物
- ✗ 数据库行结构体 → `store/entity` 的私有类型

## 依赖方向

| | |
|---|---|
| 允许 import | **仅标准库** + `internal/constant` |
| 禁止 import | 本仓库其他任何包、任何第三方库 |

由 `.golangci.yml` 的 `domain` 规则强制。

**不出现 `context.Context`，不出现 `time.Now()`。** 前者意味着这段逻辑在做 IO，
后者会让测试变成薛定谔的 —— 时间要从外面传进来。

## 检查命令

```bash
cd backend && go test ./internal/domain/... -count=1 -race
cd backend && go test ./internal/domain/... -cover   # 门槛 ≥ 90%
```

覆盖率门槛是全仓库最高的 **90%** —— 这一层零 IO、零依赖，
测不到只可能是没写测试。

## 改这里之前必读

- [`docs/spec/domain-model.md`](../../../../docs/spec/domain-model.md) —— 115 条不变量
- [`docs/rules/coding-standards.md`](../../../../docs/rules/coding-standards.md) §1.1
- [`docs/glossary.md`](../../../../docs/glossary.md) —— **状态词一律英文、不翻译**

## 本域特有的坑

- **枚举必须配穷举测试。** 加一个取值而没接进状态机时，测试要红。
  `Work` 的 11 个状态就是这么守住的。
- **状态词不许翻译**（`initializing` `executing` `waiting_user` …）。
  它们是协议的一部分，不是文案。
- **版本号按数值比，不按字符串比。** `"1.10.0" < "1.9.0"` 是字符串比较的结果，
  用它判断更新会让用户**永远收不到新版本**，而且没有任何报错。见 `version.go`。
- **解析失败要报错，不要静默返回零值。** `ParseVersion("latest")` 静默当成 `0.0.0`
  的话，发布源写错一个字符就会让全部客户端停止更新 —— 这种故障没有症状。
