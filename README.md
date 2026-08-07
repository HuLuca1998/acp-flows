# Duet

**ACP 多智能体协作编程 App。**

把 `claude-agent-acp` 与 `codex-acp` 两个 ACP Runtime 当作可编排的执行体，
用「计划 → 子计划 → 单元契约 → 尝试 → 证据 → 检查点」的状态机驱动它们协作写代码，
人只在 D2/D3 决策点介入。

macOS 桌面应用（Tauri 壳 + Go sidecar），同一份代码可作为纯 Web 运行。

> **状态：M0 进行中**，脚手架四件套（backend / frontend / shell / e2e）已跑通，
> `make check` 全绿。剩余关键路径是 Fake ACP Runtime。
> 进度见 [`docs/plan/roadmap.md`](docs/plan/roadmap.md)，分章见 [`docs/plan/milestones/`](docs/plan/milestones/)。

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

**文档入口是 [`docs/README.md`](docs/README.md)** —— 那里有分类表、
「我想…… → 看哪份」和全部文档索引。这里只列三个最高频的：

| 从哪开始 | |
|---|---|
| [`AGENTS.md`](AGENTS.md) | ★ 工作总纲：铁律、流程、目录地图、术语、禁止清单。**AI 每次对话都带着它** |
| [`docs/README.md`](docs/README.md) | ★ 文档入口：`spec/` `rules/` `notes/` `adr/` `plan/` 五类的索引 |
| [`docs/ai-playbook.md`](docs/ai-playbook.md) | ★ 路由表：你现在这个情形，该走哪条路、该读哪一节 |

`docs/` 按**文档回答什么问题**分类：

| | |
|---|---|
| [`spec/`](docs/spec/) | 系统长什么样 —— 架构、领域模型、协议、界面 |
| [`rules/`](docs/rules/) | 必须怎么做 —— 编码、测试、数据库、Git、CI、日志 |
| [`notes/`](docs/notes/) | 真机上实际是什么 —— 实测记录、[踩坑档案](docs/notes/pitfalls.md)（**与 `spec/` 冲突时以它为准**） |
| [`adr/`](docs/adr/) | 为什么这么定 —— 已关闭的决策，不重新讨论 |
| [`plan/`](docs/plan/) | 接下来做什么 —— [路线图](docs/plan/roadmap.md)、[里程碑分章](docs/plan/milestones/)、[待拍板问题](docs/plan/open-questions.md) |
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
