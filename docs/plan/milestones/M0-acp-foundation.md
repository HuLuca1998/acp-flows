# M0 · 地基：ACP 协议层

> **当前进度**：S0.1 完成 · S0.2 完成 U0.2.1 · S0.3 完成 U0.3.1、U0.3.2 部分 ·
> S0.9 完成 U0.9.1 · S0.10 完成 U0.10.1。
> 剩余关键路径：**U0.4.1/U0.4.2 Fake Runtime**（整套测试策略的支点，未开工）。
>
> 体系与编号规则见 [`README.md`](README.md)。写代码前必读
> [`../../spec/acp-integration.md`](../../spec/acp-integration.md)（规格）与
> [`../../notes/acp-field-notes.md`](../../notes/acp-field-notes.md)（实测与已踩过的坑）。

## 目标

**`duetd` 能起来，能和一个 Fake ACP Runtime 完整跑完一轮会话，且真机行为已被探针验证。**

## 完成标志

```bash
make dev-web            # duetd 起来，浏览器打开有响应
make check              # 全绿，覆盖率门槛达标
go test ./internal/acp/... -race   # 对着 Fake Runtime 的全流程集成测试通过
make probe                         # 真机探针跑通，产出可对账的能力报告
```

## 为什么 M0 是这个顺序

没有 Fake Runtime，上层一切都测不了 —— 它是整套测试策略的支点。
但**在写 Fake 之前必须先有真机探针**：Fake 要模仿的是真实行为，
凭 `acp-field-notes.md` 里标着「待验证」的假设去写，等于把猜测固化成夹具，
所有依赖它的上层测试都会是假的。

> 这正是铁律 1 在架构层面的体现：**先证明现实是什么样，再写模仿现实的东西。**

## 依赖

无。**M1 可与本里程碑并行**（不同 worktree），交集只有 `/v1/system/*` 几个端点。

## 全局停止条件

触发任一条 **立刻停下来上报**，不要自行扩大范围：

- 探针结果与 `acp-field-notes.md` 的实测结论冲突 → 先更新笔记并裁定，再继续
- 需要改 `api/openapi.yaml` 而当前单元没授权
- 需要引入未经批准的第三方依赖
- 发现分层依赖方向必须调整

---

## 子计划 DAG

```
S0.1 工程地基
  ├─────────────────────────────────────────┐
  ▼                                         ▼
S0.2 JSON-RPC 传输层                    S0.9 领域模型骨架
  ├──────────────┐                          │
  ▼              ▼                          │
S0.3 真机探针 ★  S0.4 Fake Runtime ★        │
  └──────┬───────┘                          │
         ▼                                  │
      S0.5 会话生命周期                      │
         ├──────────────┐                   │
         ▼              ▼                   │
     S0.6 两段式取消   S0.7 能力探针与注册表  │
                        ▼                   │
                     S0.8 adapter 差异内化   │
                        └──────────┬────────┘
                                   ▼
                        S0.10 HTTP API + duetd serve
```

**可并行**：`S0.9` 与 `S0.2..S0.8` 整条链无交集，从第一天就能并行开。
`S0.3` 与 `S0.4` 在 `S0.2` 完成后可并行。

---

## S0.1 · 工程地基

**阶段交付物**：`make check` 在有真实 Go 代码的情况下全绿，CI 接通。

### ✓ U0.1.1 · Go module 与分层目录骨架  ·  `1da80e9`

| | |
|---|---|
| `goal` | 建立 `backend/` 的 module 与分层目录，使 `go build ./...` 通过，`make check` 不再跳过后端 |
| `allowed_changes` | `backend/go.mod` · `backend/cmd/duetd/` · `backend/internal/**` 的空包与 doc.go · `backend/.golangci.yml` |
| `forbidden_changes` | 不写任何业务逻辑；不改 `api/openapi.yaml`；不新增第三方依赖（本单元只用标准库） |
| `stop_conditions` | 发现 `architecture.md` §3 的分层与实际写起来冲突 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `go build ./...` 通过 | CI `backend` job 绿 |
| R2 | `make check-docs` 识别到新建的 `internal/*` 子包并要求文档 | 故意删一个 `AGENTS.md`，`make check-docs` 必须红 |
| R3 | `depguard` 拦住 `domain` import 基础设施包 | 写一个临时文件让 `domain` import `store`，`golangci-lint` 必须红 |
| R4 | `depguard` 拦住生产代码 import `tests/testutil` | 同上手法验证 |

