# AGENTS.md · frontend/src/features/report

> 本目录的规则。**就近优先**：与上级 [`../AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**报表页**。一次通过率、平均单元耗时、驳回原因分布、各 Runtime 用量。

对应验收点 **V14**，归 `M12`——2026-08-08 里程碑重排后它还没建施工图，
验收条件见 [`roadmap.md`](../../../../docs/plan/roadmap.md) 的「M5–M12 的验收条件」一节。

## 不负责什么

- **不直接调后端。** 走 `src/api/`，不在组件里写 `fetch`。
- **不认识 Tauri。** 平台差异全在 `src/platform/`。
- **不跨 feature 直接 import。** 要复用就抽到 `src/ui/` 或 `src/models/`。

## 当前状态

**骨架占位**，只有灰条 + 一句「这里将来是什么」。真正实现见上面的施工图。

## 检查命令

```bash
cd frontend && pnpm exec vitest run src/features/report
cd frontend && pnpm exec tsc --noEmit
```

## 本域特有的坑

- **文案一律 `t('key')`**，中文字面量会被 ESLint 拦；英文字面量拦不住，靠 `make check-i18n` 与审查兜底。
- **状态词不翻译**：`executing` 这类在中英两版都保持英文原值、等宽显示。
- **颜色与尺寸一律 `var(--…)`**，写死 hex 或裸 px 会被 stylelint 拦。
- ★ **骨架占位绝不编造数据。** 一个显示「一次通过率 87%」但其实是编的界面，
  比空白更糟——用户会在假信息上做判断。
