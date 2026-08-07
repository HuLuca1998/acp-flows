# ACP 实测笔记

> 来源：`~/Documents/obsidian/30.tech/ACP协议/`（10 篇），出自前一个项目 `ai-workflows` 的
> 真实运行记录。核对日期 **2026-07-31 / 08-03**，实跑版本 `claude-agent-acp 0.63.0` ·
> `codex-acp 1.1.7` · SDK `1.3.0`。
>
> [`acp-integration.md`](../spec/acp-integration.md) 是**规格**（对着官方规范写的，该怎么做）；
> 本文是**实测**（真机上是什么样，以及前一个项目在哪些地方栽了）。
> **两者冲突时，先看 §7 的裁定表。**

## 权威性分级

动手前先分清一条结论有多硬：

| 级别 | 来源 | 怎么对待 |
|---|---|---|
| **A · 官方规范** | agentclientprotocol.com | 权威。与实测冲突时按规范写，但要留降级 |
| **B · 本机实测** | 本文 | 可依赖，但**绑定到具体版本**——升级 runtime 要复验 |
| **C · 源码阅读** | adapter 仓库 | 可依赖，但内部实现随时会变 |
| **D · 设计稿假设** | `design/*.dc.html` | **最弱**。设计稿是产品意图，不是协议事实 |

---

## 1. 前一个项目的 10 条硬性错误 ★★

**这是本文最有价值的部分。** 这些不是理论风险，是同一个人在同一个协议上已经踩过的坑。
Duet 的设计稿里那条「主管 AI 会话 id 丢失修复」，就是 H-1。

**每一条都要在 Duet 里有对应的防线，而且防线要有测试。**

| # | 错误 | 症状 | Duet 的防线 |
|---|---|---|---|
| **H-1** | 会话 id 在映射层被丢，每条消息变成一条会话 | 不是报错，是**变慢 + 失忆**：每问一句起一个新进程，agent 手上永远白纸 | `sessionId` 端到端穿透要有集成测试：**第 2 轮必须能复述第 1 轮的内容** |
| **H-2** | 流式断在中间层，且没有推送通道 | 工具调用、思考全在 turn 结束后一次性冒出，中间几十秒白屏 | SSE 从第一天就通；测试断言**第一个 chunk 的到达时刻远早于 turn 结束** |
| **H-3** | 系统提示词每轮重发 | 浪费 token + **模型行为漂移**（N 份说明里夹着 N 份过期快照） | 首轮发系统提示，后续只发用户原话。要有断言 |
| **H-4** | 翻开历史会话继续问，agent 一无所知 | 界面从 DB 渲染出历史，**但 ACP 会话早就不在了**，用户以为记得 | 用 `session/load`，不要自己拼「以下是之前的对话」 |
| **H-5** | `stopReason` 被丢弃，截断的答案当成功 | `max_tokens` / `refusal` / `max_turn_requests` 当成 `end_turn` 交出去 | **只有 `end_turn` 算正常说完**，其余四种各有处理 |
| **H-6** | 权限防线够不着，全拒的代码一次都没生效 | 见 §2，默认档下裁决回调根本不被调用 | 建会话后**显式设档**，且设档动作本身要落事件流 |
| H-7~H-10 | 权限档未显式设、事件流未接、界面状态与真实不符、模型列表同步 | | |

### 教训（比清单本身更重要）

> **界面说了一件实现里没有的事，就是硬性错误。**

「已恢复之前的对话」「AI 记得你说过什么」「权限受控」—— 这些承诺一旦写在界面上，
后端必须真的兑现。用户第一句就会发现失忆。

这条直接对应 Duet 的铁律 5（证据优先）与设计规范的「结论不带证据入口」反例。

---

## 2. 权限：默认档下你的裁决代码一次都不会被调用 ★★

前一个项目最危险的一条。**三组对照实测**，同一件事（在 cwd 里建一个文件）：

| 档位 | `request_permission` 次数 | 文件建了吗 |
|---|---|---|
| `agent`（codex 默认）+ 客户端**全拒** | **0** | ✅ **建了** |
| `agent` + 客户端全允许 | 0 | ✅ 建了 |
| `read-only`（`set_mode` 之后）+ 客户端全拒 | 2 | ❌ 没建 |

