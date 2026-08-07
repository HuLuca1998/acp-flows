# Runtime 配置：字段与取值

> **设计稿管形，实测管值**（根 [`AGENTS.md`](../AGENTS.md) 铁律 3）。
> 界面长什么样看 `design/ACP Duet 1a.dc.html`；**每个字段叫什么、能取什么值，看本文**。
>
> 数据来自 `make probe` 的真机报告（`backend/tests/fixtures/probe/`），
> 绑定版本：**`codex-acp 1.1.7`** · **`claude-agent-acp 0.63.0`**（2026-08-07）。
> 升级 runtime 后必须复验——本文的每张表都是版本相关的。

---

## 1. 一条总规则：按 `category` 取，不按 `id` 取

真机实测（[`acp-field-notes.md`](acp-field-notes.md) §7.1）：

| 概念 | claude 的 `id` | codex 的 `id` | 两端 `category` |
|---|---|---|---|
| 权限档 | `mode` | `mode` | **`mode`** |
| 模型 | `model` | `model` | **`model`** |
| 推理强度 | `effort` | `reasoning_effort` | **`thought_level`** |
| 快速模式 | `fast` | `fast-mode` | **`model_config`** |

`id` 三处不同、`category` 四处全同。

```go
// ✓ 两端通用，接第 3 个 runtime 也不用改
opt, ok := byCategory(sess, constant.ConfigCategoryMode)

// ✗ 硬编码 id，换个 runtime 就取不到
opt := byID(sess, "reasoning_effort")
```

**`category` 可能为空**（claude 的 `agent` 项就是），按 category 取时要容忍。

---

## 2. 权限档：取值随 runtime 变，不能硬编码

| | claude 0.63.0 | codex 1.1.7 |
|---|---|---|
| 取值 | `auto` · `default` · `acceptEdits` · `plan` · `dontAsk` · `bypassPermissions` | `read-only` · `agent` · `agent-full-access` |
| 默认 | `default` | **`agent`** |
| 档位数 | 6 | 3 |

**两组取值没有任何交集。** 界面上的下拉选项必须**动态来自 `session/new` 的响应**，
不能写死。

### ⚠️ 设计稿这里有事实性错误

设计稿的「角色与 Runtime」表把 **codex 的实现工程师配了 `auto` 档** ——
codex 根本没有这个档，`auto` 只在 claude 的 6 个里。

按实测修正后的绑定（见 §4），并回设计稿改正。

### 应用层用 `PermissionProfile`，不用 runtime 的原始档名

```go
type PermissionProfile string   // 应用概念，跨 runtime 稳定
const (
    PermissionReadOnly  PermissionProfile = "read_only"
    PermissionWorkspace PermissionProfile = "workspace"
    PermissionFull      PermissionProfile = "full"
)
```

映射表在 adapter 内部，**上层永远不出现 `"read-only"` 或 `"acceptEdits"` 这样的字面量**：

| `PermissionProfile` | claude | codex |
|---|---|---|
| `read_only` | `plan` | `read-only` |
| `workspace` | `acceptEdits` | `agent` |
| `full` | `bypassPermissions` | `agent-full-access` |

> `plan` 对应 `read_only` 是推定：claude 的 `plan` 档是"只规划不动手"，
> 语义最接近只读。**待用真实会话验证**（M0 U0.3.2 的剩余项）。

---

## 3. 设置方式：`set_config_option`，参数名是 `configId`

```jsonc
// ✓
{ "sessionId": "...", "configId": "mode", "value": "read-only" }
```

三条实测出来的：

1. **参数名是 `configId`**，不是 `optionId`
2. **`session/new` 的 params 里带 `model` 两端都静默忽略** —— 必须建会话后单独设
3. **响应回全量 `configOptions`** —— 设了是否生效当场能回读，
   这是两端唯一都成立的「它到底用了哪个模型」的证据

`session/set_mode` 官方已废弃（ADR 0006 Q4c），只在 Runtime 不提供
`configOptions` 时降级使用。

---

## 4. 角色与 Runtime 的默认绑定（按实测修正）

设计稿的 8 个角色不变，**权限档一列按实测重写**：

