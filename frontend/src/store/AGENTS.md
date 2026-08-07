# AGENTS.md · frontend/src/store

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

Zustand slices：纯 UI 状态（栏宽、折叠、抽屉栈、过滤器），持久化到 localStorage。

## 不负责什么

- **不放服务端状态** —— 列表、详情、变更走 TanStack Query
- 不放业务规则

## 本域特有的坑

- **服务端状态与 UI 状态分开管**，混在一起会导致缓存失效逻辑互相打架
- 实时事件流写 event slice，**不进 TanStack Query 缓存**
- 持久化的键要带版本号，结构变了能识别旧数据而不是崩掉