**一份看起来很严的全拒代码，在默认配置下从未被调用过。**

### 三层防线，缺一不可

```
① runtime 沙箱档（session/set_mode）   ← 执法开关，不设等于没有
② 隔离的 cwd（worktree）              ← 边界
③ OS 级兜底（容器 / 独立用户 / 只读挂载） ← 最后一道
```

**权限回调是策略层，不是安全边界。** 用它做安全会同时失去安全性和可用性（见 §5 坑 2）。

### 档位语义在两端完全不同

| | claude 0.63.0 | codex 1.1.7 |
|---|---|---|
| 可选档 | 6 个：`auto` `default` `acceptEdits` `plan` `dontAsk` `bypassPermissions` | 3 个：`read-only` `agent` `agent-full-access` |
| **默认档** | `default` —— 危险操作会问 | **`agent`** —— 读写文件、跑命令，**不问** |
| 默认档下建文件 | 请求权限 1 次 ✅ | 请求权限 **0** 次 ❌ |

**档位名没有任何交集。** 硬编码任何一个都会在另一端静默失效 ——
这正是 [`design-principles.md`](../rules/design-principles.md) §4.4「差异内化」要解决的问题。

---

## 3. 差异内化的关键实证：按 `category` 取，不按 `id` 取 ★

`configOptions` 里同一个概念，两端的 `id` 不同，但 **`category` 相同**：

| 概念 | claude 的 `id` | codex 的 `id` | 两端的 `category` |
|---|---|---|---|
| 权限档 | `mode` | `mode` | `mode` ✅ |
| 模型 | `model` | `model` | `model` ✅ |
| **推理强度** | **`effort`** | **`reasoning_effort`** | **`thought_level`** ✅ |
| **快速模式** | **`fast`** | **`fast-mode`** | — |

```go
// ✓ 两端通用，接第 3 个 runtime 也不用改
func byCategory(session *Session, category string) (*ConfigOption, bool)
```

**这是「统一抽象、差异内化」最漂亮的一个实证**：不需要维护映射表，
协议本身就提供了语义层的稳定键。**优先找这种稳定键，找不到再建映射表。**

### 其余实测差异

| 差异 | claude | codex | 后果 |
|---|---|---|---|
| `session/new` 顶层 `models` | ❌ 没有 | ✅ 25 个 | 读模型清单**只走 `configOptions`**，别走 `models.availableModels` |
| `usage.cost`（美元） | ✅ 有 | ❌ 没有 | **「这次花了多少钱」只能对 claude 做** |
| `usage.thoughtTokens` | ❌ | ✅ | |
| **fs 代理** | ✅ 走 `fs/write_text_file` | ❌ **完全不走**，用自带 shell | **codex 的 path guard 没有落点** |
| 无头认证 | 复用本机登录态 | 复用登录态 + **支持 `CODEX_API_KEY`** | CI / 批量场景优先 codex |
| `session/load` 恢复 | ✅ 全绿 | ✅ 全绿 | checkpoint 恢复可以依赖它 |

> **`session/new` 的 params 里带 `model` 两端都静默忽略。**
> 要设模型必须 `session/set_config_option`，参数名是 **`configId`**，不是 `optionId`。
> 响应回全量 `configOptions`，**设了是否生效当场能回读** —— 这是两端唯一都成立的
> 「它到底用了哪个模型」的证据。

---

## 4. 隔离与注入 —— 直接对应设计稿的三个开关 ★★

设计稿的「设置 → 角色与 Runtime → 隔离」有三个开关，实测方案已经跑通：

| 设计稿的开关 | claude 侧做法 | codex 侧做法 |
|---|---|---|
| **关闭 Runtime 机器级记忆** | `settingSources: ["project"]`（不含 `user`）+ 环境变量 `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` | `CODEX_CONFIG` 里 `skills.config` 逐个 `enabled: false` |
| **禁用未授权项目 MCP Server** | `strictMcpConfig: true` | `CODEX_CONFIG` 的 `mcp_servers` 里逐个 `enabled: false` |
| **允许 Runtime 内建 Skill** | 无法关闭（见「去不掉的边界」） | 同左 |

### 三条红线（隔离手段的边界）

