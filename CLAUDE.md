# CLAUDE.md

**本仓库的工作总纲在 [`AGENTS.md`](AGENTS.md)。先完整读完它，再动手。**

本文件只补充 Claude Code 特有的内容；所有产品规则、铁律、目录地图、术语表都以 `AGENTS.md` 为准。
两份文件冲突时，**以 `AGENTS.md` 为准**——它同时被 Codex 读取，是唯一真源。

---

## 就近优先

关键目录下都有成对的 `AGENTS.md` + `CLAUDE.md`。在某个目录下工作时，**该目录的规则覆盖根目录的同名条款**：

| 你在改 | 先读 |
|---|---|
| `backend/**` | [`backend/AGENTS.md`](backend/AGENTS.md) |
| `backend/internal/domain/**` | [`backend/internal/domain/AGENTS.md`](backend/internal/domain/AGENTS.md) |
| `backend/internal/acp/**` | [`backend/internal/acp/AGENTS.md`](backend/internal/acp/AGENTS.md) |
| `frontend/**` | [`frontend/AGENTS.md`](frontend/AGENTS.md) |
| `shell/**` | [`shell/AGENTS.md`](shell/AGENTS.md) |
| `api/**` | [`api/AGENTS.md`](api/AGENTS.md) |
| `e2e/**` | [`e2e/AGENTS.md`](e2e/AGENTS.md) |
| `design/**` | [`design/AGENTS.md`](design/AGENTS.md) |

---

## Claude Code 专属

### 必用的 skill

| 场景 | skill | 为什么 |
|---|---|---|
| 写任何 Go 测试 | `go-unit-testing` | 强制契约优先、真实实例真实数据，挡住假测试。**写 `*_test.go` 前必须调用。** |
| 需要 Codex 做独立审查 / 干活 | `codex-collab` | 本项目的双 AI 协作靠它落地，见 [`docs/ai-workflow.md`](docs/rules/ai-workflow.md) |
| 提交 / 开 PR / 合并 | `gh-commit` `gh-pr` `gh-pass` | 分支与提交规范见 [`docs/git-workflow.md`](docs/rules/git-workflow.md) |
| 需求不明确、方案有取舍 | `brainstorming` | 别猜着实现，先把问题想清楚 |

### 子代理

宽泛检索用 `Explore`，方案设计用 `Plan`。**不要**用 `hz-*` 系列代理——那套是 HAB Admin 项目的流程，与本仓库的契约/测试先行流程不兼容。

### 与 Codex 的分工

本仓库由 Claude 与 Codex 共同维护。默认分工、交接格式、冲突裁决规则写在
[`docs/ai-workflow.md`](docs/rules/ai-workflow.md)。**开始跨端协作前必读。**

一句话版本：**Claude 定契约与做审查，Codex 在冻结契约内实现。** 审查不得由实现方自己做。

### 输出语言

面向用户的回复、文档、代码注释一律**简体中文**；标识符、状态词、命令、路径保持英文原值。
状态词（`executing` `waiting_user` `reviewing_unit` 等）**不翻译**，见 `AGENTS.md` 术语表。
