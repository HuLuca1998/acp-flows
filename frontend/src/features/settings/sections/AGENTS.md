# AGENTS.md · frontend/src/features/settings/sections

设置页的各个分区。**一个分区一个组件**，父目录规则见
[`../AGENTS.md`](../AGENTS.md)，仓库总纲见根 [`AGENTS.md`](/AGENTS.md)。

---

## 加一个分区

1. 在这里建 `XxxSection.tsx`（+ `.module.css` + `.test.tsx`）
2. 在 `../section-registry.ts` 加一条记录
3. 在 `../index.tsx` 的 `renderSection` 里加一个 case

**不要改 `SettingsNav.tsx`**——它照注册表渲染。

## 分区组件的两条约束

**不自己拉数据。** 数据 hook 挂在 `../index.tsx`，结果按 props 传进来。
组件自己拉的话，每切一次分区就重新请求一次
（`../SettingsNav.test.tsx` 守着这条）。

**没做完的分区用 `Skeleton`，且骨架里不含任何数字。**
编造的数字看起来像真数据，比空白更糟。

## 动手前先看一眼设计稿

布局与视觉严格照 `design/ACP Duet 1a.dc.html` 的设置页，**不自由发挥**。
找不到对应条目时**先补设计条目再实现**（铁律 3）。

设计稿里的示意数据（版本号、项目数量）**不要照抄**——那是画稿时填的样例。
