# AGENTS.md · backend/tests

> **就近优先**：与根 [`AGENTS.md`](../../AGENTS.md) 与 [`../AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**跨包**的测试与测试基建。

```
tests/
├── INDEX.md        ★ 后端全部 Go 测试的索引（含 internal/ 下的就地单测）
├── contract/       OpenAPI 契约测试：用 kin-openapi 校验请求与响应
├── integration/    跨包集成测试：真实 SQLite + 真实 git + Fake ACP Runtime
├── fixtures/       夹具：临时 git 仓库、种子数据、golden 文件、Fake Runtime 脚本
└── testutil/       测试专用辅助：建夹具、起临时服务、断言辅助、隔离守卫
```

集成测试加构建标签，让默认 `go test ./...` 保持快：

```go
//go:build integration
```

## 不负责什么

- **不放单个包的单元测试。** 那些和源码同目录（`internal/acp/session_test.go`）。
  放这里等于测试离被测代码很远，改代码时不会有人想起来更新它。
- **不放生产代码。** `testutil` 只有测试能 import，生产代码 import 会被 `depguard` 拦下。

## `testutil` ≠ `internal/util`

| | 用途 | 谁能 import |
|---|---|---|
| `internal/util` | 生产代码的纯工具函数 | 生产代码 + 测试 |
| `tests/testutil` | 建夹具、起临时服务、断言辅助 | **只有测试** |

搞混这两个是本目录最常见的错误。

## 必须提供的三个夹具

```go
rt    := fake.NewRuntime(t, ...)     // Fake ACP Runtime，见 internal/acp/fake
paths := testutil.TempPaths(t)       // 临时数据目录（内部用 t.TempDir()）
repo  := testutil.NewGitRepo(t)      // 临时 git 仓库
db    := testutil.TempSQLite(t)      // 临时 SQLite 文件（★ 不用 :memory:）
```

`:memory:` 测不出 WAL 与并发行为，而产品会并发跑多个 Work。

## 隔离守卫（铁律 6）

`testutil` 里装了守卫：测试进程一旦尝试打开 `$HOME/.acpflows` 就直接 `t.Fatal`。

**不要绕过它。** 测试禁止读写：用户真实数据目录 / 真实 git 仓库 / 真实令牌 / 真实网络。

## 检查命令

```bash
cd backend && go test -tags=integration ./tests/... -count=1
make -C ../.. check-test-index
```

## 改这里之前必读

- [`../../docs/testing-strategy.md`](../../docs/testing-strategy.md) —— 尤其 §2 分层、§3 假测试图鉴
- `go-unit-test` skill —— 项目特化的测试约束

## 本域特有的坑

- **写新测试前先查 [`INDEX.md`](INDEX.md)。** 按**行为**搜，不是按函数名搜。
  已覆盖 → 扩展它的用例表，**不要新开一个测试函数**。
  `TestCancelWorks` / `TestCancelSuccess` / `TestSessionCancelBasic` 这种三胞胎就是不查索引来的。
- **集成测试要用外部包**（`package integration_test`），只走导出 API。
  能碰到未导出符号说明测试写在了错误的抽象层。
- **测试之间不许共享状态。** 每个测试自己的 `t.TempDir()`。
- **`-count=1` 禁用缓存** —— 缓存过的绿色不算验证。
