# docs/ 索引

**这是文档的入口。找不到东西时从这里开始，不要靠 `ls` 猜。**

`docs/` 按**文档回答的问题**分类，一共五类：

| 目录 | 回答什么问题 | 能不能改 |
|---|---|---|
| [`spec/`](spec/) | **系统长什么样** —— 架构、领域模型、协议、界面 | 随实现演进 |
| [`rules/`](rules/) | **必须怎么做** —— 编码、测试、数据库、Git、CI、日志 | 发现问题就补 |
| [`notes/`](notes/) | **真机上实际是什么** —— 实测记录、踩过的坑 | 只增不删 |
| [`adr/`](adr/) | **为什么这么定** —— 已关闭的决策 | ★ **不改，只新增** |
| [`plan/`](plan/) | **接下来做什么** —— 路线图、里程碑、待拍板问题 | 随进度更新 |

**`spec/` 与 `notes/` 冲突时，以 `notes/` 为准** —— 前者写「应该怎样」，后者写「真机上实际怎样」。

---

## 我想…… → 看哪份

### 刚接触这个项目

| 顺序 | 读 | 大概多久 |
|---|---|---|
| 1 | 根 [`AGENTS.md`](../AGENTS.md) | 5 分钟，**必读** |
| 2 | [`spec/architecture.md`](spec/architecture.md) §1–3 | 10 分钟，看清进程模型 |
| 3 | [`plan/roadmap.md`](plan/roadmap.md) | 3 分钟，看现在做到哪 |
| 4 | [`adr/0001-tech-stack.md`](adr/0001-tech-stack.md) | 3 分钟，知道为什么是 Go+Tauri |

**不要一上来通读 `spec/`** —— 那三份加起来 60k token。

### 我要动手写代码

→ 直接看 [`ai-playbook.md`](ai-playbook.md)（AI 用的路由表，人看也一样准）

### 我想知道某件事为什么这么定

→ [`adr/`](adr/) 看标题就能定位，7 份都很短

### 我想看某个功能什么时候做

→ [`plan/roadmap.md`](plan/roadmap.md) 看阶段，[`plan/milestones/`](plan/milestones/) 看单元级拆解

---

## 全部文档

### spec/ · 系统长什么样

| 文档 | 内容 | 体量 |
|---|---|---|
| [`architecture.md`](spec/architecture.md) | 进程模型、前后端分层、事件封闭枚举 | 16k |
| [`domain-model.md`](spec/domain-model.md) | 领域模型、状态机、**115 条不变量** | 82k ★ |
| [`acp-integration.md`](spec/acp-integration.md) | ACP 协议层规格、Fake Runtime | 99k ★ |
| [`frontend-guide.md`](spec/frontend-guide.md) | 设计系统落地、组件清单、渲染器注册表 | 62k ★ |
| [`release-and-update.md`](spec/release-and-update.md) | 发布与客户端自动更新 | 15k |
| [`runtime-config.md`](spec/runtime-config.md) | Runtime 配置字段与取值（**来自实测**） | 7k |

★ = 超过 8k token，**顶部有「读法」块，按它 grep 定位，不要整篇读**。

### rules/ · 必须怎么做

| 文档 | 什么时候看 |
|---|---|
| [`testing-strategy.md`](rules/testing-strategy.md) | 写任何测试之前（铁律 1） |
| [`coding-standards.md`](rules/coding-standards.md) | 命名、文件组织、工具库抽取 |
| [`design-principles.md`](rules/design-principles.md) | 设计抽象、分包、接口与实现 |
| [`database.md`](rules/database.md) | 建表、写 GORM、写迁移 |
| [`logging.md`](rules/logging.md) | 加日志、调级别 |
| [`debugging.md`](rules/debugging.md) | **排查时的查询与手法速查**（流程在 `debug` skill） |
| [`i18n.md`](rules/i18n.md) | 任何用户可见文本 |
| [`git-workflow.md`](rules/git-workflow.md) | 分支、提交、PR、worktree |
| [`ci.md`](rules/ci.md) | 改 workflow |
| [`tech-debt.md`](rules/tech-debt.md) | **撞上烂代码时** |
| [`doc-system.md`](rules/doc-system.md) | **写文档时 —— 新文档放哪、什么该写什么不该写** |
| [`forbidden.md`](rules/forbidden.md) | 禁止项完整清单 |
| [`ai-workflow.md`](rules/ai-workflow.md) | Claude × Codex 分工、交接契约 |

