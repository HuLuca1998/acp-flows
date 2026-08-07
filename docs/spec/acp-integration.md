# ACP 协议层与 Runtime 适配

> **本文是 `backend/internal/acp/` 的规格说明，改协议层前必读。**

> 读者：Claude / Codex / 人。本文描述 **M0 目标形态**；实现进度见 [`roadmap.md`](../plan/roadmap.md)。
> 与根 [`AGENTS.md`](../../AGENTS.md) 的六条铁律冲突时以铁律为准。
> 术语按 `AGENTS.md` §8；状态词与协议标识符一律英文原值、等宽显示，不翻译。

> **读法**：本文 ~27k token（占上下文 13%），**不要整篇读**。先在下表定位到章节，
> 再 `Read` 那一段。规则见 [`ai-playbook.md`](../ai-playbook.md) §1。
>
> | 你要做的事 | 读哪一节 | 定位 |
> |---|---|---|
> | 搞清楚这层为什么存在 | §1 | `grep -n '^## 1\.' docs/acp-integration.md` |
> | 确认某个结论是否核实过 | §2 ★ | `grep -n '^## 2\.' docs/acp-integration.md` |
> | 新建文件放哪个包 | §3 | `grep -n '^## 3\.' docs/acp-integration.md` |
> | 改传输层 / ndjson 分帧 | §4 | `grep -n '^## 4\.' docs/acp-integration.md` |
> | 会话建立、prompt 循环 | §5 | `grep -n '^## 5\.' docs/acp-integration.md` |
> | **两段式取消** | §6 ★ | `grep -n '两段式取消' docs/acp-integration.md` |
> | 权限裁决 / `request_permission` | §7 | `grep -n '^## 7\.' docs/acp-integration.md` |
> | 能力探针 / `configOptions` | §8 | `grep -n '^## 8\.' docs/acp-integration.md` |
> | 加一个新 Runtime | §9 §10 | `grep -n '^## 9\.\|^## 10\.' docs/acp-integration.md` |
> | 事件映射到 13 类封闭枚举 | §11 | `grep -n '^## 11\.' docs/acp-integration.md` |
> | **写 / 改 Fake Runtime** | §12 ★ | `grep -n '^## 12\.' docs/acp-integration.md` |
> | 写这层的测试 | §13 | `grep -n '^## 13\.' docs/acp-integration.md` |
> | 错误分类与降级 | §14 | `grep -n '^## 14\.' docs/acp-integration.md` |
> | 「这个要不要做」 | §15 | `grep -n '^## 15\.' docs/acp-integration.md` |
> | 卡在没核实的假设上 | §16 ★ | `grep -n '^## 16\.' docs/acp-integration.md` |
>
> **实测结论不在本文**，在 [`acp-field-notes.md`](../notes/acp-field-notes.md)。
> 本文写「应该怎样」，那份写「真机上实际怎样」——**冲突时以实测为准**。

---

## 0. 阅读约定

本文出现三种断言强度，**混用会造成实现事故，请严格区分**：

| 标记 | 含义 |
|---|---|
| **[SPEC]** | 来自 ACP 官方规范原文，附出处。改实现前先回原文核对，不要凭本文转述 |
| **[SRC]** | 来自 `claude-agent-acp` / `codex-acp` 的公开源码或 npm 元数据，附文件路径与版本 |
| **[DUET]** | 本项目自己的设计决定，协议不管，我们说了算 |
| **[待验证]** | **无法从规范或公开源码确认。实现时必须用真实 Runtime 打通后回填本文，不许当成事实用。** |

**没有标记的段落一律按 [DUET] 处理。**

---

## 1. 为什么 ACP 层是地基

Duet 的全部业务价值都建立在"能可靠驱动两个 ACP Runtime"之上。这一层塌了，上面什么都验证不了：

```
       ┌───────────────────────────────────────────────┐
       │  Work / Plan / Unit / Contract 状态机          │  ← 没有事件流就没有状态迁移
       └───────────────────┬───────────────────────────┘
                           │ 依赖
       ┌───────────────────▼───────────────────────────┐
       │  Evidence 采集 · Checkpoint · 更新 prepare      │  ← 没有两段式取消就落不了检查点
       └───────────────────┬───────────────────────────┘
                           │ 依赖
       ┌───────────────────▼───────────────────────────┐
       │  internal/acp   ★ 本文                         │
       └───────────────────────────────────────────────┘
```

具体的连锁依赖，每一条都可以被指出来：

| 上层需求 | 依赖 ACP 层的什么 | 缺了会怎样 |
|---|---|---|
| 13 类事件流渲染（`architecture.md` §4） | `session/update` → 统一事件信封的映射 | 前端 13 个渲染器一个都测不了 |
| `Evidence` 由应用直接采集（铁律 5） | `tool_call` 的 `content` / `locations` / diff | 只能拿 Agent 转述当证据，直接违反铁律 |
| 更新 `prepare` 返回 `ready`/`blocked`（`release-and-update.md` §5） | 两段式取消能确定性完成或超时 | M1 自动更新做不出来 |
| `Checkpoint` 可恢复 | 取消后 `stopReason` 落盘 + 事件游标可读 | 检查点不可信，恢复即数据错乱 |
| 角色与 Runtime 绑定（8 个预置角色） | `availableModes` / `session/set_mode` / 权限裁决 | 角色页全是死配置 |

### Fake Runtime 必须是第一个交付物

铁律 6 禁止测试碰真实 Runtime 账号，`testing-strategy.md` §5 把"ACP Runtime → Fake Runtime"写成硬约束。
于是形成一个顺序：

```
① protocol 类型 + Fake Runtime 脚本回放
        ↓ 有了它，下面每一步都能先写红测试
② jsonrpc 传输层            ← 对着 Fake 的 golden JSON 测
③ session 生命周期 + 两段式取消
④ capability 探针 / runtime 注册表 / adapter
        ↓
⑤ app 层用例、api 层、前端事件流、e2e
```

**先写 Fake 不是"顺手把测试夹具做了"，而是没有它后面每一步都无法执行铁律 1（先写会失败的测试）。**
Fake Runtime 本身也必须有测试——它是测试的地基，地基歪了上面全歪。

---

## 2. 权威资料与核实状态 ★

**本节是全文的证据入口。** 下面每条结论都能指回一个 URL 或一个源码路径。

### 2.1 已核实的权威资料

| 来源 | 用途 |
|---|---|
| `https://agentclientprotocol.com/protocol/v1/overview` | 方法清单、Agent/Client 角色划分、`camelCase` 键 / `snake_case` 判别值约定 |
| `.../v1/initialization` | `initialize` 请求响应结构、`protocolVersion` 协商规则、能力字段 |
| `.../v1/session-setup` | `session/new` / `session/load` / `session/resume` / `session/close`、`cwd`、`mcpServers`、`additionalDirectories` |
| `.../v1/session-modes` | `SessionModeState` / `SessionMode` / `session/set_mode` / `current_mode_update` |
| `.../v1/prompt-turn` | 一轮的完整时序、`session/update` 变体、`StopReason` 全集、**取消的规范性要求** |
| `.../v1/cancellation` | `$/cancel_request`、错误码 `-32800`、级联取消示例 |
| `.../v1/tool-calls` | `tool_call` / `tool_call_update` 字段、`ToolKind` / 状态枚举、`session/request_permission` 全流程 |
| `.../v1/agent-plan` | `plan` 更新的 `entries` 结构 |
| `.../v1/authentication` | `authMethods` / `authenticate` / `logout` |
| `.../v1/transports` | **stdio 换行分帧**、stderr 用途 |
| `.../v1/schema` | `ErrorCode` 全集、`SessionUpdate` 全部 11 个判别值、`StopReason` 定义 |
| `https://agentclientprotocol.com/llms.txt` | 文档索引；**ACP v2 已有 Draft** |
| `github.com/agentclientprotocol/claude-agent-acp` `src/acp-agent.ts` | claude 侧真实能力与 mode id |
| `github.com/agentclientprotocol/codex-acp` `src/AgentMode.ts` `src/CodexAcpServer.ts` `src/ApprovalOptionId.ts` | codex 侧真实能力、mode 定义与默认值 |
| npm registry（`@agentclientprotocol/{claude-agent-acp,codex-acp,sdk}`） | 包名、bin 名、版本序列、`engines` |

### 2.2 与设计稿冲突 / 需要修正的地方 ★

| # | 设计稿（`design/`、`duet_app` 原型）说法 | 核实结果 | 处理 |
|---|---|---|---|
| **C1** | 「会话模式 `set_mode`，取值来自 `session/new` 返回的 `availableModes`」 | **[SPEC]** 完全正确 | 保持 |
| **C2** | 角色表里「实现工程师 · codex · `auto`」 | **[SRC]** `codex-acp` 只有 `read-only` / `agent` / `agent-full-access` 三个 mode（`src/AgentMode.ts`）。`auto` 是 **claude** 侧的 mode id | **冲突**。角色默认绑定必须改：codex 实现工程师应绑 `agent`。**修 UI 前先改设计规范条目（铁律 3）** |
| **C3** | 角色表里 claude 绑 `plan` / `default` | **[SRC]** `claude-agent-acp` 有 `plan`、`default`（显示名已改为 `Manual`，**wire id 仍是 `default`**） | 一致。UI 显示名取 `SessionMode.name`，**不要硬编码** |
| **C4** | `mem-188`「codex 默认权限档为 `agent`，需 `session/set_mode` 收权」+ 设置页「默认权限档 `agent`（不询问）」 | **[SRC]** 默认档确实是 `agent`（`AgentMode.DEFAULT_AGENT_MODE = AgentMode.Agent`）✓。但 `agent` 的 `approvalPolicy` 是 **`on-request`**、沙箱 `workspace-write`、`networkAccess: false`；`approvalPolicy: "never"`（真正"不询问"）的是 **`agent-full-access`** | **部分冲突**。「不询问」这句话**是错的**，会误导实现。正确表述见 §10.2。收权方向是 `agent → read-only`（给只读角色），不是"从不询问收到询问" |
| **C5** | 「探针 12/12 通过」「codex 探针 11/12 通过」 | **[SPEC]** 规范里没有"探针"这个概念，它是 Duet 自定义的检测集 | 不算冲突。12 项内容由本文 §8 定义（**[DUET]**）。**codex 具体失败哪一项：[待验证]**，见 §16-O3 |
| **C6** | 取消原型代码：`rpc.notify("session/cancel", ...)` → `await_stop_reason(10s)` | **[SPEC]** 一致：`session/cancel` 是 notification；Agent **MUST** 用 `stopReason: "cancelled"` 应答原 `session/prompt` | 一致。但设计稿**漏了两条客户端义务**，见 §6.3 |
| **C7** | 事件过滤器把「轮次结束 `stopReason`」列为一类 ACP 事件 | **[SPEC]** `stopReason` 是 `session/prompt` **响应的字段**，不是 notification | 语义差。`architecture.md` §4 的 `acp/turn_end` 是**应用侧合成事件**，不是线上收到的通知。见 §11 |
| **C8** | 事件枚举 `agent_thought_chunk` | **[SPEC]** 判别值一致 | 一致。注意应用侧事件类型叫 `thought_chunk`（去掉 `agent_` 前缀），两个名字不要混 |
| **C9** | 事件枚举「工具调用 `tool_call · update`」 | **[SPEC]** 是**两个独立变体** `tool_call` 与 `tool_call_update` | 需在映射层合并成一类应用事件，见 §11.2 |
| **C10** | `protocolVersion 1` | **[SRC]** 两个 adapter 都返回 `1`（`acp-agent.ts:1562`、`CodexAcpServer.ts:221`）。**但 ACP v2 已有 Draft**（`v2.0.0-alphaX`） | 一致。`architecture.md` §8-2 的开放项仍然开放，见 §16-O1 |
| **C11** | Runtime 装到 `~/.acpflows/runtimes/<name>/<version>/` | **[SRC]** npm 包 `@agentclientprotocol/claude-agent-acp`（bin `claude-agent-acp`，`engines.node >= 22`）、`@agentclientprotocol/codex-acp`（bin `codex-acp`，内含 `@openai/codex` 依赖） | 一致。**注册表必须额外记 node 版本要求**，见 §9 |
| **C12** | 版本 `claude-agent-acp 0.63.0 → 0.64.1`、`codex-acp 1.1.7` | npm 上这些版本真实存在 | 一致。设计稿数据是真的，不是编的 |
| **C13** | —（设计稿未提及） | **[SPEC]** `session-modes` 页顶挂了废弃告示：*Dedicated session mode methods will be removed in a future version of the protocol*，改用 **Session Config Options**（`session/set_config_option` + `config_option_update`） | **新增风险**。`codex-acp` 已经**同时**暴露 `SessionModeState` 和一个 `id: "mode"` 的 `SessionConfigOption`（`AgentMode.toConfigOption()`）。见 §16-O2 |