1. **不修改系统配置** —— `~/.codex`、`~/.claude` 一个字节不写，只读
2. **不影响用户终端直接用 codex / claude**
3. **不修改 / 覆盖目标项目的 skill** —— 项目文件只读

所以隔离只有两类载体：**spawn 时的进程环境变量** + **`session/new` 的协议参数**。

### claude 侧

```jsonc
{ "cwd": "<worktree 路径>",
  "mcpServers": [ /* Duet 注入的 MCP */ ],
  "_meta": { "claudeCode": { "options": {
    "settingSources": ["project"],   // 不含 user ⇒ 机器级 ~/.claude 全不加载
    "plugins": [{ "type": "local", "path": "<Duet 的 skill 分发目录>" }],
    "strictMcpConfig": true          // 只认上面注入的 MCP
  }}}}
```

> ⚠️ **`.mcp.json` 的暴露面**：project 档开着时，目标项目可以在自己的
> `.claude/settings.json` 里写 `enableAllProjectMcpServers: true` **给自己的 MCP 自动放行**
> —— 任何被打开的项目都能让 AI 静默启动任意进程。`strictMcpConfig: true` 封死这条路。
>
> **这条对 Duet 是安全问题**：用户会用 Duet 打开各种来路不明的仓库。

### codex 侧

```jsonc
// session/new
{ "cwd": "<worktree 路径>", "mcpServers": [],   // ← 必须是空数组，见下
  "additionalDirectories": ["<Duet 的 skill 目录>", "<worktree 路径>"] }
```

```jsonc
// 环境变量 CODEX_CONFIG（JSON），会话级覆盖，零文件落地
{ "skills": { "config": [{ "name": "<SKILL.md frontmatter 的 name>", "enabled": false }] },
  "mcp_servers": { "duet": {...}, "<机器级 id>": { "enabled": false } } }
```

三条实测硬规则：

1. **`skills.config` 只有 `name` 选择器有效**，`path` 选择器实测无效。
   `name` 匹配 **SKILL.md frontmatter 的 `name`**，不是目录名，且是**全局匹配**——
   同名的项目 skill 会被误伤。
2. **Duet 注入的 MCP 必须放 `CODEX_CONFIG`，不能走 `session/new` 的 `mcpServers`** ——
   session 级参数非空时，codex-acp 会**整体覆盖** thread config 的 `mcp_servers` 键，
   禁用条目全部丢失。所以 codex 的 `session/new.mcpServers` **传空数组**。
3. 禁用只在本会话生效，**不持久化**，用户终端里的 codex 不受影响。

### skill 加载：一律走 `additionalDirectories`

> **cwd 隐式项目发现（`.codex/skills`）单独依赖时不稳定** —— 同一配置时好时坏
> （疑与信任判定时序有关）。extraRoots 显式注册在多轮实测中从未失手。

副作用：`additionalDirectories` 同时把目录加进 sandbox 可写根。
**生产上让它指向只含 skill 分发内容的独立目录**，不要指向 Duet 自己的代码目录。

### 被否决的方案

`CODEX_HOME=受控目录` 能做到全量隔离，但实测代价是 codex 会把 marketplace 的 git 克隆、
`.system` skill 同步、会话历史等**几千个文件**写满这个目录。**已否决。**

### 验证方法（省模型费）

`available_commands_update` 通知是现成的污染检测器，但**它反映的是「发现」不是「启用」**
（codex 被禁用的 skill 仍出现在列表里）。最终验证要问模型四件事：
列 skill、列 MCP 工具、调用注入的 MCP、报 cwd。

对照数字：隔离前 claude 83 条命令 / codex 53 条，隔离后机器级全消失。

---

## 5. 对 Duet 而言最致命的坑

### 坑 1 · 嵌套会话环境变量 ⚠️ 对本项目必踩

**Claude Code 会给子进程注入 `CLAUDECODE`、`CLAUDE_CODE_ENTRYPOINT`、`CLAUDE_CODE_SSE_PORT`。**
继承下去再传给 `claude-agent-acp`，它会误判自己跑在另一个 agent 内部而**拒绝服务**。

