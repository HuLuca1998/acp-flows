# AGENTS.md · backend

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

`duetd` —— 承载 Duet **全部业务逻辑**的单一 Go 进程。对外只暴露 HTTP + SSE。

它是唯一"知道产品在干什么"的地方：状态机、编排、持久化、Git 操作、
ACP Runtime 生命周期，全都在这里。

## 不负责什么

- **不渲染任何 UI。** 不返回 HTML 片段，不下发样式或文案排版决策。
  文案属于前端（除了错误 `detail` 这类必须由后端说明的）。
- **不做原生桌面能力。** 文件选择器、Finder 揭示、窗口控制在 `shell/`。
- **不直接被 Tauri 通过 IPC 调用。** Tauri 只能走 HTTP，和浏览器完全一样。
  破坏这条，Web 版当天就废。

## 分层与依赖方向

```
internal/api  →  internal/app  →  internal/domain
                      │
                      └─▶ (port 接口) ◀── store · acp · gitx · ghx · fsstore · eventbus
```

| 包 | 允许 import | 禁止 import |
|---|---|---|
| `domain` | 仅标准库（且不做 IO） | **本仓库任何其他包**、任何第三方库 |
| `app` | `domain`、自己定义的 port 接口 | `store` `acp` `gitx` `ghx` `api` 的具体类型 |
| `api` | `app`、生成的 openapi 类型 | `store` `domain` 的内部类型、任何基础设施包 |
| 基础设施包 | `domain`、`app` 的 port 接口 | 彼此（`store` 不 import `acp`，依此类推） |

这条依赖方向由 `golangci-lint` 的 `depguard` 规则强制，不是靠自觉。
**加新包时必须同步加 depguard 规则**，否则规则形同虚设。

## 测试要求

**写任何 `*_test.go` 前，Claude 必须先调用 `go-unit-testing` skill。**

| 层 | 测试形态 | 覆盖率门槛 |
|---|---|---|
| `domain` | 表驱动单元测试，真实实例、真实数据，零 mock | **≥ 90%** |
| `app` | 用例测试，port 用**手写 fake**（不是 mock 框架） | ≥ 75% |
| `acp` | 对着 Fake Runtime 的集成测试 + 协议 golden JSON | ≥ 80% |
| `store` | 临时 SQLite（`t.TempDir()`） | ≥ 70% |
| `api` | 契约测试：用 `kin-openapi` 对着 `api/openapi.yaml` 校验请求与响应 | — |

**硬性隔离（铁律 6）**：测试禁止读写 `~/.acpflows`、用户真实 git 仓库、真实令牌。
`internal/platform` 在测试模式下会把所有路径重定向到 `t.TempDir()`；
绕过它直接拼 `os.UserHomeDir()` 的代码会被 lint 拦。

**禁止的假测试**（`go-unit-testing` skill 里有完整清单）：

- 测试私有的琐碎 helper 而不测接口契约
- mock 喂 mock —— 断言的是自己刚设的返回值
- 恒真断言（`assert.NotNil(err == nil)` 这类）
- 只有 happy path 的假数据测试

## 检查命令

```bash
make -C .. test-backend      # go test ./... -race -count=1
make -C .. lint-backend      # go vet + golangci-lint（含 depguard）
make -C .. cover             # 覆盖率 + 门槛校验
```

## 改这里之前必读

| 改什么 | 读什么 |
|---|---|
| **写任何代码** | [`../docs/coding-standards.md`](../docs/coding-standards.md) —— 命名动词表、`model/` `constant/` `util/` 的准入规则 |
| 领域模型、状态机 | [`../docs/domain-model.md`](../docs/domain-model.md)、[`internal/domain/AGENTS.md`](internal/domain/AGENTS.md) |
| ACP 协议、Runtime 适配 | [`../docs/acp-integration.md`](../docs/acp-integration.md)、[`internal/acp/AGENTS.md`](internal/acp/AGENTS.md) |
| HTTP 接口 | [`../api/AGENTS.md`](../api/AGENTS.md) —— **先改 spec** |
| 任何测试 | [`../docs/testing-strategy.md`](../docs/testing-strategy.md) |

## 技术约定

| 项 | 选择 | 理由 |
|---|---|---|
| Go 版本 | 与 `go.mod` 一致，不用未发布特性 | |
| ORM | **GORM** | 生态最大；隐式行为多，规则见 `docs/database.md` §9 |
| 数据库 | **SQLite**，驱动 `github.com/glebarez/sqlite` | 纯 Go 无 CGO。**不做 MySQL**，见 `adr/0005` |
| HTTP 路由 | 标准库 `net/http` + `ServeMux`（Go 1.22+ 模式匹配） | 少一个依赖 |
| 日志 | 标准库 `log/slog`，结构化 | |
| 错误 | `errors.Is/As` + 领域错误类型，`api` 层统一映射成 `Problem` | |
| 依赖注入 | `cmd/duetd/main.go` 手工组装 | 不引 DI 框架 |
| 时间 | 通过 `Clock` 接口注入，**禁止在 domain/app 里直接 `time.Now()`** | 否则测不了 |
| ID 生成 | 通过 `IDGen` 接口注入，测试里用确定性实现 | 同上 |

**新增第三方依赖需要事先批准**，在 PR 描述里说明为什么标准库不够。

## 本域特有的坑

- **`domain` 里出现 `context.Context` 通常意味着分层错了。** 领域逻辑是纯计算，
  需要 ctx 说明它在做 IO。
- **`time.Now()` 和 `rand` 直接调用会让测试变成薛定谔的。** 一律注入。
- **SSE 连接泄漏。** 每个订阅者必须绑定 request context，客户端断开要能感知到，
  否则 `eventbus` 的订阅者列表会无限增长。
- **ACP 子进程僵尸化。** duetd 退出时必须优雅关闭所有 Runtime 子进程；
  崩溃时靠 pid 文件在下次启动清理。
- **别在 `app` 层拼 SQL。** 想拼的时候说明 port 接口设计错了，回去改接口。
