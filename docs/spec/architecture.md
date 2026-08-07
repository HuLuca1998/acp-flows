# 架构总览

> 读者：Claude / Codex / 人。改任何跨层的东西之前先读完本文。
> 决策依据见 [`adr/`](../adr/)。本文描述**目标架构**；当前实现进度见 [`roadmap.md`](../plan/roadmap.md)。

---

## 1. 一句话架构

**一个 Go 进程（`duetd`）承载全部业务逻辑，对外只暴露 HTTP + SSE。
其他一切——Tauri 壳、浏览器、未来的 CLI——都只是这套 API 的客户端。**

这条约束是整个架构的支点。守住它，"macOS App"和"Web 版"就永远是同一份代码；
破坏它（比如让 Tauri 通过 Rust IPC 直接调用业务逻辑），Web 版当天就废掉。

---

## 2. 进程模型

```
┌─────────────────────────────────────────────────────────────┐
│ Duet.app  (Tauri v2 / Rust)                                 │
│                                                             │
│  ┌──────────────────────┐      ┌───────────────────────┐    │
│  │ WKWebView            │      │ Rust 主进程            │    │
│  │  React 18 SPA        │      │  · 无边框窗口 + 自绘窗口栏│    │
│  │                      │      │  · 原生文件/文件夹选择   │    │
│  │  fetch / EventSource │      │  · 在 Finder 中显示     │    │
│  └──────────┬───────────┘      │  · updater 插件         │    │
│             │                  │  · sidecar 守护          │    │
│             │ HTTP/SSE         └───────────┬───────────┘    │
│             │ 127.0.0.1:<随机端口>          │ spawn          │
│             ▼                              ▼                │
│  ┌──────────────────────────────────────────────────────┐   │
│  │ duetd  (Go, sidecar 子进程)                           │   │
│  │   HTTP API  ·  SSE 事件流  ·  SQLite  ·  文件系统      │   │
│  └───────────────────────┬──────────────────────────────┘   │
└──────────────────────────┼──────────────────────────────────┘
                           │ JSON-RPC over stdio
              ┌────────────┴────────────┐
              ▼                         ▼
    claude-agent-acp            codex-acp
    (ACP Runtime 子进程)         (ACP Runtime 子进程)
```

### 三种运行形态，同一份代码

| 形态 | 命令 | 组成 | 用途 |
|---|---|---|---|
| `dev-web` | `make dev-web` | `duetd serve` + `vite dev`（proxy 到 duetd） | **日常开发与 AI 自测的默认形态**。浏览器打开即可，无需 Rust 工具链 |
| `dev-app` | `make dev-app` | `tauri dev` → 拉起 duetd sidecar + vite dev | 验证原生能力、窗口栏、自动更新 |
| `release` | CI 构建 | `Duet.app`：Tauri 二进制 + 内嵌 duetd + 内嵌前端 dist | 交付 |

**AI 协作者默认在 `dev-web` 下工作。** 只有涉及 `shell/` 或自动更新时才需要 `dev-app`。

### 本地回环安全

`duetd` 只监听 `127.0.0.1`，启动时：

1. 绑定随机端口（`:0`）
2. 生成一次性 bearer token
3. 把 `{port, token, pid}` 写入 `~/.acpflows/runtime/session.json`（权限 `0600`）

Tauri 读该文件，把 `port` 与 `token` 注入 WebView（`window.__DUET__`）。
`dev-web` 模式下端口固定 `7777`、token 从 `DUET_DEV_TOKEN` 环境变量读，便于调试。

所有 API 请求带 `Authorization: Bearer <token>`。**没有 token 的请求一律 401**，
包括来自本机浏览器的——防止本机其他程序静默驱动 Agent 写用户的代码。

---

## 3. 后端分层（Go）

### 依赖方向