> **C2 与 C4 是必须在写代码前解决的两条。** 它们直接决定角色默认绑定表的取值，
> 而角色绑定是 8 个预置角色的地基。按铁律 3：**先改设计规范条目，再改实现。**

---

## 3. 包结构

### 3.1 目录

```
backend/internal/acp/
├── AGENTS.md  CLAUDE.md          ← 关键目录，必须成对存在（AGENTS.md §4.1）
├── acp.go                        包门面：实现 app 定义的 RuntimeGateway port
├── errors.go                     哨兵错误（ErrCancelTimeout / ErrProtocolMismatch …）
│
├── protocol/                     ★ 线格式：类型 + 枚举 + 编解码。零 IO
│   ├── initialize.go             InitializeRequest / InitializeResponse / Capabilities
│   ├── session.go                NewSession* / LoadSession* / SetSessionMode* / Prompt*
│   ├── update.go                 SessionNotification + 11 个 SessionUpdate 变体
│   ├── tool_call.go              ToolCall / ToolCallUpdate / ToolKind / ToolCallStatus
│   ├── permission.go             RequestPermission* / PermissionOption / Outcome
│   ├── content.go                ContentBlock 五种类型
│   ├── stop_reason.go            StopReason 枚举 + IsValid + String
│   └── error_code.go             ErrorCode 枚举（-32700 … -32000 / -32002 / -32800）
│
├── jsonrpc/                      ★ JSON-RPC 2.0 over 双工字节流。不认识 ACP 语义
│   ├── conn.go                   Conn：双向请求 / 通知 / id 分配 / pending 表
│   ├── frame.go                  换行分帧的读写器
│   ├── message.go                Request / Response / Notification / Error
│   └── cancel.go                 $/cancel_request 收发
│
├── session/                      ★ 会话状态机：生命周期、两段式取消、权限裁决入口
│   ├── session.go                Session 本体
│   ├── lifecycle.go              initialize → new → set_mode → prompt
│   ├── cancel.go                 ★ 两段式取消（unit-012 的落点）
│   ├── permission.go             request_permission 应答策略执行
│   └── stream.go                 session/update → acp.Event 的翻译与派发
│
├── capability/                   ★ 12 项探针 + 能力矩阵
│   ├── probe.go                  ProbeID 枚举 + Probe 接口
│   ├── matrix.go                 Matrix：探针结果聚合、降级判定
│   └── probes.go                 12 个探针的具体实现
│
├── runtime/                      ★ 子进程生命周期 + Runtime 注册表 + 版本管理
│   ├── process.go                spawn / 健康检查 / 优雅关闭 / 崩溃重启 / 僵尸清理
│   ├── registry.go               已安装 Runtime 的发现、版本、路径、登录态
│   ├── install.go                npm 安装到 ~/.acpflows/runtimes/<name>/<version>/
│   └── activate.go               多版本并存与默认版本切换
│
├── adapter/                      ★ 两个 Runtime 的差异收敛在这里，别处不许出现 if runtime=="codex"
│   ├── adapter.go                Adapter 接口
│   ├── base.go                   ★ 共同实现，被 claude/codex 嵌入（design-principles.md §4.1）
│   ├── claude/claude.go
│   └── codex/codex.go
│
└── fake/                         ★ Fake ACP Runtime（M0 第一个交付物）
    ├── runtime.go  script.go  builder.go  recorder.go  capability.go  fault.go
    ├── cmd/fakeacp/main.go       编成真子进程，给 e2e 用
    └── testdata/                 脚本 + golden JSON
```

> 文件命名 `snake_case.go`、单文件 ≤ 400 行、禁止 `util.go`/`helper.go`/`common.go`/`misc.go`
> —— 见 [`coding-standards.md`](../rules/coding-standards.md) §1.3、§2。

### 3.2 每个子包负责什么 / 不负责什么

| 包 | 负责 | **明确不负责** |
|---|---|---|
| `protocol` | ACP 线格式类型、枚举、`IsValid()`、JSON 编解码、未知变体的宽松处理 | 任何 IO；任何状态；不认识"会话正在进行中"这种概念 |
| `jsonrpc` | 分帧、id 分配、pending 表、双向请求、通知、超时、`$/cancel_request` | 不认识 `session/*`；不认识 `stopReason`；不 spawn 进程 |
| `session` | 一个会话的完整生命周期、取消状态机、权限应答、`session/update` → `acp.Event` | 不 spawn 进程；不认识 Work/Unit/Contract；不分配 `seq` |
| `capability` | 12 项探针执行、能力矩阵生成、降级判定 | 不做 UI 文案；不决定"探针不过要不要拦住用户"（那是 app 层策略） |
| `runtime` | 子进程生命周期、注册表、安装、版本切换 | 不解析 ACP 消息；不下载 App 安装包（那是 Tauri updater） |
| `adapter/*` | 收敛两个 Runtime 的启动参数、环境变量、mode id 映射、已知怪癖 | 不写业务判断；不改变 `session` 的状态机语义 |
| `fake` | 可编排的假 Agent | **不 import `session` / `runtime` / `adapter`**（见下） |

### 3.3 包内依赖方向（单向，反向一律拒绝）

```
                       acp (门面, 实现 app 的 RuntimeGateway port)
                        │
            ┌───────────┼────────────┐
            ▼           ▼            ▼
        runtime    capability    adapter/{claude,codex}
            │           │            │
            └───────────┴─────┬──────┘
                              ▼
                          session
                              │
                              ▼
                          jsonrpc
                              │
                              ▼
                          protocol   ◀────────── fake
```

**两条硬规则，由 `depguard` 强制：**

1. `protocol` 不 import 本包内任何其他子包，也不做 IO。
2. **`fake` 只允许 import `protocol`。**
   如果 Fake 复用了 `session` 或 `jsonrpc` 的实现，测试就变成"用被测代码验证被测代码"——
   这正是 `testing-strategy.md` §3.1「mock 喂 mock」的变体，只是伪装得更好。
   Fake 必须**独立地**按规范说话，才有资格当参照物。

---

## 4. JSON-RPC over stdio 传输层

### 4.1 分帧 **[SPEC]**

- 客户端把 Agent 拉起为**子进程**；Agent 读 `stdin`、写 `stdout`。
- 消息以 `\n` 分隔，**每条消息是单行完整 JSON-RPC 对象，MUST NOT 包含内嵌换行**。
- **不是 LSP 的 `Content-Length` 头。** 写实现前不要凭 LSP 经验想当然。
- Agent **MAY** 往 `stderr` 写 UTF-8 日志；客户端可捕获也可忽略。
- `stdin`/`stdout` 上**只允许**流通合法 ACP 消息。

**实现约束（[DUET]）：**

| 项 | 决定 |
|---|---|
| 编码器 | `json.Encoder` 但**必须关掉 HTML 转义**并保证单行；实际用 `json.Marshal` + 手写 `\n`，避免 Encoder 的隐式换行行为被误读 |
| 解码器 | `bufio.Scanner` 会在超长行上静默截断——**禁止使用**。用 `bufio.Reader.ReadBytes('\n')`，行长上限 `constant.ACPMaxFrameBytes`（初值 16 MiB），超限 → `ErrFrameTooLarge` 并断开 |
| 非 JSON 行 | 记 warn 日志 + 丢弃，**不断开**。某些 Runtime 会误往 stdout 写东西；断开会把用户的一轮工作直接打死 |
| stderr | **必须**独立 goroutine 持续排空并转存到 `<project>/.acpflows/runs/<runId>/<runtime>.stderr.log`。不排空 = 子进程写日志写满管道缓冲区后死锁 |

### 4.2 双向请求

同一条流上双向跑请求。方向与发起方：

```
  duetd (Client)                                   Runtime (Agent)
       │                                                  │
       │──── request: initialize / session/new ──────────▶│
       │      session/set_mode / session/prompt           │
       │      session/load / session/cancel(通知)         │
       │                                                  │
       │◀─── request: session/request_permission ─────────│
       │      fs/read_text_file / fs/write_text_file      │
       │      terminal/* / elicitation/create             │
       │      $/cancel_request (通知)                      │
       │                                                  │
       │◀─── notification: session/update ────────────────│
```

**id 管理的坑（必须写测试守住）：**

JSON-RPC 2.0 的请求 id 由**发起方**分配。两个方向各自独立编号，
所以 **客户端发出的 `id=5` 和 Agent 发来的 `id=5` 是两个毫不相干的请求**。

```go
// jsonrpc/conn.go
type Conn struct {
    nextID   atomic.Int64                  // 只用于本端发出的请求
    pending  map[int64]chan *Response      // 只装本端发出的请求，key 是本端分配的 id
    inbound  map[string]Handler            // 处理对端发来的请求，按 method 分派
    // ...
}
```

`pending` **只**索引本端发出的请求。对端发来的请求走 `inbound` 分派，
两者共用一张 map 就会互相踩——这是这层最容易写错的地方，
`TestConn_InboundAndOutboundIDsDoNotCollide` 必须先红过。

### 4.3 超时

超时按方法分档，取值集中在 `internal/constant/limit.go`，**不许散落在调用点**：

| 方法 | 超时 | 超时后 |
|---|---|---|
| `initialize` | 15s | `ErrInitializeTimeout` → 判定 Runtime 不可用 |
| `session/new` | 30s | `ErrSessionNewTimeout` → Work 进入 `failed` |
| `session/set_mode` | 10s | `ErrSetModeTimeout` → 降级到默认 mode 并记 warn |
| `session/prompt` | **无整体超时** | 一轮可以跑几十分钟，超时只能靠取消 |
| `session/prompt` 的**静默**超时 | 无任何 `session/update` 或响应达 `ACPIdleTimeout`（初值 300s） | 记 `stall` 事件，**不自动取消**，交由 app 层决定 |
| **取消完成** | `ACPCancelTimeout`（初值 **30s**，见 §6.4） | `ErrCancelTimeout` → 更新 `prepare` 返回 `blocked` |
| `session/request_permission` 的应答 | 由 app 层裁决策略决定，协议层不设 | —— |

> 设计稿原型代码里写的是 `Duration::from_secs(10)`。**[DUET]** 我们取 30s：
> 真实 Runtime 在取消时要中止 LLM 请求 + 中止工具调用 + 补发 pending 更新，10s 偏紧。
> 具体值 **[待验证]**，接真实 Runtime 后按实测 p99 回填，见 §16-O4。

### 4.4 背压

子进程的 `stdout` 是管道。**我们不读，它就阻塞在 write 上。**

```
Runtime.stdout ──▶ [reader goroutine] ──▶ chan *Message (容量 ACPInboundBuffer=256)
                                              │
                                              ▼
                                        [dispatch goroutine]
                                              │
                              ┌───────────────┴───────────────┐
                              ▼                               ▼
                     pending 响应回填              session.stream → acp.Event
```

**规则：**

1. reader goroutine **只做分帧 + 反序列化**，绝不做业务处理。任何耗时操作都会立刻回压到子进程。
2. `chan` 满时 **阻塞，不丢弃**。`session/update` 承载事件流，
   丢一条就意味着 `seq` 出现空洞——而"取消后最后事件游标可读"是产品硬需求
   （`architecture.md` §4），空洞会让这个需求直接失效。
3. 阻塞超过 `ACPBackpressureWarn`（初值 5s）→ 记一条 `stall` 诊断（**不是**事件流里的事件）。
   持续超过 `ACPBackpressureFatal`（初值 60s）→ 判定为下游失活，主动断开并走崩溃恢复。

### 4.5 子进程生命周期

```
                 ┌───────────┐
     spawn ─────▶│ starting  │
                 └─────┬─────┘
                       │ initialize 成功且 protocolVersion 匹配
                       ▼
                 ┌───────────┐   session/new       ┌───────────┐
                 │  ready    │──────────────────▶  │  serving  │
                 └─────┬─────┘                     └─────┬─────┘
                       │                                 │
                       │  ctx 取消 / 用户关闭              │ stdout EOF / 进程退出
                       ▼                                 ▼
                 ┌───────────┐                     ┌───────────┐
                 │ stopping  │                     │  crashed  │
                 └─────┬─────┘                     └─────┬─────┘
                       │ 进程退出 + wait 回收               │ 上报 + 决定是否重启
                       ▼                                 │
                 ┌───────────┐◀────────────────────────┘
                 │  stopped  │
                 └───────────┘
```

