# AGENTS.md · frontend/src/api

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/spec/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

由 `api/openapi.yaml` 生成的 HTTP client、类型，以及 SSE 订阅封装。

## 不负责什么

- **`gen/` 下的文件不手改**——改接口要先改 `api/openapi.yaml` 再 `make gen`（铁律 2）
- 不放业务逻辑

## 本域特有的坑

- **SSE 事件不要塞进 TanStack Query 缓存** —— 两套失效模型混在一起必炸。
  事件流走 Zustand 的 event slice
- `Last-Event-ID` 续传是产品硬需求，不是优化
- 错误 `Problem.type` 是**机器可读错误码**，前端据此查 i18n 词条；
  `detail` 只给开发者看，**界面不展示**
