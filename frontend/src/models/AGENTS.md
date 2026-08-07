# AGENTS.md · frontend/src/models

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

领域类型。多数是 openapi 生成物的再包装，加上前端需要的派生类型。

## 不负责什么

- **不放常量与枚举取值** —— 那在 `constants/`
- 不放组件 Props 类型（跟着组件走）

## 本域特有的坑

- 类型名 PascalCase，**不加 `I` 前缀**
- 与后端的字段名保持一致（`snake_case`），不要在这一层改名 —— 改名要在生成器里做，否则两边对不上
