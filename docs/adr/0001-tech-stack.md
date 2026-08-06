# ADR 0001 · 技术栈选型

- **日期**：2026-08-07
- **状态**：已接受
- **决策人**：项目所有者

> ADR 只增不改。决定变了写新的 ADR，在旧的开头加 `> 已被 adr/00XX 取代`。

## 背景

从零开始做 Duet —— 一个 ACP 多智能体协作编程 App。形态是 macOS 桌面应用，
但**开发与测试期望能在浏览器里进行**（这条是硬需求：项目由 AI 维护，AI 自测通道必须简单）。

## 决策

### 1. 应用外壳：Tauri v2（Rust）+ Go sidecar

**候选与取舍**

| 方案 | 否决理由 |
|---|---|
| Go 单进程 + 薄 WebView 壳 | 原生能力弱，窗口栏自绘与文件选择器要自己啃 |
| **Tauri v2 + Go sidecar** ✅ | 原生体验最好；代价是引入 Rust 工具链 |
| Electron + Go sidecar | 包体积 100MB+，与设计稿的轻量气质不符 |
| 纯 Web，暂不做壳 | 窗口栏、原生文件选择器、自动更新全都做不了 |

**关键约束（比选型本身更重要）**：

> `duetd` 永远是一个独立可运行的 HTTP 服务。
> **Tauri 不得通过 Rust IPC 绕过 HTTP 调用业务逻辑。**

守住它，"macOS App" 与 "Web 版" 永远是同一份代码；破坏它，Web 版当天就废，
AI 自测通道随之报废。

### 2. 前端：React 18 + TypeScript + Vite

设计稿的原型 runtime 本身就是 React 驱动的；生态最大，复杂状态（事件流、DAG、
抽屉栈、可拖拽分栏）的成熟方案最多。状态管理：TanStack Query（服务端）+ Zustand（UI）。

### 3. API 契约：OpenAPI 3.1 手写 spec 先行

| 方案 | 否决理由 |
|---|---|
| **OpenAPI 3.1 spec 先行** ✅ | spec 即测试基准，前后端可并行，AI 最容易照着自查 |
| Protobuf + Connect-RPC | 类型更强、流式原生，但引入 buf 工具链，浏览器调试不如 JSON 直观 |
| Go 代码为真源反向生成 | 前端必须等后端写完，**与测试先行相冲突** |

事件流用 SSE 单独约定（OpenAPI 表达不了流），schema 仍定义在 spec 里。

### 4. 后端：Go，单进程，SQLite

- SQLite 驱动用 `modernc.org/sqlite`（纯 Go 无 CGO），交叉编译与 CI 简单
- HTTP 用标准库 `net/http`（Go 1.22+ 路由模式），少一个依赖
- 分层：`api → app → domain`，基础设施实现 `app` 定义的 port 接口，由 `depguard` 强制

### 5. 首个里程碑：ACP 协议层先行

没有 Fake ACP Runtime，上层一切都测不了。它是整套测试策略的支点，必须第一个做。

## 后果

**得到**

- 一份代码三种形态：`dev-web`（默认开发形态）/ `dev-app` / `release`
- AI 用 `make dev-web` 自测，**不需要 Rust 工具链**
- spec 先行让前后端可并行，也让契约测试有基准

**付出**

- 引入 Rust 工具链（仅 `shell/` 需要，日常开发用不到）
- 跨进程调用比 Tauri 原生 IPC 慢一点点（本地回环，可忽略）
- OpenAPI 手写 spec 有维护成本，靠 `make check-gen` 在 CI 兜底

**风险**

- Tauri v2 的 sidecar 与 updater 生态相对年轻，遇到坑要有心理准备
- 「不得绕过 HTTP」这条约束需要持续守护 —— 一旦破坏，损失是不可逆的