```go
// spawn 前必须删，固化进 runtime 注册表
claude: envRemove = ["CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "CLAUDE_CODE_SSE_PORT"]
codex:  envRemove = ["CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"]
```

**为什么对 Duet 尤其致命**：Duet 本身就是在 Claude Code 里开发的，
开发者自己手跑没问题，一从 Claude Code 会话里启动就炸。

> 由此得出一条测试纪律：**测试与试验一律用 codex**。
> 用 claude 去测会与开发环境撞在一起——嵌套会话、共用登录态、同一份配额。

### 坑 2 · 用 rejectPolicy 跑真实任务 → agent 不产出结构化输出

给了保守的全拒策略 → agent 想 Read/Grep 看真实文件被拒 → 没有素材 →
**不产出结构化结果，只回一句解释**。

**修法**：真实任务用 allow 策略，安全交给 **cwd 隔离 + 沙箱档 + OS 兜底**。

### 坑 3 · cwd 目录名导致会话历史串味 ⚠️ 隐蔽

Claude Code 按 cwd 路径在 `~/.claude/projects/<路径编码>` 下存历史。
用固定前缀创建临时目录时，**新会话可能读到同前缀的旧历史** ——
开一个全新对话，agent 却复述出上次的内容。

**对 Duet 的后果**：这是**数据串扰**，不只是体验问题。多个 Work 的 worktree 路径
如果有共同前缀，可能互相串味。建会话前要清掉自己产生的临时项目历史。

### 坑 4 · 天真的 runTurn 不是流式

```ts
const resp = await handle.prompt(...);        // ← 阻塞到整个 turn 结束
for (const n of updates.slice(before)) yield // ← 然后才补发
```

`AsyncIterable` 的外壳骗了自己。**必须用 `onUpdate` 回调推异步队列**。

### 坑 5 · stderr 必须收集

**agent 崩溃、认证失败、版本错配的信息只在 stderr 里**，ACP 消息流里什么都没有。
不收集，排查基本靠猜。

### 坑 6 · 超时后只 reject 不够

`session/prompt` 实测：追问约 9s，生成完整产物约 32s，真实编码任务到几分钟。

- `initialize` / `session/new`：60s
- `session/prompt`：240s 起

**超时后必须同时 `session/cancel` + 杀进程**，否则 agent 还在后台跑、还在烧钱、
还可能继续改文件。长 turn（>10min）下 stdio 稳定性**尚未充分验证**。

### 坑 7 · 找不到可执行文件

`claude-agent-acp` / `codex-acp` 在 `node_modules/.bin/` 下，不一定在 PATH 里。
用绝对路径，并允许环境变量覆盖。

### 坑 8 · 子进程泄漏

只 create 不 close，或会话对象被 GC 但子进程还活着。
需要：`finally` 里必 close · 并发上限 + LRU 淘汰 · close 先 `SIGTERM` 等一会再 `SIGKILL`。

---

## 6. 成本核算：`totalTokens` 会严重高估

实测一个只回一个词的 turn：

```jsonc
"usage": { "inputTokens": 2, "outputTokens": 4,
           "cachedReadTokens": 15498, "cachedWriteTokens": 12973,
           "totalTokens": 28477 }
```

**真实新增 token 只有 6 个，但缓存读了 1.5 万。**
用 `totalTokens` 判断预算会严重高估 —— **直接用 runtime 给的 `cost.amount`**（仅 claude 有）。

兜底始终保留 **turn 数上限 + 墙钟超时**。

---

## 7. 与既有文档的冲突裁定 ★