| 阶段 | 做什么 |
|---|---|
| **启动** | 用绝对路径 exec（不走 `PATH`，避免用户环境漂移）；`cwd` 设为 worktree 路径；显式白名单环境变量（见 §9.4）；三个管道全部接上 |
| **健康检查** | 唯一判据是 `initialize` 在 15s 内返回且 `protocolVersion` 可接受。**不做 ping**——协议里没有 ping |
| **优雅关闭** | ① 对每个活跃会话发 `session/cancel` 并等 `stopReason`（§6）→ ② 若 Agent 声明 `sessionCapabilities.close` 则调 `session/close` **[SPEC]** → ③ 关 `stdin`（EOF 是 stdio Agent 的标准退出信号）→ ④ 等待 `ACPGracefulExit`（初值 10s）→ ⑤ `SIGTERM` → 再等 5s → ⑥ `SIGKILL` |
| **崩溃重启** | `stdout` EOF 或 `Wait()` 返回即判定 crashed。**协议层不自动重启**——它不知道当前 Unit 能不能安全重放。上报 `ErrRuntimeCrashed` + 退出码 + 最后 64 KiB stderr，由 app 层决定。见 §14 |
| **僵尸清理** | 每个子进程**必须**有一个 goroutine 无条件 `cmd.Wait()`，否则留下 zombie。进程组用 `Setpgid: true` 建独立进程组，`SIGKILL` 打整个进程组（`codex-acp` 会再拉起 Codex App Server 子进程，只杀父进程会漏） |
| **启动时孤儿回收** | duetd 启动时扫 `~/.acpflows/runtime/orphans.json`（记录上次运行的子进程 pid + 启动时刻），逐个校验 `/proc` 等价信息后清理。**禁止**按进程名模糊匹配杀进程——会误杀用户自己开的 `claude-agent-acp` |

---

## 5. 会话生命周期

### 5.1 全景时序（ASCII） **[SPEC]**

```
 duetd (Client)                                              Runtime (Agent)
      │                                                             │
      │  ── 1. 初始化 ───────────────────────────────────────────    │
      │─── initialize {protocolVersion:1, clientCapabilities} ─────▶ │
      │◀── result {protocolVersion:1, agentCapabilities,             │
      │            agentInfo, authMethods}                           │
      │                                                             │
      │       [若返回 -32000 Authentication required]                │
      │─── authenticate {methodId} ────────────────────────────────▶ │
      │◀── result {}                                                 │
      │                                                             │
      │  ── 2. 建会话 ───────────────────────────────────────────    │
      │─── session/new {cwd:"<worktree 绝对路径>", mcpServers:[]} ──▶ │
      │◀── result {sessionId, modes:{currentModeId, availableModes}} │
      │            └── modes 是 OPTIONAL，可能为 null                 │
      │                                                             │
      │  ── 3. 收权（可选，取决于角色绑定） ─────────────────────      │
      │─── session/set_mode {sessionId, modeId} ──────────────────▶  │
      │◀── result {}                                                 │
      │                                                             │
      │  ── 4. 一轮 ─────────────────────────────────────────────    │
      │─── session/prompt {sessionId, prompt:[ContentBlock…]} ────▶  │  id=N
      │                                                             │
      │◀── notify session/update {plan}                              │
      │◀── notify session/update {agent_message_chunk, messageId}    │
      │◀── notify session/update {agent_thought_chunk}               │
      │◀── notify session/update {tool_call, toolCallId, pending}    │
      │                                                             │
      │◀── request session/request_permission {toolCall, options} ── │  id=M
      │             ★ 阻塞当前轮，直到本端应答                        │
      │─── result {outcome:{outcome:"selected", optionId}} ───────▶  │
      │                                                             │
      │◀── notify session/update {tool_call_update, in_progress}     │
      │◀── notify session/update {tool_call_update, completed,       │
      │                           content:[{type:"diff",…}]}         │
      │◀── notify session/update {usage_update}                      │
      │                                                             │
      │◀── result(id=N) {stopReason:"end_turn"} ─────────────────    │
      │       ★ 一轮到此才算结束                                      │
      │                                                             │
      │  ── 5. 下一轮 or 关闭 ───────────────────────────────────    │
      │─── session/prompt … (复用同一个 sessionId)                   │
      │─── session/close {sessionId}   [需 sessionCapabilities.close] │
```

### 5.2 各步的硬约束

| 步骤 | 约束 |
|---|---|
| `initialize` | **[SPEC]** 客户端 MUST 送自己支持的**最新**版本。Agent 支持就 MUST 回同一个值，否则回它支持的最新值。客户端不支持 Agent 回的版本 → **SHOULD 关连接并告知用户**。见 §14.2 |
| 能力协商 | **[SPEC]** `initialize` 请求里省略的能力 MUST 被当作**不支持**。所以 Duet 必须显式声明 `fs.readTextFile` / `fs.writeTextFile` / `terminal`，声明了就必须真的实现对应的 handler |
| `session/new` | **[SPEC]** `cwd` MUST 是绝对路径，且 MUST 作为会话的根之一，与子进程实际启动目录无关。**[DUET]** 一律传 worktree 路径 `~/.duet/worktrees/<project>/<work>` |
| `modes` | **[SPEC]** `NewSessionResponse.modes` 是 **OPTIONAL**（`SessionModeState \| null`）。为 null 时**不许调 `session/set_mode`**，角色的会话模式配置降级为"不适用"并在 UI 上标灰 |
| `session/set_mode` | **[SPEC]** `modeId` MUST 是 `availableModes` 里的值。**[SRC]** 两个 adapter 对非法值都抛错（claude：`Mode X is not available in this session`），不是静默忽略 |
| `session/prompt` | **[SPEC]** 内容类型 MUST 受 `promptCapabilities` 约束。文本与 `resource_link` 是所有 Agent 的基线；`image` / `audio` / `embeddedContext` 要先看能力 |
| 并发 | **[DUET]** 同一 `sessionId` **同时只允许一轮 `session/prompt` 在飞**。第二次调用在协议层直接返回 `ErrTurnInFlight`，不下发。多 Work 并发靠多进程 / 多会话，不靠一个会话并发 |

### 5.3 `availableModes` 不能硬编码 **[SRC]**

`claude-agent-acp` 的 `buildAvailableModes()` 是**按模型动态构造**的：

- `auto` 只有在 `ModelInfo.supportsAutoMode === true` 时才出现；
- `bypassPermissions` 受 `ALLOW_BYPASS` 开关约束（**以 root 运行时不可用**）；
- 会话创建时若解析出的 `permissionMode` 不在 `availableModes` 里，adapter 会**自行钳到 `default`**。

**结论：`availableModes` 是每会话、每模型都可能不同的运行期数据。**
角色配置里存的 `modeId` 必须在每次建会话后与实际 `availableModes` 求交集，
不在集合里就降级到 `currentModeId` 并发一条 `app/state_change` 说明降级原因。
把 mode 列表写进常量表 = 在用户换模型的那天炸掉。

---

## 6. 两段式取消 ★

> 这是本层最重要的一节。它是 `unit-012` 的核心验收标准，
> 也是 M1 自动更新 `POST /v1/system/update/prepare` 的直接依赖
> （`release-and-update.md` §5：`executing` 的 Work 要"发 ACP `session/cancel`（两段式：发请求 → 等 `stopReason` 落盘）"）。

### 6.1 为什么必须两段

**[SPEC]** `session/cancel` 是 **notification**——JSON-RPC 通知没有响应。
发出去只代表"字节写进了管道"，**不代表 Agent 停了、更不代表现场证据完整**。

规范原文（`/protocol/v1/prompt-turn` 取消一节）明确要求：

> After all ongoing operations have been successfully aborted and pending updates have been sent,
> the Agent **MUST** respond to the original `session/prompt` request with the `cancelled` stop reason.

以及：

> The Agent **MAY** send `session/update` notifications with content or tool call updates after
> receiving the `session/cancel` notification, but it **MUST** ensure that it does so
> **before** responding to the `session/prompt` request.

也就是说：**`session/prompt` 的响应是"现场已经补齐"的唯一同步点。**
在它到达之前落检查点，会丢掉 Agent 在取消窗口里补发的 `tool_call_update`（含 diff），
证据就残缺了——而铁律 5 要求证据由应用直接采集且完整。

```
   段 1：发出取消                          段 2：等待落地
   ─────────────────────                  ────────────────────────────────
   写 session/cancel 通知                  持续接收 session/update（可能还有！）
   本地预标记 tool_call = cancelled         应答所有 pending request_permission
   ↓ 立即返回，不阻塞 UI                     ↓ 直到 session/prompt 响应到达
   状态：cancel_requested                  状态：cancel_settled  ← 只有到这里才允许落检查点
```

### 6.2 时序（ASCII）

```
 app 层            session.Cancel()            jsonrpc                Runtime
   │                    │                         │                      │
   │─── Cancel(ctx) ───▶│                         │                      │
   │                    │ ① CAS: cancelRequested  │                      │
   │                    │    false→true           │                      │
   │                    │    已是 true → 跳到 ④    │                      │
   │                    │                         │                      │
   │                    │─ notify session/cancel ▶│───────────────────▶  │
   │                    │                         │                      │
   │                    │ ② 本地预标记：所有未完成    │                      │
   │                    │    tool_call → cancelled│                      │
   │                    │                         │                      │
   │                    │ ③ 对所有 pending 的        │                      │
   │                    │    request_permission 回  │                      │
   │                    │    outcome="cancelled"  │─────────────────────▶│
   │                    │                         │                      │
   │                    │                         │◀─ session/update ────│ 允许！
   │                    │                         │   (tool_call_update) │ 必须收下
   │                    │                         │◀─ session/update ────│
   │                    │                         │                      │
   │                    │ ④ 等 promptDone 通道      │◀─ result(id=N)       │
   │                    │    （幂等路径也等这里）     │   stopReason:        │
   │                    │                         │   "cancelled"        │
   │                    │◀────────────────────────│                      │
   │                    │ ⑤ 持久化游标 + 落 stopReason                      │
   │◀── nil ────────────│                         │                      │
   │                    │                         │                      │
   │  超时分支：ctx 到 ACPCancelTimeout 仍无响应                             │
   │◀── ErrCancelTimeout│                         │                      │
```

### 6.3 客户端的三条协议义务（设计稿漏了两条） **[SPEC]**

发出 `session/cancel` 之后，客户端**不是**只等就行：

| # | 义务 | 规范措辞 | 漏了会怎样 |
|---|---|---|---|
| 1 | 预先把本轮所有未完成的 `tool_call` 在**本地**标记为 `cancelled` | SHOULD | UI 里工具调用永远转圈 |
| 2 | **对所有 pending 的 `session/request_permission` 用 `{"outcome":{"outcome":"cancelled"}}` 应答** | **MUST** | Agent 卡在等权限上，永远不回 `stopReason` → 每次取消都超时 → `prepare` 永远 `blocked` |
| 3 | 继续接收并接受 `session/cancel` 之后到达的 `tool_call_update` | SHOULD | 丢掉取消窗口内补发的 diff，证据残缺 |

**第 2 条是最容易漏、后果最严重的一条。** 必须有一条先红的测试锁住它：

```go
// R3 附属：取消时必须解掉在飞的 request_permission，否则 stopReason 永不到达
func TestSessionCancel_ResolvesPendingPermissionAsCancelled(t *testing.T) { ... }
```

Fake Runtime 的 `AskPermission` 步骤配合 `CancelBehavior{WaitPermission: true}`
就是专门用来让这条测试**真的会红**的（§12.6）。

### 6.4 幂等 ★

验收标准 **R3：连续取消两次只发送一次协议取消请求。**

```go
// internal/acp/session/cancel.go

// Cancel 请求取消当前轮，并阻塞到该轮的 stopReason 落盘。
//
// 幂等：并发或连续调用只会向 Runtime 发送一次 session/cancel 通知；
// 后续调用不再发通知，但同样等待同一个 stopReason，因此所有调用方
// 都能观察到一致的完成语义（要么都成功，要么都拿到 ErrCancelTimeout）。
func (s *Session) Cancel(ctx context.Context) error {
    if !s.cancelRequested.CompareAndSwap(false, true) {
        return s.awaitSettled(ctx)      // ← 幂等路径：不发通知，但仍然等
    }
    if err := s.conn.Notify(ctx, protocol.MethodSessionCancel,
        protocol.CancelParams{SessionID: s.id}); err != nil {
        s.cancelRequested.Store(false)  // 通知没发出去，允许重试
        return fmt.Errorf("notify session/cancel %s: %w", s.id, err)
    }
    s.markInflightToolCallsCancelled()
    s.resolvePendingPermissions(protocol.PermissionOutcomeCancelled)
    return s.awaitSettled(ctx)
}
```

