# 术语表

> 从根 `AGENTS.md` 下沉——它是常驻上下文，装不下这张表（见 `ai-playbook.md` §7）。
> **改术语要同步四处**：本表 · `internal/constant/` · `api/openapi.yaml` · 前端 `constants/`。

界面语言是简体中文，**标识符与状态词保留英文原值并等宽显示**。

界面语言是简体中文，**标识符与状态词保留英文原值并等宽显示**。

| 中文 | 英文标识 | 说明 |
|---|---|---|
| 工作 | `Work` | 一次完整的开发任务，独占一个 git worktree |
| 计划 | `Plan` / `PlanVersion` | 只增不改，重规划产生新版本 |
| 子计划 | `Subplan` | 计划下的阶段，构成有向无环图 |
| 单元 | `Unit` | 最小可执行工作单位 |
| 单元契约 | `UnitContract` | 冻结的交接规格，含写入边界与验收标准 |
| 尝试 | `Attempt` | 对一个单元的一次执行 |
| 证据 | `Evidence` | diff / 测试输出 / 命令记录 / 审查意见 |
| 检查点 | `Checkpoint` | 可恢复点，绑定 commit hash |
| 决策等级 | `D0`–`D3` | D0 自动 · D1 记录 · D2 需确认 · D3 逐次授权 |
| 主工作树 | `worktree` | git worktree |
| 权限档 | `PermissionProfile` | 经 `set_config_option`（`category: mode`）设置。`session/set_mode` 官方已废弃，只作降级 |
| 权限裁决 | `request_permission` | ACP `session/request_permission` |
| 角色 | `Role` | 8 个预置角色，先定义再绑定 Runtime |
| 运行时 | `Runtime` | 一个 ACP adapter 进程（claude / codex） |

**状态词一律英文、不翻译、等宽显示（共 11 个）：**

`initializing` · `initializing_failed` · `clarifying` · `planning` · `ready` ·
`executing` · `reviewing_unit` · `waiting_user` · `paused` · `completed` · `failed`

> 设计规范 §09 只列了后 9 个——那是**对话状态行显示的子集**，
> `initializing` 阶段还没有对话。见 [`docs/adr/0006`](docs/adr/0006-open-question-rulings.md) Q1。
>
> **终态三个**：`initializing_failed` · `completed` · `failed`。

**语气：工程化、精准、可核对。**

- ✓ 「单元 unit-012 契约 v3 已冻结」
- ✗ 「我已经准备好开始写这块了 🎉」

按钮用动词短语（`创建 worktree 并开始` / `请求 push 授权`），不用「确定」「提交」这类空动词；
破坏性动作要在按钮上写清后果（`丢弃 2:14 工作`）。

---
