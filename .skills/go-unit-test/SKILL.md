---
name: go-unit-test
description: 在本仓库写或改任何 Go 测试文件时使用（*_test.go、backend/tests/ 下的任何文件）。触发场景：给 Go 代码补测试、提高覆盖率、修红的测试、写集成测试或契约测试。负责本项目特有的约束——Fake ACP Runtime、临时 SQLite、tests 包的边界、假测试图鉴、测试索引登记。Claude 用本 skill 前应先调用全局 go-unit-testing skill 获取通用方法论，本 skill 只补项目特化部分。
---

# Go 测试 · 项目特化规范

> 通用方法论（契约优先、真实实例真实数据）见全局 `go-unit-testing` skill。
> 本文只写**本项目特有**的东西。完整策略见 [`docs/testing-strategy.md`](../../docs/rules/testing-strategy.md)。

## 测试放哪

| 测什么 | 放哪 | 包名 |
|---|---|---|
| 单个包的契约 | 与源码同目录 `xxx_test.go` | 同包 或 `xxx_test` 外部包 |
| 跨包组装起来的行为 | `backend/tests/integration/` | 一律外部包，**只走导出 API** |
| OpenAPI 契约一致性 | `backend/tests/contract/` | 外部包 |
| 夹具、种子数据、golden | `backend/tests/fixtures/` | — |
| 建夹具 / 起临时服务 / 断言辅助 | `backend/tests/testutil/` | — |

**`tests/testutil` ≠ `internal/util`：**

- `internal/util` 是生产代码的工具，生产代码可以 import
- `tests/testutil` 只有测试能 import，生产代码 import 它会被 `depguard` 拦下

集成测试加构建标签，让默认 `go test ./...` 保持快：

```go
//go:build integration
```

## 三个夹具，不要自己造

### Fake ACP Runtime

```go
rt := fake.NewRuntime(t,
    fake.WithScript(fake.ScriptFromFile("testdata/cancel_flow.json")),
    fake.NeverStops(),                  // 不回 stopReason → 测取消超时
)
```

**任何需要 Agent 行为的测试都用它**，绝不起真实 `claude-agent-acp` / `codex-acp`。

### 临时数据目录

```go
paths := testutil.TempPaths(t)   // 内部用 t.TempDir()，自动清理
```

`testutil` 里装了守卫：测试进程一旦尝试打开 `$HOME/.acpflows` 就 `t.Fatal`。
**不要绕过它**（铁律 6）。

### 临时 git 仓库

```go
repo := testutil.NewGitRepo(t)   // 现场 git init + 几个 commit
```

绝不碰用户真实仓库。

## 确定性：三个必须注入的东西

| 东西 | 生产实现 | 测试实现 |
|---|---|---|
| 时间 | `platform.SystemClock` | `testutil.FixedClock(t0)` |
| ID | `platform.ULIDGen` | `testutil.SeqIDGen("unit-%03d")` |
| 随机 | 注入的 `rand.Source` | 固定种子 |

**domain / app 层里出现 `time.Now()` 或 `rand` 直接调用 = 测试变成薛定谔的。**
lint 会拦。

## 数据库用临时文件，不用 `:memory:`

```go
db := testutil.TempSQLite(t)     // 临时文件，不是 :memory:
```

`:memory:` 测不出 WAL 行为和并发，而这个产品会并发跑多个 Work。

## 假测试图鉴 —— 写完对着自查

完整版见 [`docs/testing-strategy.md`](../../docs/rules/testing-strategy.md) §3。速查：

| ✗ | 为什么假 |
|---|---|
| `repo.On("Find").Return(x)` 然后断言拿到 `x` | mock 喂 mock，只证明了 mock 库能工作 |
| 唯一断言是 `assert.NoError(err)` | 只测了"没崩" |
| `assert.NotNil` / `assert.True(len>=0)` | 恒真，不可能失败 |
| 调完方法就结束，不验证状态 | 只运行不断言 |
| 只有 happy path | 边界、错误、幂等都没测 |
| 断言 SQL 行数 | 抽象层错了，绑死持久化细节 |
| 测私有琐碎 helper | 覆盖率好看，没验证任何对外承诺 |

**最好用的自查：把实现里的关键一行删掉，这个测试会红吗？**

## 每个测试至少覆盖四类用例

```go
tests := []struct{ name string; ... }{
    {"正常路径", ...},
    {"边界值", ...},
    {"错误路径", ...},
    {"幂等（若适用）", ...},
}
```

## 收尾：登记测试索引

新增顶层 `func TestXxx` → 在 [`backend/tests/INDEX.md`](../../backend/tests/INDEX.md) 加一行。

```bash
make check-test-index
```

**登记前先搜一遍**：如果和已有测试实质重复，应该**合并**而不是补登记。
`TestCancelWorks` / `TestCancelSuccess` / `TestSessionCancelBasic` 这种三胞胎就是这么来的。

## 命令

```bash
cd backend && go test ./internal/acp/... -run TestSessionCancel -count=1 -v
cd backend && go test ./... -race -count=1
cd backend && go test -tags=integration ./tests/... -count=1
make cover                      # 覆盖率 + 分包门槛
```

`-count=1` 禁用缓存 —— 缓存过的绿色不算验证。

## 覆盖率门槛

`domain` 90% · `acp` 80% · `app` 75% · `store` 70%。

**覆盖率是下限不是目标。** 90% 配一堆恒真断言等于 0%。
它只用来发现"完全没测的地方"。