**三个必须写进测试的语义（缺一条就不算幂等做对了）：**

| 语义 | 断言方式 |
|---|---|
| 只发一次协议请求 | `fake.CountMethod("session/cancel") == 1` |
| **第二次调用也要等**，不能立刻返回 nil | 第二个 goroutine 在 `stopReason` 到达前不返回 |
| 通知**写失败**时允许重试 | 注入 `FaultCloseStdout`，断言 `cancelRequested` 被回滚 |

第二条最容易做错。"幂等 = 第二次直接 `return nil`"是**错的**：
`prepare` 会对同一个 Work 连续调两次，如果第二次立刻返回 nil，
调用方会以为现场已落盘而去读检查点——那时 `stopReason` 还没到，读到的是残缺证据。

> 设计稿原型代码 `if self.cancelled.swap(true) { return Ok(()); }` 就是这个错误版本。
> **[DUET]** 我们的实现必须是"第二次也等"。

### 6.5 超时与降级

```
awaitSettled 的三种出口
├── stopReason 到达（"cancelled" 或其他值）
│      → 持久化游标 → 返回 nil
│      注：Agent 可能回 "end_turn"（取消发出前它已经自然结束了）
│         这不是错误，一样算 settled
├── ctx 超时 / 到达 ACPCancelTimeout
│      → 返回 ErrCancelTimeout
│      → app 层：prepare 该 Work 进 blocked，reason="cancel 超时，Runtime 无响应"
│      → UI：让用户选「强制更新（丢弃 work-11 的 2:14 工作）」或「稍后」
└── 连接断开（Runtime 崩溃）
       → 返回 ErrRuntimeCrashed
       → app 层：视为 settled（进程都没了，不会再有更新），但证据标记为 partial
```

`ErrCancelTimeout` 是 `release-and-update.md` §5 里 `blocked` 的**唯一来源**，
它必须是一个可被 `errors.Is` 判定的哨兵错误，不是字符串比较。

---

## 7. 权限裁决

### 7.1 协议形态 **[SPEC]**

`session/request_permission` 是 **Agent → Client 的请求**（有 id、要应答）：

```json
{"jsonrpc":"2.0","id":5,"method":"session/request_permission","params":{
  "sessionId":"sess_abc","toolCall":{"toolCallId":"call_001"},
  "options":[{"optionId":"allow-once","name":"Allow once","kind":"allow_once"},
             {"optionId":"reject-once","name":"Reject","kind":"reject_once"}]}}
```

应答两种形态：

```json
{"jsonrpc":"2.0","id":5,"result":{"outcome":{"outcome":"selected","optionId":"allow-once"}}}
{"jsonrpc":"2.0","id":5,"result":{"outcome":{"outcome":"cancelled"}}}
```

`PermissionOption.kind` 是 `allow_once` / `allow_always` / `reject_once` / `reject_always`，
**[SPEC]** 明确说它是"给客户端挑图标和 UI 处理方式的提示"。
**`optionId` 才是应答时必须回传的值，`kind` 只是提示。**

> **陷阱：不要按 `kind` 应答。** **[SRC]** `codex-acp` 的 `ApprovalOptionId` 里有
> `accept_execpolicy_amendment` / `apply_network_policy_amendment` /
> `allow_permissions_turn` / `allow_permissions_session` / `reject_permissions`
> 这些 **id**，它们的 `kind` 会落到四个标准值上，但语义各不相同。
> 按 `kind` 选项会选错。**必须按 `optionId` 原样回传。**

**[SPEC]** `options` 数组是 Agent 给的，**内容与顺序都不固定**。
客户端不许假设"第 0 个是允许"。裁决策略必须在 `options` 上做匹配，匹配不上就走保守分支。

### 7.2 三种裁决策略（对应角色配置） **[DUET]**

设计稿的角色表里「权限裁决」列有两个取值：`逐条询问` / `自动允许读`。
我们再补一个 `拒绝`，凑成协议层需要的完整三态：

| 策略 | `PolicyID` | 行为 | 绑定的角色（设计稿） |
|---|---|---|---|
| 逐条询问 | `ask_each` | 每个请求都生成一条 `acp/request_permission` 事件，**阻塞当前轮**，等用户点 | 需求分析师 · 计划架构师 · 单元设计师 · 实现工程师 |
| 自动允许读 | `auto_allow_read` | `toolCall.kind ∈ {read, search, fetch, think}` → 自动选 `allow_once` 类的选项；其余走 `ask_each` | 测试执行者 · 实现审查员 · 决策顾问 · 记忆管理员 |
| 拒绝 | `deny` | 一律选 `reject_once` 类的选项，并发一条 `acp/request_permission` 事件（已裁决态，只做记录） | 无默认绑定；用于"只读探查"类临时会话 |

```go
// internal/acp/session/permission.go

// Adjudicator 决定如何应答一次 session/request_permission。
// 实现必须是纯决策：不落库、不发 HTTP、不阻塞在 IO 上。
// 需要人来点的场景由 ask_each 返回 DecisionAskUser，交给 app 层去驱动 UI。
type Adjudicator interface {
    Adjudicate(req protocol.RequestPermissionRequest) Decision
}

type Decision struct {
    Kind     DecisionKind  // DecisionSelect | DecisionAskUser
    OptionID string        // Kind==DecisionSelect 时必填，原样回传
    Reason   string        // 写进事件 payload，用于 UI 与审计
}
```

**`auto_allow_read` 的匹配规则（必须有穷举测试）：**

```
① toolCall.kind 不在 {read, search, fetch, think} → 交给 ask_each
② 在 options 里找第一个 kind == "allow_once" 的项 → 选它
③ 找不到 allow_once，找 kind == "allow_always" 的项 → 选它，但记 warn
   （allow_always 会让 Agent 记住选择，比我们想要的范围大）
④ 两个都没有 → 交给 ask_each。绝不"猜一个 optionId"
```

第 ④ 条是保守分支。**协议层永远不许在选项集合不认识时替用户拍板。**

### 7.3 「阻塞当前轮」到底阻塞了什么

`architecture.md` §4 把 `request_permission` 标为「阻塞当前轮」。精确语义是：

```
阻塞的是：这一个 session/prompt 请求的推进
         （Agent 在等我们应答，它不会继续跑这个工具）
不阻塞：  ① duetd 的 HTTP API（用户照样能看计划、看证据、开别的 Work）
         ② 同一个 Runtime 上的其他 session（如果有）
         ③ 事件流本身（SSE 连接不断，还能收到别的 Work 的事件）
```

设计规范禁止「用弹窗打断执行中的单元来展示非阻塞信息」（`AGENTS.md` §9），
但 `request_permission` **是**阻塞信息，它的描边卡 + 「允许一次 / 拒绝」是合规的。

**超时？协议层不设。** 权限请求可以挂几个小时等用户回来——这是产品语义，不是故障。
唯一会解掉它的是取消（§6.3 第 2 条）或连接断开。

### 7.4 与角色配置的关系

```
Role（8 个预置角色，先定义）
  ├── runtime_name       claude | codex        → runtime.Registry 解析成可执行路径
  ├── mode_id            plan | default | …    → session/set_mode（先与 availableModes 求交）
  └── permission_policy  ask_each | auto_allow_read | deny  → session.Adjudicator
```

**角色先于 Runtime**（设计稿原话）：同一个角色换绑另一端不影响状态机。
所以 `permission_policy` 存在角色上，不存在 Runtime 上；
Runtime 只提供 `options`，策略由角色决定怎么挑。

---

## 8. 能力探针

### 8.1 12 项探针 **[DUET]**

数量 12 由设计稿的设置页写死（`探针 12/12 通过`）。内容由本文定义。
**每项探针都必须是一次真实的协议往返，不许靠"包版本号猜能力"。**

| # | `ProbeID` | 探什么 | 通过判据 | 不过的降级 |
|---|---|---|---|---|
| 01 | `spawn` | 可执行文件能拉起，stdout 出现合法 JSON-RPC 行 | 进程存活 ≥ 1s 且至少一行可解析 | **硬失败**：Runtime 标记 `unavailable`，不允许绑定角色 |
| 02 | `initialize` | `initialize` 在 15s 内返回 | 收到 result | **硬失败**：同上 |
| 03 | `protocol_version` | 协商结果可接受 | `protocolVersion == 1` | **硬失败**：见 §14.2 |
| 04 | `agent_info` | 返回 `agentInfo.name` / `version` | 两个字段非空 | 软失败：注册表版本列显示"未知"，其余功能不受影响 |
| 05 | `auth_ready` | 无需交互即可建会话（= 已登录） | `session/new` 不返回 `-32000` | 软失败：Runtime 标 `未登录`，UI 引导登录，**不允许绑定角色** |
| 06 | `session_new` | `session/new` 返回 `sessionId` | 非空字符串 | **硬失败** |
| 07 | `session_modes` | `session/new` 返回非空 `modes.availableModes` | `len ≥ 1` | 软失败：角色的会话模式配置置灰不可用（§5.2） |
| 08 | `set_mode` | 切到 `availableModes[0]` 成功 | result 无错误 | 软失败：同 07 |
| 09 | `prompt_stop_reason` | 一次极小 prompt 能拿到 `stopReason` | 收到 result 且 `StopReason.IsValid()` | **硬失败** |
| 10 | `message_stream` | 收到 ≥ 1 条 `agent_message_chunk` | 计数 ≥ 1 | 软失败：`message_chunk` 事件不可用，UI 提示"该 Runtime 不流式输出正文" |
| 11 | `tool_call_stream` | 探针 prompt 能触发 `tool_call` + `tool_call_update` | 两个变体各 ≥ 1 | 软失败：证据采集降级为只用 git diff，不用 `tool_call.content` |
| 12 | `cancel_settles` | ★ 发 `session/cancel` 后在 `ACPCancelTimeout` 内拿到 `stopReason` | 收到 result | **硬失败**：不满足这条的 Runtime 会让每次更新 `prepare` 都 `blocked`，等于不可用 |

**探针 12 是唯一"看起来可选、实际必需"的一项。** 它不过就意味着 M1 自动更新对这个 Runtime 永远不可用。

### 8.2 探针不作为独立项、但要记进能力矩阵的东西

这些是**可选能力**，缺了不算探针失败，但要落进矩阵供上层查询：

| 能力键 | 来源 | 谁在用 |
|---|---|---|
| `thought_stream` | 是否收到 `agent_thought_chunk` | 事件流「思考摘要」类型 |
| `prompt.image` / `prompt.audio` / `prompt.embeddedContext` | `agentCapabilities.promptCapabilities` | 「粘贴图片/截图」附件功能 |
| `load_session` / `session.resume` / `session.close` / `session.list` / `session.delete` / `session.fork` | `agentCapabilities.loadSession` / `sessionCapabilities.*` | 检查点恢复的实现选型（§16-O5） |
| `mcp.http` / `mcp.sse` | `agentCapabilities.mcpCapabilities` | 项目 MCP Server 接入 |
| `auth.logout` | `agentCapabilities.auth.logout` | 设置页「退出登录」按钮是否显示 |
| `additional_directories` | `sessionCapabilities.additionalDirectories` | 「选择文件夹…只读挂载」附件功能 |

### 8.3 能力矩阵

```go
// internal/acp/capability/matrix.go

type Outcome string
const (
    OutcomePass Outcome = "pass"
    OutcomeFail Outcome = "fail"
    OutcomeSkip Outcome = "skip"   // 前置探针硬失败后，后续项不再执行
)

type Result struct {
    ID       ProbeID
    Outcome  Outcome
    Detail   string          // 失败时的可读原因，直接进 UI「能力矩阵」抽屉
    Evidence json.RawMessage // ★ 实际收到的报文片段，脱敏后留存
    Elapsed  time.Duration
}

type Matrix struct {
    RuntimeName string
    Version     string
    ProbedAt    time.Time
    Results     []Result           // 固定 12 条，顺序固定
    Optional    map[string]bool    // §8.2 的可选能力
}

func (m Matrix) Passed() int                     // UI 上的 "11/12" 里的 11
func (m Matrix) Total() int                      // 恒为 12
func (m Matrix) HasHardFailure() bool            // 任一硬失败项 fail
func (m Matrix) Degradations() []Degradation     // 软失败 → 具体降级动作清单
```

**规则：**

1. 探针**顺序执行**，前置硬失败后面全部 `skip`（不是 `fail`）。
   把 `skip` 算成 `fail` 会让 UI 显示 "1/12"，掩盖真正的根因。
