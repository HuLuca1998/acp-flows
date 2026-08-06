# AGENTS.md · backend/internal/acp

> **就近优先**。这是 M0 的核心，也是整个项目的地基。
> 规格在 [`../../../docs/acp-integration.md`](../../../docs/acp-integration.md)，改这里之前必须读完。

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

完整规则见 [`../../../docs/design-principles.md`](../../../docs/design-principles.md) §4.4。

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

- [`../../../docs/acp-integration.md`](../../../docs/acp-integration.md) —— 规格，含 §2.2 的 13 条设计稿冲突与 §16 的 10 条待验证假设
- [`../../../docs/open-questions.md`](../../../docs/open-questions.md) Q4 系列 —— **设计稿里有已核实的事实性错误，别照抄**
- [`../../../docs/design-principles.md`](../../../docs/design-principles.md) §4

## 本域特有的坑

> 以下每条都是对着真实 ACP 规范与两个 adapter 源码核实过的。

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
