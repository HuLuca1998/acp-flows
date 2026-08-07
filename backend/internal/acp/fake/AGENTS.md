# AGENTS.md · backend/internal/acp/fake

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](/AGENTS.md) 冲突时以本文件为准。
>
> **这是所有上层测试的地基。地基歪了上面全歪。**

## 负责什么

一个**可编排的假 ACP Agent**：按脚本回放事件、控制时序、如实记录收到的一切。

没有它，上层任何测试都得依赖真实 `claude-agent-acp` / `codex-acp` ——
慢、不确定、要账号、要网络，AI 自测通道直接废掉。

两种运行形态**共用同一份脚本**：

| 形态 | 入口 | 给谁用 |
|---|---|---|
| 进程内 | `Transport()` → 内存双工管道 | 单元 / 集成测试 |
| 子进程 | `Serve(ctx, in, out)` → stdio | e2e（`cmd/fakeacp`） |

e2e 里跑的和单测里跑的必须是同一个 Fake，
否则「单测绿 + e2e 红」的时候你不知道该信谁。

## 不负责什么

- **不认识任何业务概念。** Work / Unit / Contract / PlanVersion 不得出现在本包。
- **不去重、不过滤、不纠正。** 见下面那条铁律。
- **不做取消语义。** 收到 `session/cancel` 只记录。取消是 S0.6 的题目。

## 最重要的一条：Fake 如实记录它收到的一切

**Fake 绝不能自己去重。**

「连续取消两次只发送一次协议取消请求」（U0.6.1 R1）测的是**被测代码**的幂等。
Fake 若自己去重，那条断言就永远绿 —— 这正是「测试制造虚假安全感」的典型。

```go
// 手写发两次，Fake 必须记两条
rt.CountMethod("session/cancel") == 2
```

同理：不认识的方法回 `-32601`，不要静默成功；脚本没写的轮次**不响应**，
不要编一个 `end_turn` —— 编出来的话，测试会以为脚本覆盖到了实际没覆盖的轮次。

## 依赖方向

| | |
|---|---|
| 允许 import | **仅标准库 + `acp/protocol`** |
| 禁止 import | `jsonrpc` · `session` · `adapter` · `domain` · 其余一切 |

由 `.golangci.yml` 的 `depguard` 规则 **`acp-fake`** 强制。

**为什么这么严**：Fake 若复用 `jsonrpc` 的分帧实现，分帧写错时
Fake 和被测代码会**一起错**，测试照样绿。这是 mock 喂 mock 的变体，
只是伪装得更好。所以本包自己实现了 ndjson 分帧与 JSON-RPC 收发
（[`wire.go`](wire.go)），**明知 `jsonrpc` 包里有**。

## 检查命令

```bash
cd backend && go test ./internal/acp/fake/... -count=1 -race
cd backend && go test ./internal/acp/fake/... -count=8   # 时序测试，多跑几次才算稳
cd backend && golangci-lint run ./internal/acp/fake/...
```

## 改这里之前必读

- [`docs/spec/acp-integration.md`](../../../../docs/spec/acp-integration.md) **§12**
  —— 完整设计（API 签名、脚本格式、builder DSL、故障注入、自测要求）
- [`docs/rules/testing-strategy.md`](../../../../docs/rules/testing-strategy.md) §3.5、§4
  —— 四个预设的签名**固定**，改了会让一批表驱动测试全部改写

## 本域特有的坑

- **预设签名固定为 `func(*Runtime)`。** `NeverStops` 直接是函数值（不带括号），
  `SilentAfter(d)` / `Slow(base,jitter)` / `Reorder(seed)` 返回同样的签名。
  这样才能放进表驱动测试的 `setup` 字段。
- **测试里禁止用 `time.Sleep` 等事件**，用 `WaitFor(ctx, pred)`。
  睡眠等待要么慢要么 flaky，二选一。
- **所有随机由 `Seed` 驱动。** `Reorder` 保证一定改变顺序（长度 ≥ 2）：
  否则某些 seed 下这个开关会静默失效，而测试看起来是绿的。
- **写 `out` 必须串行化。** 脚本回放 goroutine 与响应可能同时写，
  交错会产出两个半行拼起来的畸形帧。`frameWriter` 里那把锁不是装饰。
- **`bufio.Scanner` 的缓冲会被复用。** 拿到 `sc.Bytes()` 要立刻拷贝，
  否则下一帧读进来时前一帧的内容被覆写。
- **断流计时从首个 `prompt` 起算**，不是从 `Serve` 起算——
  从 `Serve` 起算的话握手耗时会算进去，测试时序变得不可控。
- **测试客户端不要用「往 channel 塞 error」表示连接结束。**
  同时在等的 `call()` 与 `nextNotification()` 会竞争同一个值，
  谁先抢到谁醒、另一个等到超时。用 `close(done)` 广播。
  **这个坑在 R4 断流测试上真踩过。**
