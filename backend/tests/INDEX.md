# 测试索引 · backend

> **写任何新测试前，先在本表里按「行为」搜一遍。**
> AI 反复写重复测试的根因是不知道已经测过什么——本表就是用来挡这个的。
>
> 规则见 [`../../docs/testing-strategy.md`](../../docs/testing-strategy.md) §8。
> `make check-test-index` 会逐项比对，不一致即红。

## 登记规则

- 每个**顶层 `func TestXxx`** 一行，覆盖 `backend/**` 下全部 `*_test.go`
- 表驱动的子用例**不单独登记**，在「覆盖的行为」列里概述
- 「覆盖的行为」写**行为**，不要抄函数名——抄函数名的索引没有检索价值
- 发现两个测试实质相同 → **合并**，不要并列

## 索引

| 测试 | 文件 | 层 | 覆盖的行为 / 验收标准 |
|---|---|---|---|
| _（尚无条目）_ | | | |

<!--
登记示例，新增时照抄这个格式：

| `TestWorkTransition`               | `internal/domain/model/work_test.go`   | domain | Work 状态机全部合法/非法迁移；completed 前必须经过 reviewing_unit |
| `TestSessionCancel_R3_IsIdempotent`| `internal/acp/session_test.go`         | acp    | R3 连续取消只发一次协议请求；含 Runtime 无响应超时 |
| `TestUpdatePrepare_BlocksOnTimeout`| `tests/integration/update_test.go`     | 集成    | prepare 在 Runtime 不回 stopReason 时返回 blocked，不落检查点 |
-->
