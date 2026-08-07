# AGENTS.md · backend/tests/testutil

> **就近优先**。测试策略见 [`../../../docs/rules/testing-strategy.md`](../../../docs/rules/testing-strategy.md)。

## 负责什么

测试专用的夹具与辅助：隔离守卫、临时路径、确定性时钟与 ID 生成器。

```
testutil/
├── guard.go          ★ 铁律 6 的落点：拦住访问用户真实数据的测试
├── paths.go            TempPaths —— 把全部数据目录重定向到 t.TempDir()
└── deterministic.go    FixedClock · SeqIDGen —— 消除时间与随机的不确定性
```

## 不负责什么

- **不放业务夹具。** 建 git 仓库、造 Work 数据这类放 `../fixtures/`
- **不放生产代码的工具函数。** 那是 `internal/util`

## ★ 与 `internal/util` 是两回事

| | 用途 | 谁能 import |
|---|---|---|
| `internal/util` | 生产代码的纯工具函数 | 生产代码 + 测试 |
| `tests/testutil` | 建夹具、起临时服务、断言辅助 | **只有测试** |

生产代码 import 本包会被 `depguard` 拦下（`scripts/check-naming.sh` 也查一遍）。
**搞混这两个是本目录最常见的错误。**

## 守卫必须自己有测试

`guard.go` 是铁律 6 的唯一执行点。**一个失效的守卫比没有守卫更糟**——
它制造了「测试碰不到真实数据」的错觉。

所以 `guard_test.go` 要同时测两个方向：

1. 真实数据目录**必须被拦下**，且错误信息指向铁律 6
2. 临时目录**必须放行**（包括 `<tmp>/.acpflows` 这种同名路径），否则所有测试都跑不了

改守卫时两个方向都要补用例。

## 确定性三件套

| 不确定性 | 生产实现 | 这里的测试实现 |
|---|---|---|
| 时间 | `platform.NewClock()` | `FixedClock(T0)`，可 `Advance` |
| ID | `platform.NewIDGen()` | `SeqIDGen()`，按前缀递增 |
| 路径 | `platform.NewPaths()` | `TempPaths(t)` |

**`TempPaths` 每次调用给出不同目录。** 测试之间不共享状态——
一个测试写脏的数据让另一个测试莫名失败，是最难排查的一类问题。

## 检查命令

```bash
cd backend && go test ./tests/testutil/... -count=1
```

## 本域特有的坑

- **新增受保护路径时要同时补 `guard_test.go` 的用例**，否则守卫扩了但没人验证
- **ULID 的测试实现必须单调递增**：事件表按它排序，不递增会让断点续传的测试失去意义
- **`CheckPathAllowed` 在拿不到家目录时放行**，不是拦下——宁可漏判也不要在 CI
  这类没有家目录的环境里把所有测试打死