**测试**：R2–R4 都是「**故意制造违规，确认检查会红**」——
`scripts/AGENTS.md` 要求的「检查脚本自己要能被测」。

### ✓ U0.1.2 · 注入式 Clock / IDGen / Paths  ·  `1da80e9`

| | |
|---|---|
| `goal` | 让 `domain` 与 `app` 层不再可能直接调 `time.Now()` / `rand` / `os.UserHomeDir()`，为所有后续测试的确定性打底 |
| `allowed_changes` | `backend/internal/app/port/system.go` · `backend/internal/platform/**` · `backend/tests/testutil/**` |
| `forbidden_changes` | 不改分层依赖方向；`platform` 不得 import `domain` 以外的内部包 |
| `stop_conditions` | 发现某个场景确实无法注入时间源 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `Clock` / `IDGen` / `Paths` 三个 port 接口存在且零实现在 `port/` | `port` 包内 `grep -c 'func '` 为 0 |
| R2 | 测试实现产出确定性序列 | 同一个 `SeqIDGen` 连续调用产出 `unit-001` `unit-002` |
| R3 | **隔离守卫生效**：测试进程访问 `$HOME/.acpflows` 直接 `t.Fatal` | 写一个故意访问该路径的测试，断言它 fail 且信息指向铁律 6 |
| R4 | lint 拦住 `domain`/`app` 里的裸 `time.Now()` | 故意写一行，`golangci-lint` 必须红 |

> R3 是铁律 6 的落点。**这个守卫本身必须有测试**，否则它可能一直是失效的。

---

## S0.2 · JSON-RPC over stdio 传输层

**阶段交付物**：能对任意 ACP 进程收发消息，进程生命周期可控、可诊断。

### ✓ U0.2.1 · ndjson 编解码与双向路由  ·  `29a45c1`

| | |
|---|---|
| `goal` | 实现 ACP 的传输层：**换行分帧**的 JSON-RPC 2.0，支持我们调对方、对方反向调我们 |
| `allowed_changes` | `backend/internal/acp/jsonrpc/**` · `backend/internal/acp/protocol/**` |
| `forbidden_changes` | 不实现任何 ACP 语义（`session/*` 方法名不出现在本单元）；不起真实子进程 |
| `stop_conditions` | 发现分帧方式与 `acp-field-notes.md` 不符 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **换行分帧**，单行完整 JSON，不含内嵌换行 | 编码一个含 `\n` 的字符串，输出仍是一行 |
| R2 | 请求/响应按 id 配对，乱序响应能正确归位 | 并发发 3 个请求，响应逆序返回，全部正确 resolve |
| R3 | 通知不带 id、不等响应 | 发通知后立即返回，不阻塞 |
| R4 | **反向请求**能被路由到注册的 handler | 对方发 `session/request_permission`，我方 handler 被调用并回值 |
| R5 | 超时后请求被取消，且不泄漏 goroutine | `goleak` 断言无泄漏 |
| R6 | 收到非法 JSON 不致命，记录后继续 | 注入一行垃圾，后续消息仍正常处理 |

**测试**：全部对着 `io.Pipe` 做，**不起子进程**——传输层不该知道进程的存在。

### ○ U0.2.2 · 子进程生命周期与 stderr 采集

| | |
|---|---|
| `goal` | 拉起/健康检查/优雅关闭 ACP 子进程，**stderr 必须被采集** |
| `allowed_changes` | `backend/internal/acp/runtime/process.go` · `backend/internal/platform/proc.go` |
| `forbidden_changes` | 不实现 Runtime 发现与版本管理（那是 S0.7） |
| `stop_conditions` | 发现需要按进程组杀（孙进程）而当前抽象做不到 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **spawn 前清除嵌套会话环境变量** | 断言子进程环境里没有 `CLAUDECODE` / `CLAUDE_CODE_ENTRYPOINT` / `CLAUDE_CODE_SSE_PORT` |
| R2 | **stderr 被完整采集**，报错时带出来 | 让假子进程往 stderr 写一行，断言错误信息里包含它 |
| R3 | 关闭先 `SIGTERM`，超时再 `SIGKILL` | 用一个忽略 SIGTERM 的假进程，断言最终被 KILL 且耗时 ≈ 宽限期 |
| R4 | 崩溃时 pid 文件残留，下次启动能清理 | 写 pid 文件 → 模拟崩溃 → 重启 → 断言僵尸被回收 |
| R5 | 并发会话数达上限时拒绝新建，不是无限起进程 | 断言第 N+1 次返回 `ErrTooManySessions` |

