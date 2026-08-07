# AGENTS.md · backend/internal/api

> **就近优先**。契约规则见 [`../../../api/AGENTS.md`](../../../api/AGENTS.md)。

## 负责什么

HTTP 传输层：路由、鉴权中间件、SSE、错误映射。

**只做协议翻译**：解析请求 → 调 `app` 用例 → 序列化响应。

## 不负责什么

- **不写业务逻辑。** 任何 `if` 只要是在判断业务规则，就该在 `app` 或 `domain`
- **不 import `store` / `acp` / `domain` 的内部类型**
- **不返回用户可见的中文文案**（见下）

## 铁律 2：handler 接口由 spec 生成

改接口的顺序永远是 **改 `api/openapi.yaml` → `make gen` → 改实现**。
`gen/` 下的文件不手改，CI 的 `contract` job 会重新生成并比对 diff。

> 当前是 M0 的最小骨架（手写 `NewRouter`），生成器接入在 M0 U0.10.1。
> 接入后这段说明要删掉。

## 错误：只返回机器可读的错误码

```go
// Problem.Type 是 snake_case 错误码，前端据此查 i18n 词条
writeProblem(w, 404, "not_found", "Resource not found")
```

| 字段 | 给谁看 |
|---|---|
| `type` | **前端**，据此查词条 |
| `title` / `detail` | 只给开发者，**界面不展示** |
| `params` | 前端词条插值 |

**后端绝不返回中文文案给界面展示**，见 [`../../../docs/rules/i18n.md`](../../../docs/rules/i18n.md) §3。
新增错误码要同时加进 `openapi.yaml` 与两个 locale 文件。

## 检查命令

```bash
cd backend && go test ./internal/api/... -count=1
```

## 本域特有的坑

- **401 响应里不能带任何内部信息** —— 不回显 token、不回显版本。
  有测试断言这件事。
- **token 比较用 `subtle.ConstantTimeCompare`**，普通 `==` 会因为比较耗时泄漏信息
- **duetd 只监听回环还不够** —— 回环上的任何本机进程都能连，
  所以每个请求都要过 token
- **SSE 连接必须绑 request context**，客户端断开要能感知到，
  否则 `eventbus` 的订阅者列表会无限增长
- 未匹配路由返回 `application/problem+json` 的 404，**不要用 Go 默认的纯文本 404**
