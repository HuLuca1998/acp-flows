# AGENTS.md · frontend/src/features/rail

> 本目录的规则。**就近优先**：与上级 [`../AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**左栏**。5 项导航 + 项目树 + 最近工作 + Runtime 状态。**左栏回答「你在哪个项目的哪个工作里」**，不是「你要去哪个功能页」。

对应验收点 **V0 V4**，施工图见
[`M3-project-onboarding.md`](../../../../docs/plan/milestones/M3-project-onboarding.md)。

## 不负责什么

- **不直接调后端。** 走 `src/api/`，不在组件里写 `fetch`。
- **不认识 Tauri。** 平台差异全在 `src/platform/`。
- **不跨 feature 直接 import。** 要复用就抽到 `src/ui/` 或 `src/models/`。

## 当前状态

已实现。继续改动前先跑一遍现有测试，别让回归悄悄溜过去。

## 检查命令

```bash
cd frontend && pnpm exec vitest run src/features/rail
cd frontend && pnpm exec tsc --noEmit
```

## 本域特有的坑

- **文案一律 `t('key')`**，中文字面量会被 ESLint 拦；英文字面量拦不住，靠 `make check-i18n` 与审查兜底。
- **状态词不翻译**：`executing` 这类在中英两版都保持英文原值、等宽显示。
- **颜色与尺寸一律 `var(--…)`**，写死 hex 或裸 px 会被 stylelint 拦。
- ★ **骨架占位绝不编造数据。** 一个显示「一次通过率 87%」但其实是编的界面，
  比空白更糟——用户会在假信息上做判断。
