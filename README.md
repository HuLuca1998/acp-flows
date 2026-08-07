# Duet

**ACP 多智能体协作编程 App。**

把 `claude-agent-acp` 与 `codex-acp` 两个 ACP Runtime 当作可编排的执行体，
用「计划 → 子计划 → 单元契约 → 尝试 → 证据 → 检查点」的状态机驱动它们协作写代码，
人只在 D2/D3 决策点介入。

macOS 桌面应用（Tauri 壳 + Go sidecar），同一份代码可作为纯 Web 运行。

> **状态：M0 进行中**，脚手架四件套（backend / frontend / shell / e2e）已跑通，
> `make check` 全绿。剩余关键路径是 Fake ACP Runtime。
> 进度见 [`docs/roadmap.md`](docs/roadmap.md)，分章见 [`docs/milestones/`](docs/milestones/)。

---

## 这个仓库由 AI 维护

本项目由 Claude 与 Codex 共同维护，**默认没有人类逐行审阅 diff**。
所有约束都做成了可被 CI 验证的检查，而不是靠自觉。

**动手前先读 [`AGENTS.md`](AGENTS.md)。** 它是唯一真源。

| 你是 | 入口 |
|---|---|
| Codex | [`AGENTS.md`](AGENTS.md) |
| Claude Code | [`CLAUDE.md`](CLAUDE.md) → 指向 `AGENTS.md` |
| 人 | 也是 [`AGENTS.md`](AGENTS.md) |

关键目录下都有成对的 `AGENTS.md` + `CLAUDE.md`，**就近优先**。

---

## 快速开始

```bash
# 全量检查（提交前必跑）
make check

# 默认开发形态：duetd + vite，浏览器打开 http://localhost:5173
# 不需要 Rust 工具链
make dev-web

# Tauri 壳联调（需要 Rust）
make dev-app

# 看所有可用命令
make help
```

---

## 六条铁律

1. **测试先行** —— 先写会失败的测试，跑一次确认它是红的，再写实现
2. **契约先行** —— 改接口的顺序永远是 `openapi.yaml → make gen → 实现`
3. **设计合规** —— UI 必须能在 `design/Duet Spec.dc.html` 找到条目；找不到先加条目
4. **不扩大边界** —— 只改允许改的范围，需要越界就停下来上报
5. **证据优先** —— 结论必须附 diff / 测试输出 / 命令记录，转述不算
6. **不碰用户真实数据** —— 测试禁止读写 `~/.acpflows`、真实仓库、真实令牌

完整版见 [`AGENTS.md`](AGENTS.md) §2。

---

## 文档地图

| 文档 | 管什么 |
|---|---|
| [`AGENTS.md`](AGENTS.md) | ★ 工作总纲：铁律、流程、目录地图、术语、禁止清单 |
| [`docs/architecture.md`](docs/architecture.md) | 进程模型、前后端分层、事件流、目录规划 |
| [`docs/domain-model.md`](docs/domain-model.md) | 聚合、状态机、不变量 |
| [`docs/design-principles.md`](docs/design-principles.md) | 抽象怎么切、接口放哪、怎么复用而不复制 |
| [`docs/database.md`](docs/database.md) | GORM 实体与领域模型的边界、表/列/索引命名、迁移、GORM 陷阱 |
| [`docs/coding-standards.md`](docs/coding-standards.md) | 命名、文件组织、`model`/`constant`/`util` 归属、工具库索引 |
| [`docs/testing-strategy.md`](docs/testing-strategy.md) | 测试先行五步、假测试图鉴、测试索引 |
| [`docs/acp-integration.md`](docs/acp-integration.md) | ACP 协议层规格、Fake Runtime 设计 |
| [`docs/runtime-config.md`](docs/runtime-config.md) | Runtime 配置的字段与取值（**实测为准**，非设计稿） |
| [`docs/acp-field-notes.md`](docs/acp-field-notes.md) | ★ 实测笔记：两端真实差异、隔离与注入方案、前一个项目踩过的 10 个坑 |
| [`docs/frontend-guide.md`](docs/frontend-guide.md) | 设计系统落地、组件规格、事件渲染器 |
| [`docs/i18n.md`](docs/i18n.md) | 中英双语规范 |
| [`docs/tech-debt.md`](docs/tech-debt.md) | 撞上烂代码：铲还是不铲、怎么铲、债务登记 |
| [`docs/git-workflow.md`](docs/git-workflow.md) | 分支、提交、PR、worktree |
| [`docs/ci.md`](docs/ci.md) | CI 设计：只跑受影响的部分、汇总门禁、时长预算 |
| [`docs/release-and-update.md`](docs/release-and-update.md) | 发版流水线、签名、客户端自动更新 |
| [`docs/ai-workflow.md`](docs/ai-workflow.md) | Claude × Codex 分工与交接 |
| [`docs/roadmap.md`](docs/roadmap.md) | 里程碑总览 |
| [`docs/milestones/`](docs/milestones/) | ★ 里程碑分章：子计划 → 单元 → 五段契约 → 可断言的验收标准 |
| [`docs/open-questions.md`](docs/open-questions.md) | ★ 待人拍板的问题（AI 不许替这些拍板） |
| [`docs/adr/`](docs/adr/) | 架构决策记录 |
| [`.skills/`](.skills/) | 项目级 skill（Claude 与 Codex 共用） |

---

## 技术栈

| 层 | 选择 |
|---|---|
| 外壳 | Tauri v2（Rust） |
| 后端 | Go · 单进程 `duetd` · HTTP + SSE · GORM + SQLite（纯 Go 驱动，无 CGO） |
| 前端 | React 18 + TypeScript + Vite |
| 契约 | OpenAPI 3.1（手写 spec 先行） |
| 测试 | Go test · Vitest + Testing Library · Playwright |
| 发布 | release-please + GitHub Actions + Tauri updater（minisign 签名） |

选型理由见 [`docs/adr/0001-tech-stack.md`](docs/adr/0001-tech-stack.md)。

---

## 许可

未定。
