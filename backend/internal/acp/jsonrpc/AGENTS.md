# AGENTS.md · backend/internal/acp/jsonrpc

> **就近优先**。规格见 [`../../../../docs/acp-integration.md`](../../../../docs/spec/acp-integration.md)，
> 实测坑见 [`../../../../docs/acp-field-notes.md`](../../../../docs/notes/acp-field-notes.md)。

## 负责什么

ACP 用的 JSON-RPC 2.0 over stdio：编解码、请求响应配对、双向路由。

## 不负责什么

- **不认识任何 ACP 语义** —— `session/*` 这些方法名不许出现在本包
- **不起子进程** —— 传输层不该知道进程的存在（那是 `../runtime`）
- 不做重试、不做超时策略（调用方通过 `context` 控制）

## 三条容易搞错的

1. **分帧是 ndjson（换行分隔），不是 LSP 的 `Content-Length`。**
   搞错了对方一条都解析不出来。用 `json.Marshal` 保证内容里的换行被转义。
2. **连接是双向的。** agent 会反过来调我们（`session/request_permission`、`fs/*`）。
   不注册 handler 会让 agent 吃到 -32601，整轮可能失败。
3. **响应可能乱序。** 按 id 配对，不能按顺序。

## 检查命令

```bash
cd backend && go test ./internal/acp/jsonrpc/... -count=1 -race -timeout 60s
```

## 本域特有的坑

- **测试夹具必须后台抽干输出。** `io.Pipe` 是同步的——没人读时 `Conn` 的写会一直阻塞，
  测试会**挂死而不是失败**。这个坑踩过一次：R3 与 R5 同时挂住。
  夹具用后台 goroutine 把输出解析进 channel，`readFrame` 带超时。
- **scanner 的 buffer 要调大。** ACP 单条消息可能很大（`tool_call` 里带 diff），
  默认 64KB 会截断。当前设 16MB。
- **pending 表必须在 defer 里摘掉**，无论成功失败——否则超时的请求会让 map 无限增长。
- 反向请求要在**新 goroutine** 里处理，否则 handler 里再发请求会死锁读循环。
