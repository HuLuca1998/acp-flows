# AGENTS.md · frontend/src/design

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/frontend-guide.md)。

## 负责什么

设计令牌：`tokens.css`（Nocturne，抽取物）+ `duet.css`（产品层）。

## 不负责什么

- **`tokens.css` 不手改** —— 它由 `design/_ds/nocturne/styles.css` 抽取。
  要改令牌值回 Claude Design 项目改，再同步

## 本域特有的坑

- **组件里一律 `var()`，禁止写死 hex 与裸 px** —— stylelint 与 ESLint 都会拦
- 语义色**只有 `--color-pass` 与 `--color-fail` 两个**，不得引入第三种
- 强调色只做线与光，**不做大面积填充**
- 新增令牌要同时在 `docs/frontend-guide.md` §1 登记，否则没人知道它存在
