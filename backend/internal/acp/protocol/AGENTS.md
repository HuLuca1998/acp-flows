# AGENTS.md · backend/internal/acp/protocol

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](/AGENTS.md) 冲突时以本文件为准。

## 负责什么

ACP **v1 线格式**的类型、枚举、判别式编解码。零 IO、零状态。

它存在的理由只有一个：给 `acp/fake/`（Fake Runtime，U0.4.1）一个
**不依赖任何被测代码**的协议参照物。
Fake 若复用了 `session` 或 `jsonrpc` 的实现，测试就变成「用被测代码验证被测代码」，
参照物资格当场失效。

## 不负责什么

- **不认识「会话正在进行中」这类概念。** 没有状态、没有连接、没有超时。
- **不做能力判断。** `agentCapabilities` 保持 `json.RawMessage` —— 能力是可扩展的，
  建强类型映射意味着每次 Runtime 升级都要跟着改，而漏改的字段会静默丢失。
  能力判断走 `capability/` 的探针（S0.7）。
- **不实现 v2。** SDK 里有 `dist/v2/schema`（`PROTOCOL_VERSION = 2`），
  但真机协商结果是 `1`。**协议版本以协商结果为准，不以 SDK 目录里存在什么为准。**
- **不做字段级合并。** `tool_call_update` 原样上抛，合并是 `session/` 的职责。

## 依赖方向

| | |
|---|---|
| 允许 import | **仅标准库** |
| 禁止 import | 本仓库任何包，包括 `acp/` 下的兄弟子包 |

由 `.golangci.yml` 的 `depguard` 规则 **`acp-protocol`** 强制。
配套规则 **`acp-fake`** 限定 `fake/` 只能 import 本包。
两条规则都验证过「故意违规会红」。

## 检查命令

```bash
cd backend && go test ./internal/acp/protocol/... -count=1 -race
cd backend && golangci-lint run ./internal/acp/protocol/...
```

## 改这里之前必读

- [`docs/spec/acp-integration.md`](../../../../docs/spec/acp-integration.md) §3.3（两条硬规则）、§11.2（变体处理表）
- [`docs/notes/acp-field-notes.md`](../../../../docs/notes/acp-field-notes.md) **§7.2**
  —— 判别值全集的裁定与复核命令
- 权威来源是 **`@agentclientprotocol/sdk` 的 `dist/schema/types.gen.d.ts`**，
  不是任何一份 Duet 文档。文档里的数字会漂移，生成的 schema 不会。

## 本域特有的坑

- **判别值数量别信文档。** 三份 Duet 文档曾把 v1 的 `sessionUpdate` 变体数写成
  9 / 11 / 13 三个不同的数字。真值是 **13**。改动前跑一遍复核命令（field-notes §7.2），
  别照抄任何一句话里的数字。
- **`plan_update` / `plan_removed` 官方标了 `UNSTABLE`。** 要认得（否则掉进未知分支
  刷 warn，把真正的未知变体淹掉），但**不建任何映射**。
- **`ConfigOption.Category` 必须是 `*string`。** claude 的 `agent` 选项 category
  是**空字符串**（实测），而「字段缺失」与「空字符串」是两回事。
  取值一律用 `CategoryOrEmpty()`，直接解引用会 panic。
- **`NewSessionRequest.MCPServers` 不能是 nil。** nil slice 会写成 `null`，
  而 codex 需要显式的 `[]`；给非空值会整体覆盖 thread config 的 `mcp_servers` 键，
  禁用条目全部丢失（field-notes §4）。
- **加变体时必须同时加三样**：常量、`allSessionUpdateKinds` 里一行、golden 样本。
  少任何一样，`TestSessionUpdate_R4_GoldenRoundTripLosesNothing` 会红——**这是故意的**。
- **round-trip 测试不能只比结构体。** struct 漏定义一个字段时，解析会丢弃它、
  序列化也不产出它，round-trip 照样相等，测试全绿而字段没了。
  必须拿**原始 JSON 的键**做对照。
