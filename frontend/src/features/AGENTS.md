# AGENTS.md · frontend/src/features

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

按页面分的业务组件：conversation · plan · memory · skill · report · settings · roles · project。

## 不负责什么

- **不写业务逻辑**——状态机、编排在后端
- **不直接 fetch**——走 `api/` 的生成客户端 + TanStack Query

## 本域特有的坑

- **`features/a` 不许 import `features/b`** —— 跨 feature 要走 `ui/` 或 `models/`
- **13 类事件用注册表分发，禁止 `switch (event.type)`**（ESLint 会拦）
- 新增第 14 类事件要同时改四处：openapi.yaml + architecture.md §4 + Duet Spec + 渲染器注册表
- 单个组件的测试与组件**同目录**，不集中放 `tests/`
