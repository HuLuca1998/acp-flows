# AGENTS.md · backend/internal/acp

> **就近优先**。这是 M0 的核心，也是整个项目的地基。
> 规格在 [`../../../docs/acp-integration.md`](../../../docs/spec/acp-integration.md)，改这里之前必须读完。

## 负责什么

与 ACP Runtime 子进程（`claude-agent-acp` / `codex-acp`）通信的全部细节。

```
acp/
├── protocol/     纯类型：JSON-RPC 信封 + ACP 方法与通知定义
├── jsonrpc/      双向 JSON-RPC over stdio（★ 换行分帧，不是 LSP 的 Content-Length）
├── session/      会话生命周期、两段式取消、事件泵
├── capability/   能力探针与能力矩阵
├── runtime/      Runtime 注册表、版本发现、多版本并存
├── adapter/
│   ├── base.go   ★ 共同实现，被 claude/codex 嵌入
│   ├── claude/
│   └── codex/
└── fake/       ★ Fake Runtime —— 所有上层测试的地基，第一个交付物
```

## 不负责什么

- **不碰业务状态机。** 这一层不认识 Work / Unit / Contract / PlanVersion。
- **不做持久化。** 事件的 `seq` 分配与落盘在 `eventbus` / `store`。

## 最重要的一条：差异内化

**`grep -rn 'codex\|claude' backend/internal/{app,domain,api}` 必须是空的。**

两个 Runtime 的差异一律在 `adapter/` 内部填平，上层只表达意图、只查询能力。
接口按**能力**命名而不是按协议方法命名（`RestrictPermissions` 而不是 `SetMode`）——
ACP 官方已给 `session/set_mode` 挂了废弃告示，接口名照抄协议名的话协议一变全仓库跟着改。

完整规则见 [`../../../docs/design-principles.md`](../../../docs/rules/design-principles.md) §4.4。

## Fake Runtime 必须第一个做

没有它，上层任何测试都得依赖真实 Runtime —— 慢、不确定、要账号、要网络，
AI 自测通道直接废掉。

必须支持：按脚本回放事件 · 可配置延迟/乱序/断流 · **不回 `stopReason`**（测取消超时）·
主动发权限请求 · 记录收到的全部请求（断言幂等）· 声明任意探针通过组合。

**Fake Runtime 自己也要跑一致性契约测试** —— 它如果和真实 adapter 行为不一致，
所有依赖它的上层测试都是假的。

## 检查命令

```bash
cd backend && go test ./internal/acp/... -count=1 -race
cd backend && go test -tags=integration ./tests/integration/... -run ACP
```

覆盖率门槛 **≥ 80%**。

## 改这里之前必读

- [`../../../docs/acp-integration.md`](../../../docs/spec/acp-integration.md) —— 规格，含 §2.2 的 13 条设计稿冲突与 §16 的 10 条待验证假设
- [`../../../docs/open-questions.md`](../../../docs/plan/open-questions.md) Q4 系列 —— **设计稿里有已核实的事实性错误，别照抄**
- [`../../../docs/design-principles.md`](../../../docs/rules/design-principles.md) §4

## 本域特有的坑

> 前一半对着官方规范与 adapter 源码核实；后一半（★）来自
> [`../../../docs/acp-field-notes.md`](../../../docs/notes/acp-field-notes.md) 的**本机实测**，
> 每一条前一个项目都真实踩过。

### ★ 实测踩过的（优先看这些）

- **嵌套会话环境变量 —— 对本项目必踩。** Claude Code 给子进程注入 `CLAUDECODE` 等变量，
  继承下去传给 `claude-agent-acp` 会让它**拒绝服务**。spawn 前必须 `envRemove`。
  Duet 本身就在 Claude Code 里开发，自己手跑没问题，一从会话里启动就炸。
  → **测试与试验一律用 codex**，避免与开发环境撞车。
- **`configOptions` 按 `category` 取，不按 `id` 取。** 推理强度在 claude 是 `effort`、
  codex 是 `reasoning_effort`，但两端 `category` 都是 `thought_level`。
  这是「差异内化」最漂亮的实证——协议本身给了稳定键，不需要维护映射表。
- **`session/set_config_option` 的参数名是 `configId`，不是 `optionId`。**
  `session/new` 的 params 里带 `model` **两端都静默忽略**。
- **codex 完全不走 fs 代理**（用自带 shell），所以 **path guard 对 codex 没有落点**。
  这条推翻了「所有文件操作都过我的 guard」这个看起来很合理的架构假设。
- **cwd 目录名会导致会话历史串味。** claude 按 cwd 路径在 `~/.claude/projects/<编码>`
  存历史，同前缀的临时目录可能读到旧历史 —— 对 Duet 是**数据串扰**，不只是体验问题。
- **stderr 必须收集。** agent 崩溃、认证失败、版本错配的信息**只在 stderr 里**。
- **超时后只 reject 不够**，必须同时 `session/cancel` + 杀进程，否则 agent 还在烧钱改文件。
- **`totalTokens` 会严重高估成本**（缓存读远大于新增 token）。用 `cost.amount`，但**只有 claude 有**。
- **隔离与注入方案已跑通**，直接对应设计稿设置页的三个开关：
  claude 走 `_meta.claudeCode.options`（`settingSources` / `plugins` / `strictMcpConfig`），
  codex 走 `additionalDirectories` + `CODEX_CONFIG` 环境变量。
  **codex 的 `session/new.mcpServers` 必须传空数组** —— 非空会整体覆盖 thread config
  的 `mcp_servers` 键，禁用条目全部丢失。细节见 field-notes §4。

### 对着规范核实的

- **`session/cancel` 是 notification，不是请求。** 取消完成的唯一同步点是
  `session/prompt` 的响应带回 `stopReason: "cancelled"`。这就是「两段式」的由来。
- **取消时必须用 `cancelled` 应答所有 pending 的 `session/request_permission`。**
  规范要求，设计稿完全没提。漏了会导致每次取消都超时、M1 的 `prepare` 永远返回 `blocked`。
- **stdio 是换行分帧**（单行完整 JSON，不含内嵌换行），**不是** LSP 的 `Content-Length`。
- **ACP 的 `plan` 更新是 Agent 的 TODO 清单，不是 Duet 的 `PlanVersion`。**
  同名陷阱，误映射会污染只增不改的计划版本链。有专门的测试防它。
- **`stopReason` 不是通知**，是 `session/prompt` 的响应字段。前端看到的 `turn_end` 是应用侧合成事件。
- **设计稿的 `mem-188` 是错的**：codex 的 `agent` 档 `approvalPolicy` 是 `on-request`（会询问），
  真正不询问的是 `agent-full-access`。收权方向是 `agent → read-only`。见 open-questions Q4a。
- **codex 没有 `auto` 模式。** 它只有 `read-only` / `agent` / `agent-full-access`。见 Q4b。
- **子进程僵尸化。** duetd 退出时必须优雅关闭所有 Runtime；崩溃时靠 pid 文件在下次启动清理。
- **取消必须幂等** —— 连续取消两次只发一次协议请求。这是产品的硬需求，不是优化。
