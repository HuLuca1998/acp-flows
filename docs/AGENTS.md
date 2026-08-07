# AGENTS.md · docs

> **就近优先**：与根 [`AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

架构与规范文档。**它们是被当作规格来执行的，不是参考资料。**

| 文档 | 管什么 |
|---|---|
| [`architecture.md`](spec/architecture.md) | 进程模型、分层、依赖方向、事件流、目录规划 |
| [`domain-model.md`](spec/domain-model.md) | 聚合、状态机、不变量 |
| [`design-principles.md`](rules/design-principles.md) | 抽象怎么切、接口放哪、包怎么分、怎么复用而不复制 |
| [`coding-standards.md`](rules/coding-standards.md) | 命名、文件组织、model/constant/util 归属、工具库索引 |
| [`testing-strategy.md`](rules/testing-strategy.md) | 测试先行五步、假测试图鉴、测试索引 |
| [`acp-integration.md`](spec/acp-integration.md) | ACP 协议层规格、Fake Runtime 设计 |
| [`frontend-guide.md`](spec/frontend-guide.md) | 设计系统落地、组件规格、事件渲染器 |
| [`git-workflow.md`](rules/git-workflow.md) | 分支、提交、PR、worktree、发版触发 |
| [`release-and-update.md`](spec/release-and-update.md) | CI/CD、签名、客户端自动更新 |
| [`ai-workflow.md`](rules/ai-workflow.md) | Claude × Codex 分工与交接 |
| [`roadmap.md`](plan/roadmap.md) | 里程碑 |
| [`adr/`](adr/) | 架构决策记录 |
| [`templates/`](templates/) | `AGENTS.md` / `CLAUDE.md` 骨架模板 |

## 不负责什么

- **不放设计稿。** 设计真源在 [`../design/`](../design/)，本目录只做转译与落地规格。
- **不放 API 契约。** 契约真源是 [`../api/openapi.yaml`](../api/openapi.yaml)。
- **不放教程。** 这些文档写给会读代码的 AI 和人，不解释基础概念。

## 规则

### 文档滞后按缺陷处理

改动让某份文档过期了 → **当场修正，和代码改动放同一个提交**。
不要开 issue 攒着——攒着的结果永远是不修。

### 一件事只在一处写

同一条规则不许在两份文档里各写一遍——重复的内容必然漂移。
需要交叉引用时**用链接**，不要复制。

发现两处描述同一件事且不一致 → 立刻合并到其中一处，另一处改成链接。

### 每条规则都要有强制手段

写在文档里却没有检查手段的规则等于无效规则。
新增规则时同步问：**这条怎么被 CI 拦？**

拦不了的规则要么改成能拦的形式，要么明确标注「靠自觉，可能失效」。

### ADR 只增不改

架构决策记录一旦写下就不修改正文。决定变了 → 写新的 ADR，
在旧的开头加一行 `> 已被 adr/00XX 取代`。

## 检查命令

```bash
make check-docs        # 关键目录是否都有填实的 AGENTS.md + CLAUDE.md
```

## 本域特有的坑

- **别写「应该」「尽量」「建议」。** 这类词在无人类审阅的仓库里等于没写。
  要么是硬规则（配检查），要么删掉。
- **数值要具体。** 「适当的间距」→ `gap 7–9`；「文件不要太长」→ `400 行`。
- **术语严格按根 `AGENTS.md` §8。** 状态词英文不翻译。
