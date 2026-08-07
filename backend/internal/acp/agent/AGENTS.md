# AGENTS.md · backend/internal/acp/agent

上级规则见 [`backend/internal/acp/AGENTS.md`](../AGENTS.md)，仓库总纲见根 [`AGENTS.md`](../../../../AGENTS.md)。
本文件只写本目录独有的东西。

---

## 这个包干什么

**把一条 ACP 会话跑完，并把 Agent 说的话翻成契约里的工作事件。**

它是 `acp/session`（一条会话怎么跑）与 `app/work`（工作的状态机）之间那一层：

```
app/work  ──port.AgentRunner──▶  ProcessRunner  ──▶  runtime.Start（拉进程）
                                       │                  │
                                       │                  ▼
                                       └──────────▶  session.Open/Prompt
                                                          │
                                                     session.Event
                                                          │  translate()
                                                          ▼
                                                    port.WorkEvent ──▶ 总线 ──▶ SSE ──▶ 时间线
```

`app` 层**不认识**这个包（depguard 挡着：`app` 不许 import `acp`）。
装配在 `cmd/duetd`，`app` 只认识 `port.AgentRunner` 接口。

---

## 本目录的铁律

### 1. 翻译表是唯一真源，且必须盖住全集

ACP 的 13 类 `sessionUpdate` 与契约的 13 类 `Event` **不是一一对应的**：

| ACP | 契约 |
|---|---|
| `agent_message_chunk` / `user_message_chunk` | `message_chunk` |
| `tool_call` / `tool_call_update` | `tool_call` |
| `plan` / `plan_update` / `plan_removed` | `plan_version` |
| `current_mode_update` | `state_change` |
| `usage_update` 等会话元数据 | 空串（认识，但不上时间线） |

**空串是显式决定，不是遗漏。** `TestTimelineType_CoversEveryKind` 逼着每一类都表态：
`protocol` 包多一个常量，这条测试立刻红。

> 不许写成 `string(ev.Kind)` 直接当事件类型。那样前端注册表会收到一堆认不出的
> 类型，全落到兜底渲染器上，而且没有任何检查会红。

### 2. 事件边说边发，不攒批

`Sink.Emit` 跑在 jsonrpc 的读 goroutine 上，**里面不许做慢操作**——
阻塞它等于阻塞整条会话。攒完再发的表现是用户盯着不动的界面等好几秒。

### 3. 进程跑完必须收掉

`RunTurn` 的 `defer` 里 `Process.Stop`，杀的是**整个进程组**。
留着的话，用户每提一个需求就多一个常驻进程，各自握着一个 worktree——
关掉应用之后它们还在。

改这里时对着 `TestProcessRunner_LeavesNoOrphan` 想：它用一个 `sleep 300`
的假 Agent，跑完之后去 `ps` 查那个 pid。

### 4. 错误里必须带补救办法

一个 Runtime 都没就绪时，错误要说「npm i -g @agentclientprotocol/claude-agent-acp」
而不是「connection refused」。这条错误最终会出现在时间线的失败事件里，
**是用户唯一能看到的线索**。

Agent 起来了又立刻退出时，把它的 stderr 带回来——真正的原因
（「请先登录」之类）否则躺在一个没人读的管道里。

---

## 测试怎么写

| 验什么 | 对手方 |
|---|---|
| 翻译、时序、stopReason | `acp/fake` 的 Fake Runtime（走管道，不起进程） |
| 进程怎么拉起来、拉不起来时用户看到什么 | `t.TempDir()` 里的 shell 脚本冒充 Agent |
| 真 Agent 的实际行为 | `//go:build integration`，照 `make probe` 的做法 |

第二行不许换成 Fake：那正好把被测的东西（进程生命周期）替换掉了。

写 `*_test.go` 前先调 `go-unit-testing` skill。
每加一条断言，问一遍「把实现改坏，它会不会红」——**造负例，别猜**。
