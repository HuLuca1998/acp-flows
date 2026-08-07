# AGENTS.md · frontend/src/platform

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/spec/frontend-guide.md)。

## 负责什么

平台适配层：Tauri 与 Web 双实现。文件选择、Finder 揭示、编辑器打开、窗口控制、自动更新。

## 不负责什么

- **不放业务逻辑**
- 不放通用工具（那是 `utils/`）

## 本域特有的坑

- ★ **这是全前端唯一可以 import `@tauri-apps/*` 的目录**，其他地方 ESLint 直接拦
- **Web 降级必须真的可用**，不能是空实现 —— `dev-web` 是 AI 自测的主要通道，
  降级坏了等于自测通道坏了
- 降级的每条能力都要有验收标准（见 frontend-guide.md §12）