2. 探针全程用**独立的临时 `cwd`**（`t.TempDir()` 级别的临时目录），**绝不用用户的 worktree**——铁律 6。
3. 探针结果持久化到 SQLite，`POST /v1/runtimes/{name}/probe` 重跑并覆盖。
4. **[SPEC 引用]** 安装新版本后"全过才允许切换为默认版本"（`release-and-update.md` §6）——
   这里的"全过"指**无硬失败**，不是 12/12。否则 codex 永远升不了级。
   **这是对 `release-and-update.md` §6 措辞的一处收紧，实现时按本文为准。**

---

## 9. Runtime 注册表

### 9.1 数据形态

```go
// internal/acp/runtime/registry.go

type Installed struct {
    Name        Name          // NameClaude | NameCodex
    Version     string        // "0.64.1"
    BinPath     string        // 绝对路径，指向 bin 而非包目录
    Source      Source        // SourceManaged | SourceExternal
    NodePath    string        // 该版本要用的 node 绝对路径
    LoggedIn    bool          // 由探针 05 得出，不是猜的
    Matrix      *capability.Matrix
    InstalledAt time.Time
}

type Registry interface {
    List(ctx context.Context) ([]Installed, error)
    Find(ctx context.Context, name Name, version string) (Installed, error) // 未找到 → ErrNotFound
    Active(ctx context.Context, name Name) (Installed, error)               // 当前默认版本
    Activate(ctx context.Context, name Name, version string) error
}
```

> 命名遵循 `coding-standards.md` §3.3：`Find` 可能不存在、`List` 返回空切片不返回 nil、
> 不用 `Get` 前缀、不用 `Manager` 后缀。

### 9.2 发现顺序

```
① 托管安装（Source=managed）—— 权威来源
   ~/.acpflows/runtimes/<name>/<version>/node_modules/.bin/<bin>
   目录本身就是版本清单，不需要额外索引文件

② 用户指定路径（Source=external）—— 设计稿「指定已有路径」
   记在 ~/.acpflows/runtime/external.json，绝对路径
   版本靠 `<bin> --version` 取；取不到标 "unknown" 并只允许软失败降级

③ PATH 探测 —— 仅用于「首次引导」提示，★ 不作为运行时解析来源
   理由：PATH 会因为用户装了别的 node 版本管理器而漂移，
        今天能跑明天跑不了，且用户无从知道我们用了哪个
```

**运行时永远用绝对路径 exec。** 设计稿显示的 `~/.npm-global/bin/claude-agent-acp`
是「用户指定路径」的展示形态，不是我们去 `PATH` 里搜出来的。

### 9.3 多版本并存与切换

```
~/.acpflows/runtimes/
├── claude-agent-acp/
│   ├── 0.63.0/node_modules/.bin/claude-agent-acp
│   ├── 0.64.1/node_modules/.bin/claude-agent-acp
│   └── active            ← 软链或纯文本文件，指向 "0.64.1"
└── codex-acp/
    ├── 1.1.7/…
    └── active
```

切换规则（`release-and-update.md` §6 已写死，这里补协议层细节）：

| 情形 | 行为 |
|---|---|
| 目标版本探针有硬失败 | 拒绝切换，保留旧版本，把失败项与 `Detail` 显示给用户 |
| 该 Runtime 有活跃会话 | **不打断执行中的单元**。标记 `pendingActivate`，等所有会话 `stopReason` 落盘后再切 |
| 切换后 | 新建的会话用新版本；**已存在的会话继续用旧版本进程直到结束**（进程已经起来了，中途换不了） |
| 回退 | 旧版本目录还在，改 `active` 即可。**永不删除上一个版本**，只在磁盘超过 `ACPRuntimeKeepVersions`（初值 3）时清理更老的 |

### 9.4 环境变量白名单 **[DUET]**

拉起子进程时**只**透传白名单里的变量，其余一律不传。
理由：Runtime 会读一堆环境变量改行为，用户 shell 里的残留会让"同一份配置两台机器行为不同"。

| 变量 | 谁需要 | 说明 |
|---|---|---|
| `HOME` `PATH` `TMPDIR` `LANG` `TZ` | 都要 | `PATH` 只放我们托管的 node 目录 + 系统基础目录 |
| `NODE_OPTIONS` | 都要 | 由我们设定，不透传用户的 |
| `INITIAL_AGENT_MODE` | codex **[SRC]** | 见 §10.2 |
| `CODEX_PATH` `CODEX_CONFIG` `MODEL_PROVIDER` `NO_BROWSER` `APP_SERVER_LOGS` | codex **[SRC]** | 由 adapter 决定是否设置 |
| `CODEX_API_KEY` `OPENAI_API_KEY` | codex **[SRC]** | **[DUET] 不由 duetd 注入。** 登录态归 Runtime 自己管（`~/.codex`、`~/.claude`），我们不碰用户凭据 |

**绝不透传：** `GITHUB_TOKEN` / `GH_TOKEN` 及任何 `~/.acpflows/credentials` 里的内容。
设计稿写死「令牌不写入任何项目目录、不进入 Agent 上下文」——环境变量也是上下文。

---

## 10. 两个 adapter 的差异

### 10.1 已核实的能力差 **[SRC]**

数据来自 `claude-agent-acp@0.65.0` `src/acp-agent.ts` 与 `codex-acp@1.1.10` `src/CodexAcpServer.ts`
的 `initialize` 返回值。**版本会变，实现时以探针实测为准，本表只作为设计输入。**

| 能力键 | `claude-agent-acp` | `codex-acp` | 对 Duet 的影响 |
|---|---|---|---|
| `protocolVersion` | `1` | `1` | 一致 |
| `loadSession` | `true` | `true` | 检查点恢复两端都可用 |
| `promptCapabilities.image` | `true` | `true` | 「粘贴截图」两端可用 |
| `promptCapabilities.embeddedContext` | `true` | `true` | 「附加文件到本轮上下文」两端可用 |
| `promptCapabilities.audio` | 未声明（= 不支持） | 未声明（= 不支持） | 不做音频附件 |
| `mcpCapabilities.http` | `true` | `true` | 一致 |
| `mcpCapabilities.sse` | `true` | **`false`** | **差异**。项目 MCP 只用 stdio + http，不用 SSE（MCP 自己也已弃用 SSE） |
| `auth.logout` | `{}` | `{}` | 设置页可显示「退出登录」 |
| `sessionCapabilities.resume` | `{}` | `{}` | 一致 |
| `sessionCapabilities.close` | `{}` | `{}` | 优雅关闭可用（§4.5） |
| `sessionCapabilities.list` / `delete` | `{}` | `{}` | 一致 |
| `sessionCapabilities.fork` | `{}` | **未声明** | **差异**。任何"从检查点分叉会话"的设想在 codex 上不可用 |
| `sessionCapabilities.additionalDirectories` | `{}` | `{}` | 「只读挂载文件夹」两端可用 |
| `providers` | `{}` | `{}` | 客户端托管 LLM 路由——**[DUET] 不用**（设计稿写死"模型不在协议里，由 Runtime 自身配置决定"） |
| `_meta.steering` | `supported` | `supported` | 「补充消息入队」可能可以走它，**[待验证]** 见 §16-O6 |
| `engines.node` | `>= 22` | 未声明 | 注册表要记 node 版本要求；claude 在 node < 22 上会直接起不来 |

### 10.2 会话模式：两端完全不同 ★ **[SRC]**

**`claude-agent-acp`（`buildAvailableModes()`）：**

| `id` | `name` | 说明 | 出现条件 |
|---|---|---|---|
| `auto` | Auto | 用模型分类器自动批准/拒绝权限请求 | **仅当 `ModelInfo.supportsAutoMode === true`** |
| `default` | **Manual** | 标准行为，危险操作会询问 | 总是 |
| `acceptEdits` | Accept Edits | 自动接受文件编辑 | 总是 |
| `plan` | Plan Mode | 规划模式，不实际执行工具 | 总是 |
| `dontAsk` | Don't Ask | 不询问，未预授权的一律拒绝 | 总是 |
| `bypassPermissions` | Bypass Permissions | 绕过全部权限检查 | 仅当 `ALLOW_BYPASS`（**以 root 运行时不可用**） |

**`codex-acp`（`src/AgentMode.ts`）：**

| `id` | `name` | `approvalPolicy` | `sandboxMode` | `networkAccess` |
|---|---|---|---|---|
| `read-only` | Read-only | `on-request` | `read-only` | `false` |
| **`agent`** ← **默认** | Agent | **`on-request`** | `workspace-write` | `false` |
| `agent-full-access` | Agent (full access) | **`never`** | `danger-full-access` | 允许 |

```
static DEFAULT_AGENT_MODE = AgentMode.Agent;
static getInitialAgentMode() {
    const m = process.env["INITIAL_AGENT_MODE"];
    return m ? (AgentMode.find(m) ?? DEFAULT_AGENT_MODE) : DEFAULT_AGENT_MODE;
}
```

**对 `mem-188` 的修正（对应 §2.2-C4）：**

| 设计稿 / 记忆的说法 | 实际 | 正确的做法 |
|---|---|---|
| 「codex 默认权限档为 `agent`」 | ✓ 正确 | 保留 |
| 「（不询问）」 | **✗ 错误**。`agent` 是 `approvalPolicy: "on-request"`——workspace 内写不问，**越界写与联网仍会发 `session/request_permission`**。真正"不询问"的是 `agent-full-access`（`never`） | 记忆条目要改写成：**「codex 默认档 `agent` = workspace-write + on-request 审批；越界与联网仍会请求权限。需要只读角色时用 `session/set_mode` 切 `read-only`」** |
| 「需 `session/set_mode` 收权」 | ✓ 方向正确，但收权目标是 `read-only`，不是"从不问改成问" | 保留，补上目标 mode |
| 「`INITIAL_AGENT_MODE` 也能设初始档」 | 设计稿未提 | **[DUET]** 两条路都用：起进程时设 `INITIAL_AGENT_MODE` 收窄初值（省一次往返、消除首轮窗口），建会话后再 `set_mode` 兜底 |

> **首轮窗口问题：** 只靠 `session/set_mode` 收权，存在一个窗口——
> 会话建好到 `set_mode` 返回之间，如果有一轮已经在跑，它跑在默认档上。
> 我们的流程里 `session/new` 之后不会立刻 `prompt`，窗口理论上为零，
> 但 `INITIAL_AGENT_MODE` 是零成本的第二道保险，**必须设**。

### 10.3 adapter 接口

```go
// internal/acp/adapter/adapter.go

// Adapter 收敛一个 ACP Runtime 的启动方式与已知怪癖。
// 除本包外，任何地方出现 `if name == "codex"` 都视为违规。
type Adapter interface {
    Name() runtime.Name

    // Command 返回拉起该 Runtime 的完整命令与环境。
    // env 是白名单结果（§9.4），不是 os.Environ() 的拷贝。
    Command(inst runtime.Installed, cwd string) (bin string, args []string, env []string)

    // ResolveMode 把角色配置里的 modeId 映射到该 Runtime 实际提供的 modeId。
    // available 来自本次 session/new 的返回，不是常量表（§5.3）。
    // 无法映射时返回 ok=false，调用方降级到 currentModeId。
    ResolveMode(want string, available []protocol.SessionMode) (got string, ok bool)

    // Quirks 返回该 Runtime 已知的行为偏差，供 session 层做针对性处理。
    Quirks() Quirks
}

type Quirks struct {
    // InitialModeEnv 非空时，Command 会设置该环境变量为收权后的初始 mode。
    // codex: "INITIAL_AGENT_MODE"；claude: ""（无此机制）
    InitialModeEnv string

    // SpawnsGrandchildren 为真时，优雅关闭必须按进程组处理。
    // codex: true（会再拉起 Codex App Server）；claude: 待验证
    SpawnsGrandchildren bool
}
```

`ResolveMode` 的映射表由各 adapter 自己持有，是**语义映射**不是字符串对齐：

| Duet 角色语义 | claude modeId | codex modeId |
|---|---|---|
| 只规划不动手 | `plan` | `read-only` |
| 只读探查 | `plan`（无更贴切的） | `read-only` |
| 正常实现（危险操作要问） | `default` | `agent` |
| 放开编辑但仍受权限约束 | `acceptEdits` | `agent` |
| 完全放开 | `bypassPermissions` | `agent-full-access` |

> **[DUET] 最后一行默认不可选。** 设计稿的「D3 外部动作一律逐次授权」与
> 「关闭 Runtime 机器级记忆 / 禁用未授权项目 MCP Server」是同一套隔离思路；
> `bypassPermissions` / `agent-full-access` 会绕过我们全部的权限裁决，
> 等于把 Duet 的核心机制关掉。要开必须走 D3 逐次授权。

---

## 11. ACP 事件 → 应用事件的映射

### 11.1 边界