> R1 对本项目**必踩**：Duet 自己就在 Claude Code 里开发，
> 不清这些变量，`claude-agent-acp` 会误判自己嵌套而拒绝服务。
> 见 [`../../notes/acp-field-notes.md`](../../notes/acp-field-notes.md) §5 坑 1。

---

## S0.3 · 真机探针 ★（测试先行的落点）

**阶段交付物**：一份**可对账的**真机能力报告，把 `acp-field-notes.md` 里
标注「待验证」的假设逐条钉死或推翻。

> **这个子计划的产物不是代码，是事实。** 它决定后面 Fake Runtime 该模仿什么。

### ✓ U0.3.1 · 零模型开销的 list-only 探针  ·  `29a45c1`

| | |
|---|---|
| `goal` | 只做 `initialize` + `session/new`，收集响应结构，**不发 prompt、不产生模型费用** |
| `allowed_changes` | `backend/cmd/acpprobe/**` · `Makefile` 的 probe 目标 · `backend/tests/fixtures/probe/**` |
| `forbidden_changes` | 探针**只读**：不写目标目录、不改 `~/.claude` 与 `~/.codex` 一个字节 |
| `stop_conditions` | 本机未登录任一 runtime（应给出清晰提示而不是报晦涩错误） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 对 codex 与 claude 各产出一份 JSON 报告 | 报告含 `protocolVersion` / `agentCapabilities` / `modes` / `configOptions` |
| R2 | **零模型调用** | 断言全程未发 `session/prompt`；报告里 `usage` 为空 |
| R3 | 报告可 diff | 同一 runtime 连跑两次，除时间戳外逐字节相同 |
| R4 | runtime 未安装/未登录时给出**可执行的**提示 | 断言错误信息里含具体命令（`codex login`） |

**测试**：报告落 `backend/tests/fixtures/probe/` 做 golden；CI 上不跑（依赖真机登录态），
本地 `make probe` 手动跑。

### ◐ U0.3.2 · 用探针结果裁定待验证假设  ·  `29a45c1`（R1/R2/R4/R5 已裁定，R3/R6 待补）

| | |
|---|---|
| `goal` | 逐条核对 `acp-field-notes.md` §7 与 `acp-integration.md` §16 的待验证项，更新裁定表 |
| `allowed_changes` | `docs/notes/acp-field-notes.md` · `docs/spec/acp-integration.md` · `docs/plan/open-questions.md` |
| `forbidden_changes` | **不删既有裁定**，只增新行并标注日期与版本 |
| `stop_conditions` | 探针结果与两份文档同时冲突 → 说明理解有误，找人 |

**必须钉死的清单**

| # | 待验证项 | 怎么验 |
|---|---|---|
| R1 | `configOptions` 是否两端都有 `category`，且推理强度都是 `thought_level` | 直接读两份报告 |
| R2 | codex 的档位是否仍是 `read-only`/`agent`/`agent-full-access`，默认是否 `agent` | 读 `modes` |
| R3 | `session/set_mode` 是否已废弃、`set_config_option` 是否可用 | 读 `configOptions` 与能力声明 |
| R4 | claude 的 `session/new` 顶层是否**没有** `models` | 读报告 |
| R5 | 两端 `protocolVersion` 是否仍是 `1` | 读报告 |
| R6 | `_meta.claudeCode.options` 与 `additionalDirectories` 是否仍被接受 | 传入后看是否报错 |

**每条都要在文档里留下「版本 + 日期 + 结论」三元组。**
没有版本号的实测结论会在下次升级后悄悄退化成假设。

---

## S0.4 · Fake ACP Runtime ★

**阶段交付物**：一个可编排的假 Agent，让上层全部测试脱离真实 runtime。

