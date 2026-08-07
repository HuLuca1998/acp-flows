---
name: review-diff
description: 审查另一个 AI（或人）写的改动时使用。触发场景：一个 PR 等待审查、用户说「审一下这个改动」「codex 写完了帮我看看」、或你作为独立审查方要判定 accepted / implementation_fix / contract_revision / global_replan。本仓库规定实现方不得审查自己的 PR，本 skill 是审查方的执行流程。不用于自己写完代码的自查（那是 tdd-unit 的第⑦步）。
---

# 独立审查

> 规则：**实现方不得审查自己的 PR。** Claude 实现 → Codex 审查；Codex 实现 → Claude 审查。
> 见 [`docs/git-workflow.md`](../../docs/rules/git-workflow.md) §3。

## 审查方的立场

**怀疑式。「CI 全绿」不等于「验证过」。**

你要回答的核心问题只有两个：

1. **这些测试真的验证了行为吗？** 还是只是覆盖率好看
2. **边界有没有被悄悄扩大？** diff 是否越出了任务允许的范围

其余（格式、命名、lint）机器已经查过了，不要把审查时间花在那上面。

## 步骤

### ① 先看边界，再看代码

```bash
gh pr diff <n> --name-only
```

对照 PR 描述里写的「允许改动范围」。**先判断有没有越界**——
越界的 diff 无论写得多好都要打回，这是铁律 4。

设计稿里有个真实案例：`attempt 1 的 diff 越出写入边界（改动了 EngineEvent 公开枚举），已驳回`。

### ② 找到「先红的测试」，验证它是真的

```bash
gh pr checkout <n>
```

对每条验收标准问：

- 这条标准对应哪个测试？找不到 → **无证据即未通过**
- **把实现里的关键一行删掉/改掉，这个测试会红吗？** ← 最有效的一招
- 断言的是具体值，还是 `NotNil` / `NoError` 这类恒真式？
- 有没有 mock 喂 mock（断言的是自己刚设的返回值）？
- 边界、错误、幂等测了吗，还是只有 happy path？

假测试图鉴见 [`docs/testing-strategy.md`](../../docs/rules/testing-strategy.md) §3。

### ③ 检查有没有为了变绿而动测试

```bash
gh pr diff <n> -- '*_test.go' '*.test.ts*'
```

**这是最严重的违规**，出现即打回：

- 删掉了原本会红的测试
- 加了 `t.Skip()` / `it.skip()`
- 把 `assert.Equal(3, n)` 放宽成 `assert.True(n > 0)`
- 注释掉断言

### ④ 契约与文档

- 改了接口 → `api/openapi.yaml` 同步了吗？生成物跑过了吗？
- 改了 UI → 能在 `design/Duet Spec.dc.html` 指出对应条目吗？
- 新建关键目录 → `AGENTS.md` + `CLAUDE.md` 补了且填实了吗？
- 新工具函数 / 新测试 → 索引登记了吗？**有没有和已有的重复？**

### ⑤ 判定四选一

| 结论 | 什么时候用 | 后续 |
|---|---|---|
| `accepted` | 边界内、测试真实、契约同步 | 可以合并 |
| `implementation_fix` | 实现有问题，但契约/需求没问题 | 打回，实现方修，不改契约 |
| `contract_revision` | 需求或接口本身定义有问题 | 回去改 spec 或任务描述，重新冻结 |
| `global_replan` | 架构假设错了 | **停下来找人**，不要自行重构 |

## 输出格式

```markdown
## 结论：implementation_fix

## 依据

1. **写入边界越界** — `internal/domain/model/event.go` 不在允许范围内，
   且改动了 `EngineEvent` 的公开枚举（外部行为变化）。
2. **R5 无证据** — 验收标准「reviewing_unit 状态下取消被拒绝」没有对应测试。
3. **TestSessionCancel 是假的** — 把 `session.go:42` 的 `swap` 改成 `store`
   后测试仍然绿，说明它没有验证幂等。

## 建议

R5 建议补一个单元，而不是放宽验收标准。
```

**每一条依据都要能指向具体文件行号或命令输出。** 转述不算依据。

## 用 Codex 做审查（Claude 侧）

```
调用 codex-collab skill，只读模式，把 PR diff 与验收标准交给它，
要求返回四选一结论 + 逐条依据。
```

**不要让 Codex 直接改代码** —— 审查是只读动作。

## 禁止

- ✗ 审查自己实现的 PR
- ✗ 只看 CI 绿了就 `accepted`
- ✗ 结论不带具体依据（文件行号 / 命令输出）
- ✗ 在审查里顺手把代码改了 —— 审查是只读的，问题要打回给实现方
- ✗ 把 lint 能查的东西当作审查重点