| # | 冲突 | 裁定 |
|---|---|---|
| **1** | `acp-integration.md` §2.2 与 `open-questions.md` Q4a 曾判定「设计稿 `mem-188` 说 codex 默认档不询问是**错的**」（依据：源码里 `AgentMode.Agent` 的 `approvalPolicy` 是 `on-request`） | **判定过重，已撤回。** 两者其实一致：`agent` 档 = `workspace-write` 沙箱 + `on-request` 审批。**沙箱内的写操作根本不需要审批**，所以观测到 0 次 `request_permission` 是正确行为。`on-request` 只对越出沙箱的操作生效。**`mem-188` 的实用结论（默认档不问、必须收权）是对的。** |
| 2 | 笔记说 codex 档位是 `read-only`/`agent`/`agent-full-access`；设计稿角色表把 codex 绑到 `auto` | **笔记为准。** `auto` 是 codex **旧版本**（0.16.0）的档名，1.1.7 已改。设计稿用的是过时档名。open-questions Q4b 成立 |
| 3 | 笔记（2026-07-31）未提 `session/set_mode` 废弃；子代理查到官方已挂废弃告示 | **两者都对，时间差。** 迁移方向是 `session/set_config_option`（按 `category` 取 `mode`）。**新代码直接写 `set_config_option`**，`set_mode` 只作降级 |
| 4 | 笔记只说取消要发 `session/cancel`；规范要求**同时用 `cancelled` 应答所有 pending 的权限请求** | **规范为准**（A 级 > B 级）。笔记写于该细节被注意到之前 |
| **5** | `acp-integration.md` §11.2 写 v1 的 `sessionUpdate` 判别值「共 **11** 个」；`M0-acp-foundation.md` U0.4.1 R1 / U0.5.1 R6 写「**9** 类」 | **都不对，实际是 13 个**（下 §7.2）。已就地修正三处并接入穷举测试。**这类"数一数有几个"的结论必须由测试守住**——文档里的数字会漂移，`grep -c` 不会 |

---

## 7.1 真机复验（2026-08-07）★

用 `backend/cmd/acpprobe` 在**本机真实 runtime** 上复验。报告归档在
`backend/tests/fixtures/probe/{codex,claude}.json`，可 diff。

```bash
make probe          # 零模型开销：只做 initialize + session/new
```

实跑版本：**`codex-acp 1.1.7`** · **`claude-agent-acp 0.63.0`** —— 与笔记记录的版本一致。

### 已确认（B 级 → 保持 B 级，但绑定到具体版本与日期）

| 结论 | 复验结果 |
|---|---|
| 两端 `protocolVersion` 都是 `1` | ✅ |
| codex 档位 = `read-only` / `agent` / `agent-full-access`，**默认 `agent`** | ✅ 逐字一致 |
| claude 档位 6 个 = `auto` / `default` / `acceptEdits` / `plan` / `dontAsk` / `bypassPermissions`，**默认 `default`** | ✅ 逐字一致 |
| 两端 `loadSession: true` | ✅ |
| 两端 `promptCapabilities` 含 `image` 与 `embeddedContext` | ✅ |

### ★ 最重要的一条得到证实：按 `category` 取，不按 `id` 取

| 概念 | claude 的 `id` | codex 的 `id` | 两端 `category` |
|---|---|---|---|
| 权限档 | `mode` | `mode` | **`mode`** ✅ |
| 模型 | `model` | `model` | **`model`** ✅ |
| 推理强度 | **`effort`** | **`reasoning_effort`** | **`thought_level`** ✅ |
| 快速模式 | **`fast`** | **`fast-mode`** | **`model_config`** ✅ |

**`id` 三处不同，`category` 四处全同。** 这条不是推测，是本机实测。
[`design-principles.md`](../rules/design-principles.md) §4.4 的「差异内化最佳实证」由此坐实——
协议本身提供了语义层的稳定键，**不需要维护映射表**。

> 笔记原文对「快速模式」的 `category` 记的是 `—`（未知）。现在补上：两端都是 `model_config`。

### 新发现（笔记里没有的）

| # | 发现 | 影响 |
|---|---|---|
| N1 | codex 多一项 `collaboration_mode`（category 同名），claude 没有 | 属于 codex 私有能力，**必须走能力查询暴露，不能假装两端一致** |
| N2 | claude 多一项 `agent` configOption，且 **`category` 为空字符串** | 按 category 取时要能容忍空 category，不能崩 |
| N3 | codex 的 `agentCapabilities.sessionCapabilities` 明确列出 `additionalDirectories` / `close` / `delete` / `list` / `resume` | 隔离方案依赖的 `additionalDirectories` **确实被声明为能力**，不是靠猜 |
| N4 | codex 的 `authMethods` 有 `api-key`（`_meta` 里标了 provider: openai）与 `chat-gpt` 两种 | 印证「codex 支持纯环境变量认证，CI/批量优先选 codex」 |
| N5 | 两端 `mcpCapabilities` 都是 `http: true` / `sse: false` / `acp: false` | MCP 注入只能走 http |