> 依赖 S0.3：**照着探针钉死的真实 shape 写**，不照假设写。

### ○ U0.4.1 · 脚本回放与时序控制

| | |
|---|---|
| `goal` | 按脚本回放 `session/update` 事件序列，支持延迟、乱序、断流 |
| `allowed_changes` | `backend/internal/acp/fake/**` · `backend/tests/fixtures/acp/**` |
| `forbidden_changes` | 不认识任何业务概念（Work / Unit / Contract 不得出现在本包） |
| `stop_conditions` | 脚本格式无法表达某个必要的时序场景 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 按脚本顺序推送全部 9 类 `sessionUpdate` | 每类各一个测试，断言消费方收到且字段完整 |
| R2 | 可配置每条事件的延迟 | 断言两条事件的到达间隔 ≥ 配置值 |
| R3 | 可配置**乱序**推送 | 消费方按 `seq` 归位后顺序正确 |
| R4 | 可配置**中途断流** | 断言消费方感知到断开而不是永久阻塞 |
| R5 | **不回 `stopReason`** 模式 | `session/prompt` 永不 resolve，供 S0.6 测取消超时 |
| R6 | **记录收到的全部请求** | 可断言「只发了一次协议 cancel」 |

### ○ U0.4.2 · 权限请求与能力矩阵编排

| | |
|---|---|
| `goal` | Fake 能主动发 `session/request_permission`，能声明任意探针通过组合 |
| `allowed_changes` | `backend/internal/acp/fake/**` |
| `forbidden_changes` | 同上 |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 主动发权限请求并阻塞等应答 | 断言未应答前 turn 不结束 |
| R2 | 应答 `cancelled` 时正确解除阻塞 | 断言 turn 以 `cancelled` 收尾 |
| R3 | 可声明 12 项探针的任意通过组合 | 断言能力矩阵与声明一致 |
| R4 | **Fake 自己跑一致性契约测试** | 见 U0.8.2，同一批断言对 Fake 也成立 |

> R4 是关键：**Fake 若与真实 adapter 行为不一致，所有依赖它的上层测试都是假的。**

---

## S0.5 · 会话生命周期

### ○ U0.5.1 · initialize → session/new → prompt → stopReason

| | |
|---|---|
| `goal` | 打通一轮完整会话，**事件实时流出**（不是攒到 turn 结束） |
| `allowed_changes` | `backend/internal/acp/session/**` |
| `forbidden_changes` | 不处理取消（S0.6）；不做 adapter 差异（S0.8） |
| `stop_conditions` | 发现 `session/prompt` 的阻塞语义与规格不符 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 协议版本协商：对方回不支持的版本时关闭连接 | Fake 回版本 `2`，断言连接被关且错误可诊断 |
| R2 | `cwd` 必须是**已存在的绝对路径**，否则前置校验失败 | 传相对路径断言 `ErrInvalidCwd`，且**没有**发出 `session/new` |
| R3 | **真流式**：第一个 chunk 的到达远早于 turn 结束 | Fake 首 chunk 后延迟 2s 才结束，断言首 chunk 在 200ms 内到达 |
| R4 | `stopReason` 五种取值**各有处理**，只有 `end_turn` 算正常 | 五个用例，断言 `max_tokens`/`refusal`/`max_turn_requests` 不被当成功 |
| R5 | 系统提示词**只在首轮发** | 断言第二轮的 prompt 内容里不含系统提示词 |
| R6 | 9 类 `sessionUpdate` **每类都有去处**（可以是显式丢弃） | 穷举测试：新增一类而未处理时必须红 |

> R4、R5 分别对应前一个项目的 **H-5** 与 **H-3**；R3 对应 **H-2**。
> 见 [`../../notes/acp-field-notes.md`](../../notes/acp-field-notes.md) §1。

### ○ U0.5.2 · 会话恢复与游标持久化

