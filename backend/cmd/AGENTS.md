# AGENTS.md · backend/cmd

> **就近优先**。仓库总纲见根 [`AGENTS.md`](../../AGENTS.md)。

## 负责什么

可执行程序入口。**这是全仓库唯一做依赖装配的地方。**

```
cmd/
├── duetd/      主进程：HTTP API + SSE + ACP 编排
└── acpprobe/   真机探针（M0 S0.3）：只读收集 runtime 能力，零模型开销
```

## 不负责什么

- **不写业务逻辑。** `main.go` 只做：读配置 → new 各层 → 接线 → 启动 → 等信号 → 优雅关闭
- **不做 DI 框架。** 手工 `new`，一眼能看出谁依赖谁。引 DI 框架会让依赖关系变隐式

## 装配顺序

```
platform（Clock/IDGen/Paths）
  → store（DB 连接 + 迁移）
  → acp（Runtime 注册表）· gitx · ghx · fsstore · eventbus
  → app（用例，注入上面的 port 实现）
  → api（HTTP handler，注入 app）
  → 启动 HTTP server
```

**反向依赖一律拒绝。** 装配顺序就是依赖方向的证明——如果某一层需要它下面的层
先构造好，说明分层错了。

## 关闭顺序（与启动相反）

```
停止接受新请求 → 等在途请求结束 → 关闭 ACP 子进程（SIGTERM→SIGKILL）
  → 关闭 SSE 订阅 → 关闭 DB → 删 pid 文件
```

**ACP 子进程必须优雅关闭。** 漏了会留下僵尸进程，下次启动靠 pid 文件清理。

## 检查命令

```bash
cd backend && go build ./cmd/...
make -C .. dev-web            # 冒烟：起得来且有响应
```

## 本域特有的坑

- **`main.go` 超过 150 行就该抽 `wire.go` 之类的装配文件**，但仍然手写、不引框架
- **配置只从环境变量与命令行读**，不读配置文件——桌面应用的配置在 DB 里
- **信号处理不能漏 `SIGTERM`**：Tauri 关闭 sidecar 时发的是它
- 探针 `acpprobe` **只读**：不写目标目录、不碰 `~/.claude` 与 `~/.codex`