```
        ┌──────────────────────────────────────┐
        │  internal/api      HTTP 传输层         │  实现 openapi 生成的 ServerInterface
        └──────────────┬───────────────────────┘
                       ▼
        ┌──────────────────────────────────────┐
        │  internal/app      用例编排 / 事务边界  │  定义 port 接口
        └──────────────┬───────────────────────┘
                       ▼
        ┌──────────────────────────────────────┐
        │  internal/domain   领域模型 / 状态机    │  零 IO、零框架、纯 Go
        └──────────────────────────────────────┘
                       ▲
                       │ 实现 app 定义的 port 接口
        ┌──────────────┴───────────────────────┐
        │ store · acp · gitx · ghx · fsstore   │  基础设施
        └──────────────────────────────────────┘
```

**规则：**

- `domain` 不 import 任何本仓库的其他 internal 包，不做 IO，不认识 SQL / HTTP / JSON-RPC。
  → 它 100% 可用表驱动单元测试覆盖，这是测试策略的地基。
- `app` 定义它需要的 port 接口（`WorkRepo`、`RuntimeGateway`、`GitGateway`…），
  基础设施包去实现。`app` 永远不 import 基础设施包的具体类型。
- `api` 只做协议翻译：解析请求 → 调 `app` → 序列化响应。**不写业务逻辑。**
- 依赖注入在 `cmd/duetd/main.go` 手工组装，不引入 DI 框架。

### 包职责

### 固定目录：模型 / 常量 / 工具

三个专用文件夹，全仓库统一，准入规则见 [`coding-standards.md`](../rules/coding-standards.md) §1：

| | Go | TypeScript |
|---|---|---|
| 模型 | `internal/domain/model/`（一个聚合一个文件，充血） | `src/models/` |
| 常量 | `internal/constant/`（按主题分文件） | `src/constants/` |
| 工具 | `internal/util/`（纯函数、零业务语义、≥2 调用方、有测试） | `src/utils/` |

`util/` 是最容易腐化成垃圾桶的地方——`util.go` `helper.go` `common.go` `misc.go`
这四个文件名由 CI 直接拦下。

### 包职责

| 包 | 职责 | 不做什么 |
|---|---|---|
| `internal/domain/model` | 聚合根、值对象、状态机、不变量校验 | 任何 IO |
| `internal/domain/policy` | 跨聚合的业务策略（决策等级判定、注入选择、DAG 校验） | 任何 IO |
| `internal/app` | 用例编排、事务边界、领域事件发布 | 直接写 SQL、直接起进程 |
| `internal/api` | HTTP handler、SSE、鉴权中间件、错误映射 | 业务判断 |
| `internal/acp` | ACP JSON-RPC 客户端、会话生命周期、Runtime 适配、能力探针、**Fake Runtime** | 业务状态机 |
| `internal/store` | SQLite 读写、migrations、查询 | 业务规则 |
| `internal/fsstore` | `.acpflows/` 下 md 文件读写、frontmatter 解析、与 DB 索引对账 | 业务规则 |
| `internal/gitx` | worktree 创建/清理、diff 采集、commit、branch | GitHub 远端操作 |
| `internal/ghx` | PAT 存取（加密）、push、PR、按 remote 匹配账号 | 本地 git 操作 |
| `internal/eventbus` | 领域事件扇出 → SSE 订阅者 + 运行日志 | 事件语义 |
| `internal/platform` | 路径解析、keychain、可执行文件探测、进程管理 | 业务规则 |

### 存储：md 是内容，DB 只存索引与状态

这是设计规范写死的原则，不要动摇。

| 数据 | 落在哪 | 说明 |
|---|---|---|
| 记忆正文 | `<project>/.acpflows/memory/<id>.md` | frontmatter + 正文，人可读可编辑，可入 git |
| 记忆索引与状态 | SQLite | `scope` `kind` `status` `confidence` `source_refs` `注入统计` |
| Skill | `<project>/.acpflows/skills/<name>/SKILL.md` + `scripts/` `references/` `assets/` | 同上 |
| 项目配置 | `<project>/.acpflows/project.yaml` | |
| 运行记录 | `<project>/.acpflows/runs/` | **写进 `.gitignore`**，不入库 |
| Work / Plan / Unit / Contract / Attempt / Evidence / Decision / Checkpoint | SQLite `~/.acpflows/duet.db` | 结构化状态 |
| GitHub PAT | `~/.acpflows/credentials`（加密） | **绝不写入任何项目目录，绝不进入 Agent 上下文** |