| | |
|---|---|
| `goal` | `session/load` 恢复历史；事件 `seq` 单调递增且持久化 |
| `allowed_changes` | `backend/internal/acp/session/**` · `backend/internal/eventbus/**` |
| `forbidden_changes` | 不做 SSE 传输（S0.10） |
| `stop_conditions` | Fake 无法模拟 `session/load` 的历史回放 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 恢复后新 turn 能延续上下文 | Fake 记住暗号 → 重建会话 → 断言能复述 |
| R2 | **`sessionId` 端到端穿透**，第 2 轮记得第 1 轮 | 断言两轮落在同一 `sessionId` 上 |
| R3 | `seq` 单调递增且跨重启连续 | 重启后断言 `seq` 不回退、不重复 |
| R4 | 按 `Last-Event-ID` 断点续传 | 断言从 `seq=N` 恢复只收到 `> N` 的事件 |
| R5 | 恢复失败时**不伪造「会话仍连续」** | 断言降级路径显式标记为「新会话」 |

> R2 对应前一个项目的 **H-1**（最严重的一条，Duet 设计稿里那条
> 「主管 AI 会话 id 丢失修复」就是它）。R5 对应 **H-4**。

---

## S0.6 · 两段式取消与幂等

> 这是 M1 自动更新 `prepare` 的直接依赖，也是设计稿 `unit-012` 的原型。

### ○ U0.6.1 · 两段式取消

| | |
|---|---|
| `goal` | 取消 = 发 `session/cancel` 通知 → **等 `session/prompt` 带 `stopReason: cancelled` 返回** → 落游标 |
| `allowed_changes` | `backend/internal/acp/session/cancel.go` 及其测试 |
| `forbidden_changes` | **不改任何公开接口签名**；不改 `EngineEvent` 类比物的公开枚举 |
| `stop_conditions` | 发现必须扩大写入范围；发现公开接口必须变化 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **连续取消两次只发送一次协议取消请求** | Fake 记录请求数，断言 `== 1` |
| R2 | 取消后 diff 与**最后事件游标可读取** | 断言取消后 `Cursor()` 返回有效 `seq` |
| R3 | 取消时**用 `cancelled` 应答所有 pending 的 `session/request_permission`** | Fake 发 2 个权限请求后取消，断言两个都收到 `cancelled` 应答 |
| R4 | Runtime 不回 `stopReason` 时超时并可诊断 | 用 Fake 的 `NeverStops`，断言超时错误含耗时与 sessionId |
| R5 | 超时后**同时**发出取消并杀进程 | 断言子进程已退出 |
| R6 | `reviewing_unit` 状态下取消被拒绝 | 断言返回 `ErrCancelNotAllowed`（领域规则，与 S0.9 对接） |

> R3 是规范硬要求且设计稿完全没提。**漏了会导致每次取消都超时、
> M1 的 `prepare` 永远返回 `blocked`。**

---

## S0.7 · 能力探针与 Runtime 注册表

### ○ U0.7.1 · 12 项能力探针与能力矩阵

| | |
|---|---|
| `goal` | 定义并实现 12 项探针，产出能力矩阵供上层做降级判断 |
| `allowed_changes` | `backend/internal/acp/capability/**` |
| `forbidden_changes` | 上层**不得**通过 runtime 名字做判断——只能查能力 |
| `stop_conditions` | 某项探针无法在不产生模型开销的情况下完成 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 12 项探针各有明确的通过判据 | 每项一个测试 |
| R2 | 探针不过时**显式降级**，不静默失败 | 断言降级路径被走到且被记录 |
| R3 | 能力矩阵可序列化给前端 | 与 `openapi.yaml` 的 `CapabilityMatrix` schema 一致 |
| R4 | **上层零品牌判断** | `grep -rn 'codex\|claude' internal/{app,domain,api}` 结果为空，接进 CI |

> R4 直接接进 `scripts/check-naming.sh`，成为一条常驻检查。

### ○ U0.7.2 · Runtime 注册表与多版本并存

| | |
|---|---|
| `goal` | 发现已安装 runtime、版本、路径、登录态；多版本并存于 `~/.acpflows/runtimes/<name>/<version>/` |
| `allowed_changes` | `backend/internal/acp/runtime/**` |
| `forbidden_changes` | 不实现安装/升级（M1 的 1.9）；不写 `~/.claude`、`~/.codex` |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **注册表模式**：加第 3 个 runtime 只加一个包 + 登记一行 | 加一个测试用 runtime，断言无需改任何既有文件 |
| R2 | 可执行文件用绝对路径，允许环境变量覆盖 | 断言 `DUET_ACP_CODEX_CMD` 生效 |
| R3 | 多版本并存，切换默认版本不删旧版本 | 断言切换后旧版本目录仍在 |
| R4 | 未安装/未登录时状态可查且提示可执行 | 断言状态含具体修复命令 |

