# AGENTS.md · backend/internal/platform

> **就近优先**。仓库总纲见根 [`AGENTS.md`](../../../AGENTS.md)。

## 负责什么

与**运行环境**打交道的东西：路径、时间、ID、进程、凭据存储、可执行文件探测。

它实现 `app/port` 里的 `Clock` / `IDGen` / `Paths` 等系统能力接口。

```
platform/
├── clock.go     SystemClock（生产实现）
├── ids.go       ULID / 前缀序列 ID 生成
├── paths.go   ★ ~/.acpflows 及其子路径的唯一来源
├── proc.go      进程探测、pid 文件、信号
└── keychain.go  凭据加密存取
```

## 不负责什么

- **不做业务判断** —— 「该不该写这个文件」是 `app` 的事
- **不做 git / ACP / HTTP** —— 那些在 `gitx` / `acp` / `ghx`
- **不做纯计算** —— 路径的纯拼接与规范化在 `util/pathx`，这里只做真的碰文件系统的部分

## 为什么这个包必须存在 ★

**所有不确定性的入口都收在这里**，这样测试才可能是确定的：

| 不确定性 | 生产实现 | 测试实现 |
|---|---|---|
| 时间 | `SystemClock` | `testutil.FixedClock(t0)` |
| ID | `ULIDGen` | `testutil.SeqIDGen("unit-%03d")` |
| 路径 | `~/.acpflows` | `t.TempDir()` |
| 随机 | 注入的 source | 固定种子 |

**`domain` 与 `app` 里出现裸 `time.Now()` / `rand` / `os.UserHomeDir()`，
测试就变成薛定谔的。** 由 lint 拦。

## 铁律 6 的落点

`Paths` 是 `~/.acpflows` 的**唯一来源**。任何地方直接拼
`os.UserHomeDir() + "/.acpflows"` 都是违规。

`tests/testutil` 里的守卫会在测试进程访问真实数据目录时 `t.Fatal`。
**这个守卫本身必须有测试**，否则它可能一直是失效的。

## 检查命令

```bash
cd backend && go test ./internal/platform/... -count=1
```

## 本域特有的坑

- **`~` 展开要用 `os.UserHomeDir()`，不要用 `os.Getenv("HOME")`** —— 后者在某些
  启动方式下是空的（Tauri 拉起 sidecar 时尤其要注意）
- **凭据绝不写日志、绝不进 Agent 上下文。** 加密存 `~/.acpflows/credentials`
- **进程探测要用绝对路径**，`node_modules/.bin/` 不一定在 PATH 里；
  且要允许环境变量覆盖（方便指向特定版本）
- **杀子进程要考虑孙进程。** codex-acp 会 spawn `codex app-server`，
  只杀父进程会留下孤儿——按进程组杀
