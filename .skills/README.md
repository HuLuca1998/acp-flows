# 项目级 Skill

本目录是项目 skill 的**唯一真源**。两个 agent 目录用软链接指过来，全部进 git：

```
.skills/                 ← 真源，在这里增删改
.claude/skills  ─┐
.agents/skills  ─┴──▶ 软链接到 ../.skills
```

这样 Claude Code 与 Codex 用的是同一套 skill，不会各自维护一份然后漂移。

## 什么该写成 skill

**反复执行、步骤固定、做错会有代价的流程。**

写成 skill 而不是写进 `AGENTS.md` 的判断标准：

| 放 `AGENTS.md` | 写成 skill |
|---|---|
| 始终生效的约束（铁律、命名、禁止清单） | 按需触发的**流程** |
| 一句话能说清 | 有明确的步骤序列和产物 |
| 每次工作都要遵守 | 只在做某类任务时才用得上 |

## 当前 skill

| skill | 什么时候用 |
|---|---|
| [`tdd-unit`](tdd-unit/SKILL.md) | 开始任何一个开发单元时（**最常用**，测试先行五步流程） |
| [`go-unit-test`](go-unit-test/SKILL.md) | 写或改任何 Go 测试（Fake Runtime / 临时 SQLite / 假测试图鉴） |
| [`web-ui-test`](web-ui-test/SKILL.md) | 前端测试三层：组件行为 / E2E 自动化 / 真实浏览器模拟用户走查 |
| [`review-diff`](review-diff/SKILL.md) | 审查另一个 AI 的改动（实现方不得自审） |
| [`db-operate`](db-operate/SKILL.md) | 连数据库、查数据、调试「数据对不上」、改数据 |
| [`create-issue`](create-issue/SKILL.md) | 要开 GitHub issue 时 |

### 待建（按需补，别提前造）

| skill | 什么时候值得建 |
|---|---|
| `api-contract-change` | 开始频繁改 `api/openapi.yaml` 时 |
| `new-package` | 新建包的仪式（文档 + 索引 + depguard 规则）开始出错时 |
| `design-compliance` | 设计还原度反复被打回时 |
| `release` | 发过两三次版、流程稳定下来之后 |

## 写一个新 skill

```
.skills/<kebab-case-名字>/
├── SKILL.md          必需。frontmatter + 步骤
├── scripts/          可选。可执行脚本
└── references/       可选。按需载入的参考资料，不随 SKILL.md 常驻上下文
```

`SKILL.md` 的 frontmatter 必需两个字段：

```yaml
---
name: create-issue
description: 一句话说清「什么时候该用它」，写触发场景而不是功能描述——
             这行是 AI 判断要不要加载它的唯一依据。
---
```

**新增 skill 后要在上面的表格里登记一行**，否则没人知道它存在。

## 规则

- skill 只描述**怎么做**，不重复 `AGENTS.md` 里的约束——引用即可
- 步骤要可执行：写具体命令，不写「检查一下相关配置」
- 每个 skill 有明确产物（一个 PR / 一个 issue / 一份审查结论）
- 改了 skill 要说明为什么，skill 的漂移比代码的漂移更难发现