---

## S0.8 · adapter 差异内化

> 本子计划的规格在 [`../../rules/design-principles.md`](../../rules/design-principles.md) §4.4。

### ○ U0.8.1 · base + claude/codex 三包

| | |
|---|---|
| `goal` | 用**嵌入 + 模板方法**复用共同实现，两端差异全部吃在 adapter 内部 |
| `allowed_changes` | `backend/internal/acp/adapter/**` |
| `forbidden_changes` | 上层任何文件出现 runtime 名字；接口方法名照抄协议方法名 |
| `stop_conditions` | 发现某个差异无法在 adapter 内填平（应升级为能力查询） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | `configOptions` **按 `category` 取，不按 `id` 取** | 两端的推理强度都能用 `thought_level` 取到 |
| R2 | 接口按能力命名（`RestrictPermissions`，不是 `SetMode`） | 代码评审 + `grep` 断言 `SetMode` 不出现在 port 层 |
| R3 | 档位映射表**穷举**，加一个 `PermissionProfile` 取值时测试会红 | 穷举测试 |
| R4 | codex 建会话后自动收权 | 断言 `session/new` 后紧跟一次收权调用 |
| R5 | 共同实现只有一份 | 断言 `claude/` 与 `codex/` 各自 < 150 行 |
| R6 | `session/new` 的 params 带 `model` 被忽略 → 用 `set_config_option`，参数名 `configId` | 断言发出的 JSON 里键名是 `configId` |

### ○ U0.8.2 · 跨实现一致性契约测试 ★

| | |
|---|---|
| `goal` | 同一批断言跑遍 claude / codex / fake 三个实现 |
| `allowed_changes` | `backend/tests/contract/runtime_contract_test.go` |
| `forbidden_changes` | **断言不许按实现分支**——出现 `if impl == "codex"` 即视为抽象失败 |
| `stop_conditions` | 某条断言在某个实现上无法成立 → 说明该差异应暴露为能力而非填平 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 表驱动遍历三个实现，断言完全相同 | 测试代码里零 `if impl ==` |
| R2 | 加一个新 runtime = 表里加一行 | 加测试用 runtime，断言无需改断言 |
| R3 | 真实 runtime 的用例可用构建标签跳过 | `-tags=realruntime` 才跑 |

---

## S0.9 · 领域模型骨架（可与 S0.2..S0.8 并行）

> 规格在 [`../../spec/domain-model.md`](../../spec/domain-model.md)，115 条不变量。
> 本子计划只做**骨架 + 状态机**，完整聚合在 M2。

### ✓ U0.9.1 · Work 状态机  ·  `1da80e9`

| | |
|---|---|
| `goal` | `Work` 聚合与九态状态机，含全部合法/非法迁移 |
| `allowed_changes` | `backend/internal/domain/model/work.go` 及其测试 · `backend/internal/constant/state.go` |
| `forbidden_changes` | `domain` 不得 import 任何内部包；不得出现 `context.Context` / `time.Now()` |
| `stop_conditions` | 撞上 `open-questions.md` Q1（`initializing` 状态未定）或 Q3（`executing` 与 `waiting_user` 能否共存） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 九个状态取值与术语表一字不差 | 穷举测试对照 `constant.WorkState*` |
| R2 | 全部合法迁移可达 | 表驱动，25 条迁移各一个用例 |
| R3 | 全部非法迁移被拒且错误可辨识 | 断言 `ErrInvalidTransition` 且含 from/to |
| R4 | **新增状态而未处理时测试红** | 穷举测试覆盖 `IsValid()` |
| R5 | 覆盖率 ≥ 90% | `make cover` |

> **撞上 Q1 / Q3 必须停。** 这两条挡着状态枚举的穷举测试，猜一个会让
> 后面所有依赖它的测试建立在错误前提上。

### ○ U0.9.2 · PlanVersion append-only 与 UnitContract 冻结