### Q4b 得到证实

`open-questions.md` Q4b 说「设计稿把 codex 实现工程师绑到 `auto` 模式，但 codex 没有这个档」——
**证实**：`auto` 确实只在 claude 的 6 个档里，codex 的三个档里没有。设计稿的角色表需修正。

---

## 7.2 协议 schema 复核（2026-08-07）★

§7.1 是**真机探针**（B 级：观测到的行为）。本节是**官方 schema 源码**
（**A 级**：协议全集，含探针跑不出来的变体）。两者互补——
探针只能看见 runtime 这一轮**碰巧发了什么**，看不见协议**允许发什么**。

**证据源**（本机已装，可复核）：

```bash
SDK=$(npm root -g)/@agentclientprotocol/codex-acp/node_modules/@agentclientprotocol/sdk
grep -o 'sessionUpdate: "[a-z_]*"' $SDK/dist/schema/types.gen.d.ts | sort -u   # v1，13 行
grep -n 'PROTOCOL_VERSION' $SDK/dist/schema/index.d.ts                          # = 1
```

实跑版本：**`@agentclientprotocol/sdk 1.3.0`**（`dist/schema` = v1，`dist/v2/schema` = v2）。

### 裁定 1 · v1 的 `sessionUpdate` 判别值是 **13 个**，不是 11 / 9

| 判别值 | 载荷类型 | Duet 处理 |
|---|---|---|
| `user_message_chunk` `agent_message_chunk` `agent_thought_chunk` | `ContentChunk` | 前两者见 §11.2 映射表 |
| `tool_call` / `tool_call_update` | `ToolCall` / `ToolCallUpdate` | 合并为一类，字段级合并后上抛完整快照 |
| `plan` | `Plan`（`entries[]`） | 丢弃（同名陷阱） |
| **`plan_update`** | `PlanUpdate` | 丢弃。**官方标 `UNSTABLE` / `@experimental`** |
| **`plan_removed`** | `PlanRemoved`（只有 `planId`） | 丢弃。同上 |
| `available_commands_update` `current_mode_update` `config_option_update` `session_info_update` `usage_update` | 各自类型 | 见 §11.2 |

**漏掉的两个都是 `plan_*` 家族**，且都带实验标记。危险不在于漏了功能，
而在于它们会掉进「未知变体」分支**刷 warn 日志**，把真正的未知变体淹掉。

### 裁定 2 · v2 已存在，但**现在不实现**

`dist/v2/schema` 的 `PROTOCOL_VERSION = 2`，判别值换了一批
（`agent_message` / `state_update` / `terminal_output_chunk` / `tool_call_content_chunk` …，
`tool_call` 与 `plan` 两个判别值**消失**）。

**但 §7.1 的真机探针实测：两端协商出来的 `protocolVersion` 都是 `1`。**
→ **M0 只做 v1。** 版本协商失败时关闭连接（U0.5.1 R1），不做 v2 兼容层。

> 这条记下来是为了防止下一轮 AI 打开 SDK 目录看见 `v2/` 就顺手实现它。
> **协议版本以协商结果为准，不以 SDK 里存在什么为准。**

### 这条为什么值得单独记

「一共有几个」这种结论，**写在文档里必然漂移**——三份文档写了三个数字就是证据。
正确做法是**变成检查**：`protocol` 包的穷举测试遍历判别值全集，
新增一个而未登记时测试红。见 `ai-playbook.md` §4 的「能变成检查的不要写进文档」。

---

## 8. 这些笔记怎么维护

- **绑定版本。** 每条 B 级结论都要注明实跑版本。升级 runtime 后**必须复验**，
  否则它就退化成 D 级假设
- **复验的成本很低**：`ACP_LIST_ONLY=1` 式的探针只建会话不发 prompt，零模型开销
- 新踩的坑写进这里，**同时**在 [`../backend/internal/acp/AGENTS.md`](../../backend/internal/acp/AGENTS.md)
  的「本域特有的坑」加一行指过来
- 与规格冲突时，在 §7 加一行裁定，**不要直接改另一份文档** —— 裁定过程本身有价值
