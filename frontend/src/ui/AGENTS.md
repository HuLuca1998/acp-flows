# AGENTS.md · frontend/src/ui

> **就近优先**。前端总规则见 [`../../AGENTS.md`](../../AGENTS.md)，
> 实现规格见 [`../../../docs/frontend-guide.md`](../../../docs/frontend-guide.md)。

## 负责什么

设计系统原语：Button · Tag · Card · Drawer · Dialog · Dropdown · Progress · Switch · Icon。
每个组件必须能在 `design/Duet Spec.dc.html` §05 找到对应条目。

## 不负责什么

- **不认识业务**——`ui/` 里出现 Work / Unit / Contract 就是分层错了
- **不 import `features/`**（原语不许认识业务）

## 本域特有的坑

- **新组件必须先在设计规范找到条目**；找不到 → 先在设计规范新增条目，再实现（铁律 3）
- **主按钮永不填充实心**；危险动作用 `--color-fail` 文字色而不是红底
- **纯图标按钮必须有中文 tooltip**（`title` + `data-tt`），带文字的不加
- 图标只从 Phosphor regular 取，四档尺寸 16/15/13/12。禁 emoji、自绘 SVG、Unicode 几何符号
- 测试断言**行为与可访问性契约**，不断言 CSS 类名与 DOM 结构
