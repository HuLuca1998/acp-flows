# AGENTS.md · frontend/src/constants

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

常量与枚举取值，按主题分文件：`state.ts` `event.ts` `layout.ts` `decision.ts`。

## 不负责什么

- **不放会变的配置** —— 那走后端
- 不放只在一个文件用到的常量（就地定义）

## 本域特有的坑

- **一律 `as const` 对象 + 联合类型，禁用 `enum`**（ESLint 会拦）：
  enum 生成运行时代码，且与 openapi 的字符串联合类型对不上
- 状态词等取值必须与后端 `internal/constant/` **一字不差**，改一处要同步四处
- 每个枚举配 `ALL_XXX` 数组，供穷举与筛选器生成