| 角色 | AI 操作 | Runtime | 权限档 | 权限裁决 |
|---|---|---|---|---|
| 需求分析师 | `clarify` `snapshot` | claude | `read_only` | 逐条询问 |
| 计划架构师 | `plan` `subplan_dag` | claude | `read_only` | 逐条询问 |
| 单元设计师 | `unit_contract` | claude | `read_only` | 逐条询问 |
| 实现工程师 | `implement` | codex | **`workspace`** ← 设计稿写的 `auto` 不存在 | 逐条询问 |
| 测试执行者 | `test` `report` | codex | `read_only` | 自动允许读 |
| 实现审查员 | `review_unit` | claude | `read_only` | 自动允许读 |
| 决策顾问 | `advise_decision` | claude | `read_only` | 自动允许读 |
| 记忆管理员 | `curate_memory` | claude | `read_only` | 自动允许读 |

**只有实现工程师需要写权限。** 其余七个角色都是读+分析，给 `read_only`——
这同时让「客户端裁决回调」真正生效（默认档下它一次都不会被调用，见
[`acp-field-notes.md`](acp-field-notes.md) §2）。

> `Runtime` 一列是**注册表引用**不是二值枚举（ADR 0006 Q13）——
> 设置页的下拉选项来自已安装 runtime 列表。

---

## 5. 权限裁决是**客户端策略**，与 runtime 无关

这一列不属于 ACP 协议，是我们自己对 `session/request_permission` 的应答策略：

| 取值 | 行为 |
|---|---|
| 逐条询问 | 挂起请求，投递给 UI，等用户裁决 |
| 自动允许读 | `read` / `grep` 类工具自动 allow，其余挂起 |
| 自动拒绝 | 一律 reject（**只用于探针与隔离测试**，不用于真实任务） |

**「自动拒绝」不要用于真实任务**：agent 想读文件被拒 → 没有素材 →
不产出结构化结果，只回一句解释（field-notes §5 坑 2）。

**裁决是策略层不是安全边界。** 三层防线缺一不可：
① 权限档（`set_config_option`）② 隔离的 cwd（worktree）③ OS 级兜底。

---

## 6. 模型与推理强度：只显示，不配置

设计稿的角色卡显示 `claude · sonnet · 中`，同时设置页写
「模型与推理强度不在协议里，这里不设」。两者可以共存（ADR 0006 Q32）：

**显示的是观测结果，不是配置项。**

- 数据来自 `set_config_option` 响应里回读的 `currentValue`
- 记在 **`Attempt`** 上（不在 `Role` 上）—— 它是"这次用了什么"
- 角色卡标注为「本次使用」，**不提供修改入口**

模型清单**只读 `configOptions` 的 `category: model`**，
不走 `session/new` 顶层的 `models` —— 那个只有 codex 有，且是笛卡尔积。

---

## 7. 隔离开关的实现映射

设计稿设置页三个开关，实测方案见 [`acp-field-notes.md`](acp-field-notes.md) §4：

| 开关 | 默认 | claude 实现 | codex 实现 |
|---|---|---|---|
| 关闭 Runtime 机器级记忆 | **开** | `settingSources: ["project"]` + `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | `CODEX_CONFIG` 的 `skills.config` 逐个 `enabled:false` |
| 禁用未授权项目 MCP | **开** | `strictMcpConfig: true` | `CODEX_CONFIG` 的 `mcp_servers` 逐个 `enabled:false` |
| 允许 Runtime 内建 Skill | **开**（关不掉） | — | — |

第三个开关 **UI 上必须标注「内建 Skill 无法关闭，此开关仅用于知情」**——
提供一个关不掉的开关比不提供更糟（ADR 0006 Q38）。

### codex 的两个硬约束

1. **`session/new` 的 `mcpServers` 必须传空数组** —— 非空会整体覆盖
   thread config 的 `mcp_servers` 键，禁用条目全部丢失。Duet 注入的 MCP 走 `CODEX_CONFIG`
2. **skill 加载一律走 `additionalDirectories`** —— cwd 隐式发现实测不稳定

---

## 8. 环境变量：spawn 前必须清

| runtime | 必须删掉 |
|---|---|
| claude | `CLAUDECODE` · `CLAUDE_CODE_ENTRYPOINT` · `CLAUDE_CODE_SSE_PORT` |
| codex | `CODEX_SANDBOX` · `CODEX_SANDBOX_NETWORK_DISABLED` |

**对本项目必踩**：Duet 自己就在 Claude Code 里开发，不删这些
`claude-agent-acp` 会误判自己嵌套而**拒绝服务**。

---

## 9. 改这份文档的规矩

- **每张表都绑定 runtime 版本**。升级后跑 `make probe` 复验，
  改动的地方标注新版本号与日期
- 与设计稿冲突时**以本文为准**，同时回设计稿修正（铁律 3 的边界条款）
- 标注为「推定」「待验证」的（例如 §2 里 `plan` ↔ `read_only` 的映射），
  验证前不许当成事实用