持久化用 **GORM + SQLite**，驱动是 `github.com/glebarez/sqlite`（纯 Go，无 CGO），
保证交叉编译与 CI 简单。**只做 SQLite，不做 MySQL** —— 理由见 [`adr/0005-persistence.md`](../adr/0005-persistence.md)。
表结构、实体定义、迁移规范见 [`database.md`](../rules/database.md)。
启动时执行 **DB ↔ 文件对账**：文件被人手改过或删过，索引要能自愈并上报差异。

---

## 4. 事件流

前端靠一条 SSE 连接拿到全部实时更新。

```
GET /v1/works/{workId}/events        # SSE
Last-Event-ID: <seq>                 # 断点续传
```

统一事件信封：

```json
{
  "id": "evt_01J...",
  "seq": 1423,
  "work_id": "work-08",
  "source": "acp" | "app",
  "type": "message_chunk",
  "ts": "2026-08-07T03:12:44.512Z",
  "payload": { }
}
```

`type` 是**封闭枚举，共 13 类**，与设计规范第 07 节一一对应：

| source | type | 展示形态 |
|---|---|---|
| `acp` | `message_chunk` | 徽标 + 角色名 + 正文，左侧 2px 角色色竖条 |
| `acp` | `thought_chunk` | 斜体单行，可折叠 |
| `acp` | `tool_call` | inset 表格：类型 · 目标 · +/− 或运行中圆环 |
| `acp` | `request_permission` | 描边卡 + 允许一次 / 拒绝，**阻塞当前轮** |
| `acp` | `turn_end` | 居中细线 + 等宽小字，默认折叠 |
| `app` | `plan_version` | 竖条单行 + 「查看差异」 |
| `app` | `unit_contract` | 竖条单行 + 「查看契约」 |
| `app` | `state_change` | 居中分隔线上的等宽小字，默认隐藏 |
| `app` | `injection` | 灰色单行 + ID 芯片 |
| `app` | `memory_candidate` | inset 卡 + 「审核」，**绝不自动写入** |
| `app` | `decision` | 描边卡：等级角标 + 选项 + 推荐标记 |
| `app` | `evidence` | 单行 ✓ 计数 + 「打开证据」 |
| `app` | `checkpoint` | 居中 flag 图标 + ck 编号 + commit hash |

**约束：**

- 每一类事件都必须能被前端过滤器开关（前端的过滤器 UI 直接由这个枚举生成）。
- `app` 事件**永远可点开到对应的结构化产物**（计划 / 契约 / 证据 / 记忆）；`acp` 事件只做展示与折叠。
- 不展示模型私有思维链原文，只展示摘要。
- `seq` 单调递增且持久化——「取消后最后事件游标可读」是产品的硬需求，不是可选项。

新增第 14 类事件 = 改设计规范 + 改 OpenAPI + 改前端注册表，三处缺一不可。

---

## 5. 前端结构（React 18 + TS + Vite）

```
frontend/src/
├── app/          路由、providers、布局骨架（窗口栏 42 / 左栏 252 / 主区 / 右栏 300）
├── design/       tokens.css（Nocturne 令牌）+ duet.css（产品层令牌 --s-canvas 等）
├── ui/           设计系统原语：Button Tag Card Drawer Dialog Dropdown Progress Switch Icon …
├── features/
│   ├── conversation/   对话页 + 13 类事件渲染器（注册表模式）
│   ├── plan/           悬浮计划面板、子计划 DAG、重规划记录
│   ├── memory/         记忆页（L2/L3）
│   ├── skill/          Skill 页（L4）
│   ├── report/         报表页
│   ├── settings/       Runtime / 环境检测 / 应用更新 / 项目 / GitHub / 通用
│   ├── roles/          角色与 Runtime 绑定
│   └── project/        创建项目、新建工作
├── api/          由 openapi.yaml 生成的 client + SSE 订阅封装
├── platform/     ★ 平台适配层（Tauri / Web 双实现）
├── store/        Zustand slices（UI 状态）+ TanStack Query（服务端状态）
├── models/       ★ 领域类型（多数由 openapi 生成物再包装）
├── constants/    ★ 常量与枚举（as const 对象，禁用 enum）
└── utils/        ★ 工具函数（纯函数，准入规则同后端）
```

