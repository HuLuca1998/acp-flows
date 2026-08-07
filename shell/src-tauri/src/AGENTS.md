# AGENTS.md · shell/src-tauri/src

> 本目录的规则。**就近优先**：与上级 [`../../AGENTS.md`](../../AGENTS.md) 冲突时以本文件为准。

## 负责什么

Tauri 外壳的 Rust 代码。**只做四件事**：

| 文件 | 负责 |
|---|---|
| `lib.rs` | 装配：插件、sidecar、托盘、窗口事件 |
| `sidecar.rs` | 拉起 `duetd`、等 `session.json`、把端口与 token 注入前端 |
| `tray.rs` | 菜单栏常驻与退出策略 |

## 不负责什么

- **不写业务逻辑、不做持久化、不直接和 ACP Runtime 说话。** 那些全在 `duetd` 里。
- ★ **不得通过 Rust IPC 绕过 HTTP 调用后端。**
  前端访问后端的方式必须和浏览器完全一样（fetch + EventSource 打 127.0.0.1）。
  破坏这条，Web 版当天就废——而 Web 版是 AI 自测的主要通道。
  注入前端的是**端口与 token**，不是 IPC 通道。

## 检查命令

```bash
cd shell/src-tauri && cargo build
cd shell/src-tauri && cargo clippy --all-targets -- -D warnings
```

打一个真包出来看（这是唯一能验证 sidecar 与托盘的方式）：

```bash
cd shell && APPLE_SIGNING_IDENTITY='-' \
  TAURI_SIGNING_PRIVATE_KEY="$(cat ~/.duet-updater/updater.key)" \
  TAURI_SIGNING_PRIVATE_KEY_PASSWORD="" \
  pnpm exec tauri build --bundles app,dmg
```

## 改这里之前必读

- [`docs/spec/architecture.md`](../../../docs/spec/architecture.md) §1 —— 进程模型与那条 IPC 红线
- [`docs/spec/release-and-update.md`](../../../docs/spec/release-and-update.md) —— 签名与发布链路
- [`docs/plan/milestones/M1-install-and-update.md`](../../../docs/plan/milestones/M1-install-and-update.md)

## 本域特有的坑

- **`unwrap` / `expect` 被 clippy 拒绝**（`Cargo.toml` 里 deny）。
  壳崩了整个 App 就没了，错误一律显式处理并打印可诊断的信息。
- **`beforeBuildCommand` 的 cwd 是 `shell/`，不是 `src-tauri/`。**
  而 `frontendDist` 是相对 `tauri.conf.json`。两个基准不一样，
  写成同一个相对路径必然有一个是错的——踩过一次
  （报错是 `ENOENT: /Users/…/work/frontend`，少了一层目录）。
- **sidecar 启动前要删掉旧的 `session.json`。**
  上次运行的残留会让壳连到一个已经死掉的端口，
  症状是「界面一片空白但没有任何报错」。
- **`duetd` 的 stderr 必须收走。** 端口被占、数据库锁死、迁移失败的原因
  **只在 stderr 里**。不收的话用户只看到一个白窗口。
- **拉 sidecar 要传 `--updater`。** 不传的话后端认为自己跑在浏览器形态，
  检查更新永远返回 `unsupported`，装出来的 app 更新不了。
- **托盘图标必须用 `icons/tray@2x.png` 这张 template 图**，
  不能用 `default_window_icon()`——那是彩色带底板的 Dock 图标，
  塞进菜单栏是一个突兀的方块。同时要 `icon_as_template(true)`。
- **关窗口不退出应用。** 后台可能有正在跑的 Agent 与未落盘的工作，
  点一下红叉全杀掉，用户会毫无预警地丢掉几十分钟的活。
  `CloseRequested` 里 `prevent_close()` + `hide_to_tray()`，真正退出只走托盘菜单。
- **收进菜单栏要连 Dock 图标一起收。** 只 `hide()` 窗口的话 Dock 里还占着一格，
  用户看到的是「关了但没关干净」。macOS 上要切 `ActivationPolicy::Accessory`；
  唤回窗口前必须切回 `Regular`——**Accessory 策略下窗口拿不到焦点**，
  只 `show()` 会得到一个点不动的窗口。
- **托盘图标别配两遍。** `tauri.conf.json` 的 `trayIcon` 会自动建一个，
  代码里 `TrayIconBuilder` 再建一个 → **菜单栏出现两个图标**。
  要挂菜单与事件就只在代码里建，配置里那段删掉。
- **`decorations: false` 是完全无边框**：没有交通灯、没有 macOS 的窗口圆角，
  用户看到的是一个方方正正、找不到关闭按钮的窗口。
  要自绘窗口栏又保留系统按钮，用 `decorations: true` + `titleBarStyle: "Overlay"`，
  并且**必须加 `hiddenTitle: true`** —— 否则系统画的标题会压在窗口栏内容上。
- ★ **`data-tauri-drag-region` 只对标了它的元素本身生效，不向上找父元素。**
  只标 `<header>` 的话会「按空白处能拖、按到里面的文字上就拖不动」，
  表现为时灵时不灵。窗口栏里**每个非交互子元素都要单独标**，
  交互元素（按钮）则一个都不能标。
  另外还要在 capabilities 里放行 `core:window:allow-start-dragging`——
  少了它属性形同虚设，**拖不动且不报任何错**。