### notes/ · 真机上实际是什么

| 文档 | 内容 |
|---|---|
| [`pitfalls.md`](notes/pitfalls.md) | **操作踩坑档案** —— 按症状索引。卡在一个报错上时先 grep 这里 |
| [`acp-field-notes.md`](notes/acp-field-notes.md) | 两个 ACP Runtime 的实测行为、`make probe` 的结论 |

### adr/ · 为什么这么定（不重新讨论）

| ADR | 定了什么 |
|---|---|
| [`0001`](adr/0001-tech-stack.md) | Go + Tauri v2 + React；不用 Electron |
| [`0002`](adr/0002-release-and-auto-update.md) | 发布与自动更新的整体方案 |
| [`0003`](adr/0003-ai-maintained-repo.md) | 没有人类逐行审阅 → 规则必须可被命令验证 |
| [`0004`](adr/0004-design-source.md) | 设计稿管「形」，实测管「值」 |
| [`0005`](adr/0005-persistence.md) | 只用 SQLite，不上 MySQL |
| [`0006`](adr/0006-open-question-rulings.md) | 42 条待拍板问题的裁定 |
| [`0007`](adr/0007-release-revision-from-prior-art.md) | 按前作 `ai-workflows` 修订发布流程的 6 处 |

### plan/ · 接下来做什么

| 文档 | 内容 |
|---|---|
| [`roadmap.md`](plan/roadmap.md) | M0–M4 总览与当前进度 |
| [`open-questions.md`](plan/open-questions.md) | **仍需人拍板的问题** —— 卡住时先查这里 |
| [`milestones/README.md`](plan/milestones/README.md) | 里程碑体系与编号规则 |

里程碑分章（每章是子计划的菜单，**只读你要做的那一个 S**）：

| 章 | 做什么 | 状态 |
|---|---|---|
| [`M0`](plan/milestones/M0-acp-foundation.md) | ACP 地基：协议层、Fake Runtime、工程门禁 | **进行中** |
| [`M1`](plan/milestones/M1-release-and-update.md) | 发布与客户端自动更新 | 待开始 |
| [`M2`](plan/milestones/M2-golden-path.md) | 主链路垂直切片（端到端跑通一条真实工作流） | 待开始 |
| [`M3`](plan/milestones/M3-memory-and-skill.md) | 记忆与 Skill 三层 | 待开始 |
| [`M4`](plan/milestones/M4-product-surface.md) | 产品化界面与报表 | 待开始 |

### 顶层三份

| 文档 | 为什么在顶层 |
|---|---|
| [`ai-playbook.md`](ai-playbook.md) | 路由表，指向其他所有文档，不属于任何一类 |
| [`glossary.md`](glossary.md) | 术语速查，高频且极小 |
| [`templates/`](templates/) | `AGENTS.md` / `CLAUDE.md` 的骨架模板 |

---

## 新文档往哪放

**默认动作是「往已有文档加一节」，不是「新建文件」。**
新建之前先问：这件事能不能作为 `rules/` 或 `spec/` 里某份文档的一节？

放置规则、命名规则、以及「什么东西根本不该进 `docs/`」见
[`rules/doc-system.md`](rules/doc-system.md)。

`docs/` 根目录**不接受新文件**（`make check-docs` 会拦）——
新文档必须落进 `spec/` `rules/` `notes/` `adr/` `plan/` 之一，
并在本文的「全部文档」里登记一行。