```
  ACP 线上收到的东西                本层产出                 上层（不归本文管）
  ─────────────────────           ─────────────           ────────────────────
  session/update 的 11 个变体  ──▶  acp.Event          ──▶  eventbus 分配 seq
  session/request_permission ──▶   (无 seq, 无 work_id)      + 补 work_id / id / ts
  session/prompt 的 stopReason                              + 落库 + SSE 扇出
```

**`internal/acp` 不分配 `seq`，不认识 `work_id`。**
理由：`seq` 必须在**持久化事务里**分配才能保证单调且无洞，
而协议层不碰数据库（依赖方向：`acp` 是基础设施，实现 `app` 定义的 port）。

```go
// internal/acp/acp.go

// Event 是协议层产出的原始事件，尚未编号。
type Event struct {
    SessionID string            // ACP sessionId，app 层据此找到 Work
    Type      EventType         // 5 类，见下表
    At        time.Time         // 来自注入的 Clock，不是 time.Now()
    Payload   json.RawMessage   // 已规整过的载荷，前端可直接消费
}
```

### 11.2 映射表 ★

**[SPEC]** v1 的 `SessionUpdate` 判别值共 **11 个**（schema 全集）：
`user_message_chunk` · `agent_message_chunk` · `agent_thought_chunk` · `tool_call` ·
`tool_call_update` · `plan` · `available_commands_update` · `current_mode_update` ·
`config_option_update` · `session_info_update` · `usage_update`。

| ACP 线上来源 | 应用事件 `type` | 处理 |
|---|---|---|
| `session/update` · `agent_message_chunk` | `message_chunk` | 按 `messageId` 分组；同 id 属同一条消息，id 变了就是新消息 **[SPEC]** |
| `session/update` · `agent_thought_chunk` | `thought_chunk` | **只发摘要，不发模型私有思维链原文**（`architecture.md` §4 约束）。截断规则在 app 层，协议层原样上抛 |
| `session/update` · `tool_call` | `tool_call` | 建立 `toolCallId` → 状态的会话内索引 |
| `session/update` · `tool_call_update` | `tool_call` | **合并进同一类**。所有字段除 `toolCallId` 外都是可选增量 **[SPEC]**，本层做**字段级合并**后上抛完整快照——让前端拿到的永远是完整态，而不是补丁 |
| `session/request_permission`（**请求**） | `request_permission` | 阻塞当前轮；应答见 §7 |
| `session/prompt` **响应**的 `stopReason` | `turn_end` | ★ **不是通知，是响应字段**。本层在响应到达时**合成**一条事件（对应 §2.2-C7） |

**明确丢弃 / 不映射的（每一条都要有理由，不许"忘了处理"）：**

| 变体 | 处理 | 理由 |
|---|---|---|
| `user_message_chunk` | 丢弃 | 只在 `session/load` 回放时出现。用户消息由 Duet 自己产生并已落库，回放会造成重复 |
| `plan` | **丢弃 + debug 日志** | ★ **同名陷阱**：ACP 的 `plan` 是 Agent 的 TODO 清单，**不是** Duet 的 `PlanVersion`。把它映射成 `app/plan_version` 会把 Agent 的临时待办写进只增不改的计划版本链，是严重数据污染。13 类事件枚举里没有它的位置，要加就得改设计规范 + OpenAPI + 前端注册表三处（`architecture.md` §4） |
| `available_commands_update` | 丢弃 | M0 不用 slash command |
| `current_mode_update` | **不产事件**，更新 `Session.currentModeId` | Agent 可以自己换 mode **[SPEC]**。变化本身如果需要露出给用户，由 app 层发 `app/state_change`，不由协议层伪造成 `acp` 事件 |
| `config_option_update` | 更新会话元数据 | 见 §16-O2 |
| `session_info_update` | 更新会话元数据 | 标题/时间戳，与事件流无关 |
| `usage_update` | 存进会话元数据，进执行报告 | token 与成本要进 `Attempt` 的执行报告，但不是时间线上的一条事件 |
| 未知判别值 | **记 warn 并丢弃，绝不报错** | **[SPEC]** 能力是可扩展的、新增不算破坏性变更。遇到没见过的变体就断开，等于每次 Runtime 升级都炸一次 |

### 11.3 `seq` 分配与断点续传

```
 acp.Event (无 seq)
      │
      ▼
 app 层用例：一个事务内
      ① seq = 该 work_id 的 max(seq) + 1        ← 单调、连续、无洞
      ② INSERT INTO events(work_id, seq, type, ts, payload)
      ③ COMMIT
      │
      ▼
 eventbus 扇出 → SSE:  id: <seq>\ndata: {…}\n\n
      │
      ▼
 前端断线 → 重连带 Last-Event-ID: <最后收到的 seq>
      │
      ▼
 api 层：SELECT … WHERE work_id=? AND seq > ? ORDER BY seq
        先回放历史，再接上实时订阅
```

**三条硬约束：**

1. **`seq` 按 `work_id` 分配，不是全局。** 全局序号会让两个 Work 的续传互相干扰。
2. **先落库，再扇出。** 反过来会出现"前端收到了 seq=100，重启后库里只有 99"——
   续传时永久缺一条。落库失败就不扇出，事件丢失但状态一致。
3. **回放与实时之间不许有窗口。** 订阅时先注册订阅者（消息进缓冲区），
   再查历史、发历史、然后放缓冲区。顺序反了会在高频写入时丢事件。

> 「取消后最后事件游标可读」是 `unit-012` 的验收标准 R4，也是产品硬需求。
> 它成立的前提就是上面这三条。**任何"为了性能异步落库"的提议都直接拒绝。**

---

## 12. Fake ACP Runtime 详细设计 ★

> 本节要具体到能照着写代码。看完这节写不出来，说明这节没写好，回来补。

### 12.1 定位

```
    单元 / 集成测试                          e2e (Playwright)
         │                                       │
         │ 进程内                                 │ 真子进程
         ▼                                       ▼
   fake.Runtime ──Transport()──▶ 内存双工管道   fakeacp 二进制 ──stdio──▶ duetd
         │                                       │
         └───── 同一份 Script / Capability ───────┘
```

**两种运行形态共用同一份脚本格式。** e2e 里跑的和单测里跑的必须是同一个 Fake，
否则"单测绿 + e2e 红"的时候你不知道该信谁。

**依赖约束（重申 §3.3）：`fake` 只 import `protocol`。**

### 12.2 API 形态（Go 签名级）

```go
package fake

// ── 构造 ────────────────────────────────────────────────────────────

type Options struct {
    Script     *Script          // 必填
    Capability Capability       // 零值 = 12 项探针全过
    Clock      Clock            // 必填。禁止 time.Now()（testing-strategy.md §5）
    Latency    Latency          // 零值 = 无延迟、不乱序
    Stderr     io.Writer        // Fake 自己的诊断日志，默认 io.Discard
}

// New 构造一个 Fake ACP Runtime。opts.Script 为 nil 时 panic
// （测试夹具构造失败必须立刻暴露，不该返回 error 让调用方忽略）。
func New(opts Options) *Runtime

// Transport 返回进程内双工管道，直接喂给 jsonrpc.Conn。
// 多次调用返回同一个实例。
func (r *Runtime) Transport() io.ReadWriteCloser

// Serve 以子进程形态运行：从 in 读、往 out 写，直到 ctx 结束或 in EOF。
// cmd/fakeacp/main.go 只做参数解析 + 调这个函数。
func (r *Runtime) Serve(ctx context.Context, in io.Reader, out io.Writer) error

func (r *Runtime) Close() error

// ── 断言面 ★ ────────────────────────────────────────────────────────

// Recorded 是 Fake 收到的一条消息的完整记录。
type Recorded struct {
    N      int              // 接收序号，从 1 开始
    At     time.Time        // 来自注入 Clock
    Kind   Kind             // KindRequest | KindNotification
    ID     json.RawMessage  // Kind==KindRequest 时非空
    Method string
    Params json.RawMessage  // 原始字节，测试自己解
}

// Requests 返回至今收到的全部消息，按接收顺序。返回副本，调用方可随意持有。
func (r *Runtime) Requests() []Recorded

// CountMethod 返回某个方法被调用的次数。
// R3「连续取消两次只发送一次协议取消请求」就断言 CountMethod("session/cancel") == 1。
func (r *Runtime) CountMethod(method string) int

// WaitFor 阻塞直到出现满足 pred 的消息，或 ctx 结束。
// 用于「等 Fake 真的收到 cancel 之后再做下一步」这类时序断言，
// 替代 time.Sleep —— 测试里禁止睡眠等待。
func (r *Runtime) WaitFor(ctx context.Context, pred func(Recorded) bool) (Recorded, error)

// Emit 在脚本之外主动推送一条 session/update。
// 用于测试里精确控制时序（脚本负责常规路径，Emit 负责刁钻时序）。
func (r *Runtime) Emit(sessionID string, update any) error

// Ask 在脚本之外主动发起一次 session/request_permission，返回客户端的应答。
func (r *Runtime) Ask(ctx context.Context, sessionID string, opts ...protocol.PermissionOption) (protocol.RequestPermissionOutcome, error)
```

### 12.3 脚本格式

脚本是 **JSON**（可 golden 化、可被 e2e 从磁盘加载），Go 端有等价的结构体与 builder。

```go
type Script struct {
    Name       string              `json:"name"`
    Initialize *InitializeBehavior `json:"initialize,omitempty"`
    NewSession *NewSessionBehavior `json:"new_session,omitempty"`
    Turns      []Turn              `json:"turns"`
    Default    *Turn               `json:"default,omitempty"` // 超出 Turns 长度时复用
}

type InitializeBehavior struct {
    ProtocolVersion int             `json:"protocol_version"` // 0 → 默认 1
    AgentName       string          `json:"agent_name"`
    AgentVersion    string          `json:"agent_version"`
    Capabilities    json.RawMessage `json:"capabilities,omitempty"`
    AuthRequired    bool            `json:"auth_required"`    // true → session/new 返回 -32000
    Fault           Fault           `json:"fault,omitempty"`
}

type NewSessionBehavior struct {
    SessionID     string   `json:"session_id"`      // "" → "sess_fake_0001"
    Modes         []Mode   `json:"modes,omitempty"` // nil → 响应里不含 modes 字段
    CurrentModeID string   `json:"current_mode_id"`
    Delay         Dur      `json:"delay"`
    Fault         Fault    `json:"fault,omitempty"`
}

type Turn struct {
    Match      *Match          `json:"match,omitempty"`  // nil → 按顺序取
    Steps      []Step          `json:"steps"`
    StopReason string          `json:"stop_reason"`      // "" → ★ 永不响应，用于测超时
    StopDelay  Dur             `json:"stop_delay"`
    OnCancel   *CancelBehavior `json:"on_cancel,omitempty"`
}

type Match struct {
    PromptRegex string `json:"prompt_regex"` // 对 prompt 里全部 text 块拼接后匹配
}

type Step struct {
    Delay Dur             `json:"delay"`
    Emit  json.RawMessage `json:"emit,omitempty"`  // 一个 SessionUpdate 变体，原样下发
    Ask   *PermissionAsk  `json:"ask,omitempty"`   // 发 session/request_permission 并等应答
    Raw   string          `json:"raw,omitempty"`   // 原样写这一行（造畸形报文、未知变体）
    Fault Fault           `json:"fault,omitempty"`
}

type PermissionAsk struct {
    ToolCallID string                      `json:"tool_call_id"`
    Options    []protocol.PermissionOption `json:"options"`
    // OnOutcome 记录客户端应答；脚本可根据它分支（在 Go builder 里用回调实现）
    ExpectTimeout Dur `json:"expect_timeout"` // 超过则 Fake 记一条诊断，不失败
}

type CancelBehavior struct {
    // Never=true：收到 session/cancel 也不响应 session/prompt。
    // ★ 这是 testing-strategy.md §3.5 里 fake.NeverStops 的底层开关，
    //   用于测 ErrCancelTimeout → 更新 prepare 返回 blocked。
    Never bool `json:"never"`

    // WaitPermission=true：收到 cancel 后，先等在飞的 request_permission 被应答，
    // 才回 stopReason。★ 用来让「忘了用 cancelled outcome 应答权限请求」
    //   这条 bug（§6.3 义务 2）真的会红。
    WaitPermission bool `json:"wait_permission"`

    Delay        Dur               `json:"delay"`
    ExtraUpdates []json.RawMessage `json:"extra_updates"` // 取消后仍推送（SPEC 允许）
    StopReason   string            `json:"stop_reason"`   // "" → "cancelled"
}
```

**JSON 脚本示例（`testdata/scripts/unit012_cancel_idempotent.json`）：**