| | |
|---|---|
| `goal` | 计划只增不改；契约冻结后不可变，只能出新版本 |
| `allowed_changes` | `backend/internal/domain/model/{plan,unit_contract}.go` 及其测试 |
| `forbidden_changes` | 同 U0.9.1 |
| `stop_conditions` | 撞上 Q7（`acceptance_criteria` 是否契约字段）或 Q9（谁产出契约） |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | **不存在任何修改已有 `PlanVersion` 的方法** | 反射断言：所有导出方法都不改已冻结版本的字段 |
| R2 | 重规划必须声明已验收工作的处置（四选一） | 缺少处置时断言 `ErrDispositionRequired` |
| R3 | 契约冻结后改字段返回错误 | 断言 `ErrContractFrozen` |
| R4 | 契约版本号严格递增 | 断言 v3 之后只能是 v4 |
| R5 | 覆盖率 ≥ 90% | `make cover` |

---

## S0.10 · HTTP API 骨架 + `duetd serve`

### ✓ U0.10.1 · duetd serve + 本地回环鉴权  ·  `8cf900f`

| | |
|---|---|
| `goal` | `make dev-web` 能起来，`GET /v1/system/version` 有响应，无 token 一律 401 |
| `allowed_changes` | `backend/cmd/duetd/**` · `backend/internal/api/**` · `scripts/dev-web.sh` |
| `forbidden_changes` | 手写 handler 接口——必须由 `openapi.yaml` 生成；不在 `api` 层写业务判断 |
| `stop_conditions` | 生成器无法产出符合分层要求的接口 |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 只监听 `127.0.0.1`，端口随机 | 断言无法从非回环地址连上 |
| R2 | token 写入 `session.json` 且权限 `0600` | 断言文件模式 |
| R3 | 无 `Authorization` 一律 401 | 断言 401 且不泄漏任何信息 |
| R4 | `make check-gen` 绿：生成物与 spec 一致 | CI `contract` job |
| R5 | 契约测试：响应通过 `openapi.yaml` schema 校验 | `kin-openapi` 校验 |
| R6 | `make dev-web` 真的能起来 | 冒烟脚本 curl 到 200 |

### ○ U0.10.2 · SSE 事件流

| | |
|---|---|
| `goal` | 一条 SSE 连接推送统一信封事件，支持 `Last-Event-ID` 续传 |
| `allowed_changes` | `backend/internal/api/sse/**` · `backend/internal/eventbus/**` |
| `forbidden_changes` | 不做 13 类事件的业务语义（M2）；不改事件信封 schema |
| `stop_conditions` | — |

**验收标准**

| # | 标准 | 断言 |
|---|---|---|
| R1 | 事件信封字段与 `openapi.yaml` 的 `Event` schema 一致 | schema 校验 |
| R2 | 客户端断开时订阅者被回收 | 断言 `eventbus` 订阅者数归零（**防止泄漏**） |
| R3 | `Last-Event-ID` 续传只补发 `> N` 的事件 | 断言首条事件的 `seq` |
| R4 | 慢消费者不阻塞其他订阅者 | 一个订阅者不读，断言另一个仍正常收 |
| R5 | 13 类 `type` 是封闭枚举 | 发一个未知 type 断言被拒 |

---

## M0 验收

**全部单元 `✓` 之外，还要满足：**

| # | 标准 | 怎么验 |
|---|---|---|
| A1 | `make check` 全绿，覆盖率达标 | CI |
| A2 | 对着 Fake Runtime 的**全流程**集成测试通过 | `go test -tags=integration ./tests/...` |
| A3 | 一致性契约测试对 claude / codex / fake 三个实现同时通过 | `-tags=realruntime` 本地跑 |
| A4 | 真机探针报告已归档，`acp-field-notes.md` 的待验证项**全部有裁定** | 人工核对 §7 表 |
| A5 | `grep -rn 'codex\|claude' internal/{app,domain,api}` **为空** | CI |
| A6 | 前一个项目的 10 条硬性错误，**每条都有对应的测试防线** | 逐条核对 `acp-field-notes.md` §1 的表 |
| A7 | `make dev-web` 能起来并响应 | 冒烟 |

**A6 是 M0 真正的验收标准。** 其余都是过程；不重蹈前一个项目的覆辙才是目的。
