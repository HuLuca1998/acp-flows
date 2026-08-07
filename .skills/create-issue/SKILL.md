---
name: create-issue
description: 在 HuLuca1998/acp-flows 建 GitHub issue 时使用。触发场景——发现了不属于当前任务范围的缺陷、需要记录一个待办、把一个大需求拆成可执行的条目、或用户说「开个 issue」「记一下这个问题」。负责 issue 的标题格式、必填内容、标签、以及什么情况根本不该开 issue。
---

# 建 issue 规范

## 先判断：这件事该不该开 issue

| 情况 | 处理 |
|---|---|
| 当前任务范围内、马上能修 | **直接修**，不开 issue |
| 超出当前任务边界（铁律 4），但确实是问题 | ✅ 开 issue |
| 需求还没想清楚，不知道要做什么 | ❌ 先走 `brainstorming`，想清楚再开 |
| 只是「代码看着不太顺眼」 | ❌ 不开。没有具体失败场景的观感不是 issue |
| 文档过期了 | ❌ 直接改。文档滞后按缺陷处理，当场修 |

**一个 issue 一件事。** 装了三件事的 issue 永远关不掉。

## 标题格式

```
<type>(<scope>): <祈使语气的一句话>
```

`type` / `scope` 取值与提交信息一致，见 [`docs/rules/git-workflow.md`](../../docs/rules/git-workflow.md) §2。

```
✓ fix(acp): session/cancel 未等 stopReason 就返回，游标可能丢失
✓ feat(ui): 记忆页缺少「提升为跨项目」的确认对话框
✗ bug          ✗ 优化一下事件流       ✗ TODO: 重构 store
```

## 正文必填

用这个模板，**条目不许删**：

```markdown
## 现象
<!-- 具体到可复现：什么输入 / 什么状态 → 什么结果。没有具体场景的不要开 issue。 -->

## 期望
<!-- 应该是什么行为，以及依据是哪份文档的哪一条 -->

## 证据
<!-- 命令输出 / 测试失败信息 / 相关代码位置 file.go:42。禁止转述。 -->

## 影响范围
<!-- 哪些包 / 页面 / 用户路径会受影响 -->

## 建议的验收标准
<!-- 逐条列，每条要能变成一句可执行断言。修这个 issue 的人会照着写测试。 -->
- [ ]
- [ ]
```

「建议的验收标准」是最重要的一节——它直接决定接手的人能不能做到测试先行。
写不出验收标准，说明现象还没描述清楚，回去补。

## 标签

| 标签 | 含义 |
|---|---|
| `bug` `feat` `docs` `chore` | 类型，与 type 对应 |
| `M0` `M1` `M2` `M3` `M4` | 归属里程碑，见 [`docs/plan/roadmap.md`](../../docs/plan/roadmap.md) |
| `blocked` | 被别的 issue 卡住，必须写清被谁卡住 |
| `needs-decision` | 需要人拍板才能继续（对应产品里的 D2） |

## 命令

```bash
# 建
gh issue create --title "fix(acp): ..." --body-file /tmp/issue.md --label bug,M0

# 查我负责的
gh issue list --assignee @me --state open

# 关（在 PR 里写 "Closes #12" 会自动关，优先用那种）
gh issue close 12 --comment "由 #34 修复"
```

## 禁止

- ✗ 标题写 `bug` / `优化` / `TODO` 这类无信息词
- ✗ 现象里没有可复现的具体场景
- ✗ 证据是转述（「我试了一下不行」）而不是命令输出
- ✗ 一个 issue 装多件事
- ✗ 开完 issue 就顺手在当前分支把它修了 —— 那就不该开 issue
