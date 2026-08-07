# AGENTS.md · shell

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

Tauri v2（Rust）外壳。**只做四件事**：

1. 无边框窗口 + 自绘 42px 窗口栏的拖拽区（`decorations: false`）
2. 原生文件/文件夹选择器、在 Finder 中显示、用外部编辑器打开
3. **sidecar 生命周期**：拉起 `duetd`、健康检查、崩溃重启、退出时优雅关闭
4. **自动更新**（updater 插件），见 [`../docs/spec/release-and-update.md`](../docs/spec/release-and-update.md)

## 不负责什么

**壳里不写业务逻辑、不做数据持久化、不直接和 ACP Runtime 说话。**

任何想写进 Rust 的业务需求，先问一个问题：**为什么它不能是 duetd 的一个 API？**
答不上来就说明它应该在后端。

### 最重要的一条约束

**Tauri 不得通过 Rust IPC 绕过 HTTP 调用后端逻辑。**

前端访问后端的方式必须和浏览器完全一样：`fetch` + `EventSource` 打 `127.0.0.1:<port>`。
破坏这条，Web 版当天就废——而 Web 版是 AI 自测的主要通道。

Tauri command 只允许暴露"浏览器根本做不到"的能力（原生对话框、Finder、窗口控制、updater）。

## 结构

```
shell/
├── src-tauri/
│   ├── src/
│   │   ├── main.rs          入口
│   │   ├── sidecar.rs       duetd 生命周期
│   │   ├── commands/        Tauri command，一个能力一个文件
│   │   ├── models/          ★ 数据结构
│   │   ├── constants/       ★ 常量
│   │   └── utils/           ★ 工具
│   ├── binaries/            构建时放入的 duetd 二进制（带 target triple 后缀）
│   └── tauri.conf.json      ★ 版本号真源，由 release-please 维护，不手改
└── package.json
```

## 端口与令牌传递

`duetd` 启动后把 `{port, token, pid}` 写入 `~/.acpflows/runtime/session.json`（`0600`）。
壳读它，注入 WebView 的 `window.__DUET__`。

**令牌绝不写进日志、绝不进入前端可持久化的存储。**

## 检查命令

```bash
cd shell/src-tauri && cargo clippy --all-targets -- -D warnings
cd shell/src-tauri && cargo test
make -C .. dev-app          # tauri dev
make -C .. build-app        # 需 TAURI_SIGNING_PRIVATE_KEY
```

## 改这里之前必读

- [`../docs/spec/architecture.md`](../docs/spec/architecture.md) §2 进程模型、§6 壳的职责
- [`../docs/spec/release-and-update.md`](../docs/spec/release-and-update.md) —— 更新流程，尤其 `prepare` 那一步
- [`../docs/rules/coding-standards.md`](../docs/rules/coding-standards.md) §5 Rust 规范

## 本域特有的坑

- **`unwrap()` / `expect()` 在非测试代码里禁用**（`clippy::unwrap_used` = deny）。
  壳崩了整个 App 就没了。
- **sidecar 僵尸进程。** App 崩溃时 `duetd` 可能存活；下次启动必须靠 pid 文件清理。
- **更新前必须先调后端 `prepare`**，不是先下载。
  设计稿写死了「更新前会暂停所有工作并保存检查点」——这不是形式，是领域操作。
- **`prepare` 返回 `blocked` 时不许继续安装**，要把卡住的工作显示给用户。
- **minisign 私钥丢失 = 所有已安装客户端再也收不到更新**（公钥硬编码在旧版本里）。
- **版本号只在 `tauri.conf.json`**，由 release-please 维护，任何人不得手改。
- 开发时优先用 `make dev-web`（不需要 Rust 工具链），只在改本目录时才用 `dev-app`。
