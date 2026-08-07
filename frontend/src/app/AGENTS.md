# AGENTS.md · frontend/src/app

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/spec/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

路由、providers、布局骨架（窗口栏 42 / 左栏 252 / 主区 800 居中 / 右栏 300）。

## 不负责什么

- **不放业务组件**——那些在 `features/`
- **不放设计系统原语**——那些在 `ui/`

## 本域特有的坑

- **所有折叠开关集中在窗口栏**，页面内不再放第二处（设计规范 §06 规则①）
- **窗口栏左段宽度 = 左栏宽度**，随拖动同步
- **右栏只在对话页启用**，其他页面隐藏且窗口栏图标置灰
- 布局尺寸走 `--layout-*` 令牌，注入 inline style 是**唯一合法的 inline style 用途**