### 平台适配层是 Web 版能活下来的关键

前端**除 `src/platform/` 外，任何地方都不许 import `@tauri-apps/*`**。ESLint 规则强制。

```ts
interface PlatformAdapter {
  pickDirectory(opts): Promise<string | null>
  pickFile(opts): Promise<string | null>
  revealInFinder(path): Promise<void>
  openInEditor(path): Promise<void>
  updater: UpdaterPort          // 见 release-and-update.md
  window: WindowControlPort     // 最小化/最大化/关闭，Web 下为 no-op
}
```

| 能力 | Tauri 实现 | Web 降级 |
|---|---|---|
| 选择文件夹 | 原生 dialog | 手输路径 + 后端校验存在性 |
| 在 Finder 中显示 | `revealItemInDir` | 复制路径到剪贴板 + toast |
| 在编辑器打开 | `opener` 插件 | 复制路径 |
| 窗口控制 | 原生 | 隐藏窗口栏按钮 |
| 自动更新 | updater 插件 | 显示「有新版本」但按钮不可用，附下载链接 |

Web 降级**必须真的可用**，不能是空实现——它是 AI 自测的主要通道。

### 状态管理分工

- **服务端状态** → TanStack Query（列表、详情、变更）
- **实时事件流** → 单条 SSE 连接 → 写入 Zustand 的 event slice → 组件订阅切片
- **纯 UI 状态**（栏宽、折叠、抽屉栈、过滤器）→ Zustand，持久化到 localStorage

不要把 SSE 事件塞进 TanStack Query 缓存，两套失效模型混在一起必炸。

---

## 6. Tauri 壳只做四件事

1. 无边框窗口 + 自绘 42px 窗口栏的拖拽区（`decorations: false`）
2. 原生文件/文件夹选择器、Finder 揭示、外部编辑器打开
3. **sidecar 生命周期**：拉起 `duetd`、健康检查、崩溃重启、退出时优雅关闭
4. **自动更新**（updater 插件），流程见 [`release-and-update.md`](release-and-update.md)

**壳里不写业务逻辑，不做数据持久化，不直接和 ACP Runtime 说话。**
任何想写进 Rust 的业务需求，都应该先问：为什么它不能是 duetd 的一个 API？

---

## 7. 目录规划总表

| 路径 | 内容 | 谁改 |
|---|---|---|
| `api/openapi.yaml` | 前后端契约唯一真源 | 接口变更时，**先改这里** |
| `backend/` | Go 后端 | 后端任务 |
| `frontend/` | React 前端 | 前端任务 |
| `shell/` | Tauri 壳 | 原生能力 / 更新相关任务 |
| `e2e/` | Playwright | QA 任务 |
| `design/` | 设计稿与设计系统 | **只读**。改设计要回 Claude Design 项目改，再同步下来 |
| `docs/` | 架构与规范 | 架构变更时同步更新，文档滞后视为缺陷 |
| `docs/adr/` | 架构决策记录 | 每个不可逆的技术决定加一条 |
| `scripts/` | 构建、代码生成、起服务 | |
| `.github/workflows/` | CI 与自动发版 | |

---

## 8. 尚未决定的事

这些是**明确的开放项**，不要在实现里替它们做主张，需要时提出来：

1. **Apple 开发者证书与公证** —— 当前策略是 ad-hoc 签名，首次安装需用户手动放行。
   要不要上 $99/年的 Apple Developer Program，待定。见 `adr/0002`。
2. **ACP 协议版本兼容策略** —— `protocolVersion 1` 之外的版本怎么处理，等真实 Runtime 接进来再定。
3. **跨项目记忆（L3）的去标识化规则** —— 设计稿说"去项目标识后送审"，具体脱敏规则待定。
4. **多 Work 并发上限** —— 同时能跑几个 worktree / Runtime 进程，等性能数据再定。