```json
{
  "name": "unit-012 取消幂等性与现场保留",
  "new_session": {
    "session_id": "sess_fake_0001",
    "current_mode_id": "default",
    "modes": [{"id": "default", "name": "Manual"},
              {"id": "plan", "name": "Plan Mode"}]
  },
  "turns": [{
    "steps": [
      {"delay": "10ms", "emit": {"sessionUpdate": "agent_message_chunk",
        "messageId": "msg_1", "content": {"type": "text", "text": "开始分析 cancel.rs"}}},
      {"delay": "10ms", "emit": {"sessionUpdate": "tool_call",
        "toolCallId": "call_001", "title": "编辑 cancel.rs", "kind": "edit", "status": "pending"}},
      {"delay": "5s",  "emit": {"sessionUpdate": "tool_call_update",
        "toolCallId": "call_001", "status": "in_progress"}}
    ],
    "stop_reason": "end_turn",
    "stop_delay": "60s",
    "on_cancel": {
      "delay": "50ms",
      "extra_updates": [{"sessionUpdate": "tool_call_update",
        "toolCallId": "call_001", "status": "completed",
        "content": [{"type": "diff", "path": "/w/crates/engine/src/cancel.rs",
                     "oldText": "", "newText": "pub async fn cancel(){}"}]}],
      "stop_reason": "cancelled"
    }
  }]
}
```

这一份脚本同时支撑 R3（幂等）与 R4（取消后 diff 与游标可读）两条验收标准：
`stop_delay: 60s` 保证不取消就不会自己结束，`extra_updates` 制造"取消窗口里补发 diff"的真实场景。

### 12.4 Go builder DSL

单测里手写 JSON 太噪。builder 产出同一个 `*Script`：

```go
s := fake.NewScript("unit-012 取消幂等性与现场保留").
    Session("sess_fake_0001").
    Modes(fake.Mode{ID: "default", Name: "Manual"}, fake.Mode{ID: "plan", Name: "Plan Mode"}).
    Turn().
        After(10*time.Millisecond).Say("msg_1", "开始分析 cancel.rs").
        After(10*time.Millisecond).Tool("call_001", protocol.ToolKindEdit, "编辑 cancel.rs").
        After(5*time.Second).ToolStatus("call_001", protocol.ToolStatusInProgress).
        StopAfter(60*time.Second, protocol.StopReasonEndTurn).
        OnCancel(fake.Cancelled().
            After(50*time.Millisecond).
            ToolDone("call_001", fake.Diff("/w/crates/engine/src/cancel.rs", "", "pub async fn cancel(){}"))).
    Build()

rt := fake.New(fake.Options{Script: s, Clock: testutil.FixedClock(t)})
```

**`testing-strategy.md` §3.5 里已经写死的预设必须存在，签名不许改：**

```go
// NeverStops 让 Runtime 收到 session/cancel 后永不响应 session/prompt。
// 用于测 ErrCancelTimeout。签名固定为 func(*Runtime)，见 testing-strategy.md §3.5。
func NeverStops(r *Runtime)

// 另外三个常用预设
func SilentAfter(d time.Duration) func(*Runtime)  // d 之后彻底断流
func Slow(base, jitter time.Duration) func(*Runtime)
func Reorder(seed int64) func(*Runtime)           // 确定性乱序相邻步骤
```

### 12.5 故障注入

```go
type Fault string

const (
    FaultNone        Fault = ""
    FaultCloseStdout Fault = "close_stdout"  // 立刻关 stdout → 客户端看到 EOF（崩溃）
    FaultHalfLine    Fault = "half_line"     // 写半行合法 JSON 后不写 \n 就停 → 测分帧
    FaultGarbage     Fault = "garbage"       // 写一行非 JSON → 测「记 warn 不断开」
    FaultHugeFrame   Fault = "huge_frame"    // 写一行超过 ACPMaxFrameBytes → 测 ErrFrameTooLarge
    FaultExit        Fault = "exit"          // 进程退出码 1（仅子进程形态）
    FaultHang        Fault = "hang"          // 收下请求不响应也不断开 → 测超时
    FaultSlowStdout  Fault = "slow_stdout"   // 每字节间加延迟 → 测背压与静默超时
)

type Latency struct {
    Base    time.Duration // 每个 Step 的基础延迟
    Jitter  time.Duration // ± 抖动，由 Seed 确定性生成
    Reorder bool          // 相邻 Step 按 Seed 确定性交换
    Seed    int64         // 0 → 用固定值 1，保证可复现
}
```

**所有随机都必须由 `Seed` 驱动。** 测试里禁止不可复现的随机
（`testing-strategy.md` §9 自查清单：「没有 `time.Now()` / 随机数 / 测试间顺序依赖」）。

### 12.6 必须支持的能力 → 对应哪条测试

`testing-strategy.md` §4 给了六条要求。逐条落到 Fake 的具体机制上：

| §4 要求 | Fake 的机制 | 用它写的测试 |
|---|---|---|
| 按脚本回放事件序列 | `Script.Turns[].Steps[].Emit` | 13 类事件渲染；`seq` 连续性 |
| 可配置延迟、乱序、断流 | `Latency{Base,Jitter,Reorder,Seed}` + `Fault{SlowStdout,CloseStdout,HalfLine}` | 静默超时；断点续传；背压 |
| **不回 `stopReason`** | `Turn.StopReason == ""` 或 `CancelBehavior.Never` / `NeverStops` | `ErrCancelTimeout` → `prepare` 返回 `blocked` |
| 主动发 `session/request_permission` | `Step.Ask` 或运行期 `Runtime.Ask()` | 三种裁决策略；阻塞语义；取消时的 `cancelled` outcome |
| 记录收到的全部请求 | `Requests()` / `CountMethod()` / `WaitFor()` | **R3 幂等：`CountMethod("session/cancel") == 1`** |
| 可声明能力矩阵（12 探针任意组合） | `Capability` | 探针执行顺序、`skip` 传播、硬/软失败降级 |

**能力矩阵声明：**

```go
type Capability struct {
    Probes map[capability.ProbeID]capability.Outcome
}

// AllPass 返回 12 项全过的能力声明。
func AllPass() Capability

// Failing 返回除指定项外全过的能力声明。
//   fake.Failing(capability.ProbeToolCallStream)     → 11/12
//   fake.Failing(capability.ProbeCancelSettles)      → 11/12，且含硬失败
func Failing(ids ...capability.ProbeID) Capability
```

Fake 根据 `Capability` 决定自己的行为：声明 `ProbeSessionModes` 失败，
它的 `session/new` 响应里就**真的没有** `modes` 字段——
而不是"探针函数被打了个假返回值"。**假的能力声明必须表现为真的协议行为**，
否则测的还是我们自己的探针代码，不是降级路径。

### 12.7 Fake 自己的测试

Fake 是地基，必须自证。至少四类：

| 测什么 | 怎么测 |
|---|---|
| 按脚本产出的报文与 golden 逐字节一致 | `testdata/golden/<script>.jsonl`，`-update` flag 重录 |
| 产出的每条报文都能被 `protocol` 反序列化 | 遍历 golden，逐行 `Unmarshal` 且判别值合法 |
| 故障注入真的产生了预期的字节流 | `FaultHalfLine` 后 `out` 里最后一行确实没有 `\n` |
| 幂等断言面本身是准的 | 手写发两次 `session/cancel`，`CountMethod == 2`（**Fake 不做去重，去重是被测代码的职责**） |

最后一条容易被写反。**Fake 必须如实记录它收到的一切**，
如果 Fake 自己去重了，R3 就永远绿——这正是「测试制造虚假安全感」的典型。

---

## 13. 测试策略

覆盖率门槛：`acp` 包 **≥ 80%**（`testing-strategy.md` §2）。下面是达到它的路径。

### 13.1 分层

| 层 | 位置 | 形态 |
|---|---|---|
| `protocol` | `internal/acp/protocol/*_test.go` | 表驱动：每个枚举穷举 + `IsValid()`；每个类型 marshal/unmarshal 往返；**未知判别值不报错** |
| `jsonrpc` | `internal/acp/jsonrpc/*_test.go` | 对着内存管道：id 不冲突、分帧边界、超长行、非 JSON 行、`$/cancel_request` |
| `session` | `internal/acp/session/*_test.go` | 对着 `fake.Runtime`（进程内 Transport） |
| `capability` | `internal/acp/capability/*_test.go` | 对着 `fake.New(Options{Capability: Failing(...)})` |
| `runtime` | `internal/acp/runtime/*_test.go` | 子进程用 `fake/cmd/fakeacp` 编出的真二进制；注册表用 `t.TempDir()` |
| 跨包 | `backend/tests/integration/acp_*_test.go` | `//go:build integration`，走导出 API，真 SQLite + Fake Runtime |

### 13.2 golden JSON 对比

**测什么：我们发出去的字节。** 这是唯一能挡住"字段名拼错但测试照样绿"的手段。

```
backend/internal/acp/testdata/golden/
├── initialize.jsonl          客户端发出的 initialize（含全部 clientCapabilities）
├── session_new.jsonl
├── set_mode.jsonl
├── prompt_text.jsonl
├── cancel.jsonl              ★ 只有一行 session/cancel
└── permission_reply_*.jsonl  selected / cancelled 两种
```

规整规则（否则 golden 永远对不上）：

| 字段 | 处理 |
|---|---|
| `id` | 归一成递增的 `1,2,3…`（jsonrpc 层的 id 是 atomic 分配的） |
| `sessionId` | 归一成 `sess_golden` |
| 时间戳 | 注入 `Clock` 后本来就是固定值，不需要额外处理 |
| 绝对路径 | 归一成 `/w/...`（`t.TempDir()` 每次都不同） |
| key 顺序 | 反序列化成 `map[string]any` 再按 key 排序重新序列化后比较 |

`go test ./internal/acp/... -update` 重录。
**重录必须在提交里单独说明原因**——golden 变了要么是协议变了，要么是 bug。

### 13.3 必须先红的测试清单

对应 `unit-012` 与本文关键机制，**每条都要能说出"先红的是哪个"**：

```
TestSessionCancel_R3_IsIdempotent                    连续取消只发一次 session/cancel
TestSessionCancel_R3_SecondCallAlsoWaits             ★ 第二次调用也阻塞到 settled
TestSessionCancel_R3_NotifyFailureAllowsRetry        通知写失败时 cancelRequested 回滚
TestSessionCancel_R4_CursorReadableAfterCancel       取消后 diff 与最后事件游标可读
TestSessionCancel_ResolvesPendingPermissionAsCancelled  ★ §6.3 义务 2
TestSessionCancel_AcceptsUpdatesAfterCancel          §6.3 义务 3
TestSessionCancel_TimeoutReturnsErrCancelTimeout     fake.NeverStops → ErrCancelTimeout
TestSessionCancel_CrashDuringCancelIsSettled         崩溃视为 settled 但证据标 partial
TestConn_InboundAndOutboundIDsDoNotCollide           §4.2 id 管理
TestConn_OversizedFrameRejected                      FaultHugeFrame → ErrFrameTooLarge
TestConn_GarbageLineIsLoggedNotFatal                 FaultGarbage 不断开
TestStream_UnknownSessionUpdateVariantIsDropped      未知变体不报错
TestStream_PlanUpdateIsNotMappedToPlanVersion        ★ §11.2 同名陷阱
TestStream_ToolCallUpdateMergesIntoSnapshot          增量合并成完整快照
TestPermission_AutoAllowReadOnlyMatchesReadKinds     穷举 9 个 ToolKind
TestPermission_UnknownOptionSetFallsBackToAskUser    §7.2 保守分支
TestProbe_HardFailureSkipsRemaining                  skip 不算 fail
TestProbe_SoftFailureProducesDegradation             软失败 → 降级动作
TestRegistry_ActivateBlockedByActiveSession          有活跃会话时延后切换
```

> 表驱动的子用例不单独登记，但顶层 `func TestXxx` **每一个都要进
> [`../backend/tests/INDEX.md`](../../backend/tests/INDEX.md)**，`make check-test-index` 会校验。

### 13.4 禁止

- ✗ 用真实 `claude-agent-acp` / `codex-acp` 跑任何自动化测试（铁律 6）
- ✗ 用 `time.Sleep` 等事件到达 → 用 `Runtime.WaitFor`
- ✗ 只断言 `err == nil`（`testing-strategy.md` §3.2 恒真断言）
- ✗ 断言"发了 cancel"而不断言"发了**几次**"——幂等测试的价值全在计数上
- ✗ 让 Fake 替被测代码做去重/纠错（§12.7）

---

## 14. 错误与降级

### 14.1 错误码映射 **[SPEC]**

ACP 定义的错误码全集：

| 码 | 含义 | Duet 的处理 |
|---|---|---|
| `-32700` | Parse error | 我们发出的 JSON 有问题 → **程序 bug**，记 error 并断开 |
| `-32600` | Invalid request | 同上 |
| `-32601` | Method not found | 我们调了对端不支持的方法 → 能力协商写错了。记 error，降级到不用该方法 |
| `-32602` | Invalid params | 参数错。**`session/set_mode` 传了非法 `modeId` 常落在这里** → 降级到 `currentModeId` |
| `-32603` | Internal error | Runtime 内部错 → 上抛给 app 层，Attempt 记 `failed` |
| `-32800` | **Request cancelled** | 见 §14.4 |
| `-32000` | **Authentication required** | 见 §14.3 |
| `-32002` | Resource not found | 文件不存在等 → 原样上抛 |

### 14.2 协议版本不匹配

**[SPEC]** 客户端不支持 Agent 回的版本 → **SHOULD 关连接并告知用户**。

```go
var ErrProtocolMismatch = errors.New("acp protocol version mismatch")
```

```
initialize 返回 protocolVersion = V
   │
   ├── V == 1                     → 正常
   ├── V > 1  (例如 ACP v2 落地)    → 探针 03 fail（硬失败）
   │        关连接；Runtime 标记 unavailable
   │        UI：「<name> <ver> 使用 ACP 协议 v<V>，当前 Duet 仅支持 v1。
   │             请切换到 <上一个可用版本> 或等待 Duet 更新。」
   │        ★ 多版本并存（§9.3）在这里救命：用户能一键退回旧版本
   └── V < 1                      → 同上
```

**不做自动降级协商。** 协议层不许"试着按 v1 说话看看行不行"——
ACP 的 `protocolVersion` 只在**破坏性变更**时递增 **[SPEC]**，硬撑必然产生难以诊断的错乱。

### 14.3 未登录

```
session/new → error -32000 Authentication required
   │
   ├── initialize 返回的 authMethods 非空
   │      → 探针 05 fail（软失败），Runtime 标「已安装 · 未登录」
   │      → 不允许把角色绑到它（绑了也跑不了）
   │      → UI 显示可用的 authMethods（name/description 来自 Agent）
   │      → ★ 登录动作由用户在 Runtime 自己的 CLI 里完成，Duet 不代理
   │         理由：AGENTS.md §9「禁止设计 ACP 不支持的设置项」，
   │              且凭据不该经手 duetd
   └── authMethods 为空 → Runtime 自相矛盾，记 error，标 unavailable
```

**[DUET]** 我们**不**调 `authenticate`。设计稿的设置页只显示「已安装 · 已登录」/「未登录」状态，
没有登录表单——这是刻意的，符合「不碰用户真实凭据」。

### 14.4 Runtime 崩溃

```
        stdout EOF  或  cmd.Wait() 返回
                    │
                    ▼
        收集：退出码 · signal · 最后 64 KiB stderr · 最后 20 条已收报文
                    │
                    ▼
        所有 in-flight 请求 → ErrRuntimeCrashed（含上面的诊断）
        所有活跃会话 → 标 settled（进程没了，不会再有更新）
        证据 → 标 partial
                    │
                    ▼
        ★ 协议层不自动重启。上报给 app 层。
```

**为什么不自动重启：** 协议层不知道当前 Unit 能不能安全重放。
重放一个已经写了一半文件的 `edit` 工具调用，可能造成重复写入。
决策属于 app 层（可能升级为 D2 让用户裁决）。**这条是 §15「不做什么」的具体体现。**

崩溃在取消过程中发生的特殊处理见 §6.5：视为 settled，但证据标 `partial`，
让 `prepare` 能返回 `ready` 而不是永远 `blocked`——进程都不在了，再等也等不到 `stopReason`。

### 14.5 超时汇总

| 错误 | 触发 | 上层表现 |
|---|---|---|
| `ErrInitializeTimeout` | §4.3 | Runtime `unavailable` |
| `ErrSessionNewTimeout` | §4.3 | Work → `failed` |
| `ErrSetModeTimeout` | §4.3 | 降级到 `currentModeId`，发 `app/state_change` |
| **`ErrCancelTimeout`** | §6.5 | **更新 `prepare` → `blocked`**；用户选强制或稍后 |
| `ErrFrameTooLarge` | §4.1 | 断开 + 走崩溃流程 |
| `ErrTurnInFlight` | §5.2 | 同一会话并发 prompt，直接拒绝，不下发 |
| `ErrRuntimeCrashed` | §14.4 | 见上 |

全部定义在 `internal/acp/errors.go`，**必须可被 `errors.Is` 判定**，
`api` 层统一映射成 `Problem`（`coding-standards.md` §3.5）。

---

## 15. 不做什么

**这一层不认识 Work / Plan / Subplan / Unit / UnitContract / Attempt / Evidence / Decision / Checkpoint。**

| 不做 | 归谁 | 为什么 |
|---|---|---|
| 业务状态机（`executing` → `reviewing_unit` …） | `domain/model` | `domain` 零 IO 是整套测试策略的地基（`architecture.md` §3） |
| 判断"这次取消该不该被允许" | `domain/policy` | R5「`reviewing_unit` 状态下拒绝取消」是**领域规则**，不是协议规则 |
| 分配 `seq` / 落库 / SSE 扇出 | `app` + `eventbus` + `store` | `seq` 必须在持久化事务里分配（§11.3） |
| 决定崩溃后要不要重启、要不要重放 | `app` | 协议层没有足够信息（§14.4） |
| 决定权限请求给用户看什么文案 | `api` + 前端 | 协议层只产出结构化 `Decision` 与事件载荷 |
| 采集 git diff / 跑测试命令 | `gitx` | `tool_call.content` 里的 diff 是 Agent 报的，**证据以 `gitx` 采集的为准** |
| 下载 App 安装包 | Tauri updater | `release-and-update.md` §8 写死 |
| 管理 GitHub 令牌 | `ghx` | 令牌绝不进入 Agent 上下文，包括环境变量（§9.4） |
| 写 `~/.acpflows` 用户真实数据 | —— | 铁律 6。测试一律 `t.TempDir()` |

**判据（可机器检查）：`internal/acp` 及其子包禁止 import
`internal/domain/**` 与 `internal/app`。** 由 `backend/.golangci.yml` 的 `depguard` 强制。

反过来，`app` 层在 `port/` 里定义它需要的接口，`acp` 去实现
（接口定义在使用方，见 [`design-principles.md`](../rules/design-principles.md) §3.1、§3.2）：

```go
// internal/app/port/gateway.go（示意，最终以 app 层为准）
type RuntimeGateway interface {
    OpenSession(ctx context.Context, spec SessionSpec) (Session, error)
    ProbeRuntime(ctx context.Context, name string, version string) (Matrix, error)
    ListRuntimes(ctx context.Context) ([]Installed, error)
}
```

**`acp` 不 import `app/port`。** Go 是结构化类型，实现方不需要引用接口定义方——
这正是依赖方向保持单向的原因。

---

## 16. 开放项：必须用真实 Runtime 验证的假设 ★

**下面每一条都是"看起来合理但没有证据"的假设。实现时验证完回来把 [待验证] 划掉并填上证据。
在验证之前，不许在别处把它们当事实引用。**

| # | 假设 | 为什么没法先确认 | 验证方式 | 影响面 |
|---|---|---|---|---|
| **O1** | Duet 只支持 `protocolVersion 1`，遇到别的版本直接断开 | ACP v2 已有 Draft（`v2.0.0-alphaX`），但尚未稳定；两个 adapter 目前都返回 1 | 跟踪 `agentclientprotocol.com/protocol/v2/migration`；等 adapter 开始返回 2 时再定策略 | `architecture.md` §8-2 的开放项；§14.2 |
| **O2** | 会话模式走 `session/set_mode` | **[SPEC]** 官方已挂废弃告示：dedicated session mode methods 将在未来版本移除，改用 Session Config Options（`session/set_config_option` / `config_option_update`）。**[SRC]** codex-acp 已同时暴露两套 | 实测两个 adapter 的 `session/set_config_option` 是否可用；确认迁移时间表 | 角色配置、`AGENTS.md` §8 术语表里的「会话模式 = `session/set_mode`」；§10.2 |
| **O3** | codex 探针 11/12，失败项待定 | 设计稿只写了 `11/12`，**没写失败的是哪一项**。从公开源码只能看出两处与 claude 的差（`mcp.sse=false`、无 `session.fork`），而这两项都**不在** §8.1 的 12 项里 | 对真实 `codex-acp` 跑一遍 12 项探针，把失败项与 `Detail` 记进本文 | §8.1；设置页显示 |
| **O4** | `ACPCancelTimeout = 30s` 够用 | 没有真实取消耗时数据。设计稿原型写 10s，我们放宽到 30s 纯属推测 | 对两个 Runtime 各做 20 次取消（含工具执行中取消），取 p99 | §4.3；`ErrCancelTimeout` → `prepare` 的 `blocked` 率 |
| **O5** | 检查点恢复用 `session/load`（两端都声明 `loadSession: true`） | 未验证 `session/load` 回放的完整度：是否包含 `tool_call` 与其 `content`，还是只回放消息 | 建会话 → 跑一轮含工具调用 → `session/load` → 比对回放出的 update 与原始序列 | 从检查点恢复的实现选型；也可能改用 `session/resume`（不回放） |
| **O6** | 「补充消息入队」用 `_meta.steering` 扩展 | **[SRC]** 两端都在 `initialize` 的 `_meta` 里声明 `steering.supported=true`，但这是**非标准扩展**，无公开规范 | 读 adapter 源码里 `_session/steering` 的请求形态并实测 | 设计稿的「补充消息如何处理 · 已入队 1 条」 |
| **O7** | `claude-agent-acp` 不会拉起孙子进程 | codex 明确会（Codex App Server）。claude 走 SDK，**未验证**是否 fork 子进程 | 起进程后查进程树 | §4.5 僵尸清理；`Quirks.SpawnsGrandchildren` |
| **O8** | `agent_thought_chunk` 两端都会发 | codex README 提到 "reasoning" 事件，claude 未明说；且是否发送可能取决于模型与配置 | 各跑一轮复杂任务，统计变体计数 | 事件流「思考摘要」类型；§8.2 可选能力 |
| **O9** | `tool_call.content` 里的 `diff` 足以当证据补充 | 未验证两端是否真的下发 `type: "diff"` 内容块，还是只给文本 | 跑一轮文件编辑，抓 `tool_call_update` 的 `content` | 证据采集（主证据仍以 `gitx` 为准，这只是补充） |
| **O10** | `INITIAL_AGENT_MODE` 能可靠收窄 codex 的初始档 | **[SRC]** 源码逻辑清楚，但未实测非法值/大小写的行为 | 设 `read-only` 起进程，看 `session/new` 返回的 `currentModeId` | §10.2 首轮窗口 |

### 已经确认、不再是开放项的

以下曾经是疑问，现已用规范原文或源码确认，**不要再当开放项讨论**：

- `session/cancel` 是 **notification**，取消完成的唯一同步点是 `session/prompt` 的响应 —— **[SPEC]**
- 客户端 **MUST** 用 `cancelled` outcome 应答所有 pending 的 `session/request_permission` —— **[SPEC]**
- stdio 是**换行分帧**，不是 `Content-Length` —— **[SPEC]**
- `availableModes` 是运行期数据，随模型变化，**不能硬编码** —— **[SRC]**
- codex 默认 mode 是 `agent`，但它是 `on-request` 审批而**不是"不询问"** —— **[SRC]**
- 两端 mode id 完全不重叠（除概念相近外），必须走 adapter 语义映射 —— **[SRC]**

---

## 17. 相关文档

| 你要做的事 | 读 |
|---|---|
| 理解分层与依赖方向 | [`architecture.md`](architecture.md) §3 |
| 两个 adapter 怎么复用 80% 逻辑（嵌入 + 复写） | [`design-principles.md`](../rules/design-principles.md) §4 |
| 接口定义在哪、`port/` 怎么组织、包怎么拆 | [`design-principles.md`](../rules/design-principles.md) §3 §5 |
| 13 类事件枚举与 SSE 契约 | [`architecture.md`](architecture.md) §4 |
| 写这一层的任何测试 | [`testing-strategy.md`](../rules/testing-strategy.md) §2 §3 §4 §5 |
| 更新 `prepare` 为什么依赖两段式取消 | [`release-and-update.md`](release-and-update.md) §5 |
| Runtime 多版本与升级 | [`release-and-update.md`](release-and-update.md) §6 |
| 命名、文件归属、错误处理 | [`coding-standards.md`](../rules/coding-standards.md) |
| 术语与状态词 | [`../AGENTS.md`](../../AGENTS.md) §8 |
