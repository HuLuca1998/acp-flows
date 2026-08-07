# AGENTS.md · frontend/src/features/settings

> 本目录的规则。**就近优先**：与上级 [`../AGENTS.md`](../AGENTS.md) 冲突时以本文件为准。

## 负责什么

**设置页**。五个分区：环境检测 / 应用更新 / 项目管理 / GitHub 账号 / 通用。

对应验收点 **V0.2 V0.3 V2 V3**，施工图见
[`M1-install-and-update.md`](../../../../docs/plan/milestones/M1-install-and-update.md)。

## 不负责什么

- **不直接调后端。** 走 `src/api/`，不在组件里写 `fetch`。
- **不认识 Tauri。** 平台差异全在 `src/platform/`。
- **不跨 feature 直接 import。** 要复用就抽到 `src/ui/` 或 `src/models/`。

## 当前状态

已实现。继续改动前先跑一遍现有测试，别让回归悄悄溜过去。

## 检查命令

```bash
cd frontend && pnpm exec vitest run src/features/settings
cd frontend && pnpm exec tsc --noEmit
```

## 本域特有的坑

- **文案一律 `t('key')`**，中文字面量会被 ESLint 拦；英文字面量拦不住，靠 `make check-i18n` 与审查兜底。
- **状态词不翻译**：`executing` 这类在中英两版都保持英文原值、等宽显示。
- **颜色与尺寸一律 `var(--…)`**，写死 hex 或裸 px 会被 stylelint 拦。
- ★ **骨架占位绝不编造数据。** 一个显示「一次通过率 87%」但其实是编的界面，
  比空白更糟——用户会在假信息上做判断。

---

## 新文件该放哪

| 你要加的东西 | 放这里 |
|---|---|
| 一个新的设置分区（组件 + 它的 css / test） | `sections/` |
| 分区在左栏的位置、名字、副标题 | `section-registry.ts` 加一条 |
| 拉数据的 hook | 本目录，与 `use-runtimes.ts` 并列 |
| 页面骨架、二级导航本身 | 本目录 |

**加一个分区不该改 `SettingsNav.tsx` 一行**——它照着注册表渲染
（与 `app/pages.ts` 同一套做法）。

## 两条容易违反的

**数据 hook 挂在 `index.tsx`，不要挂进分区组件。** 挂进去的话每切一次分区
就 unmount / remount 一次、于是重新请求，而 Runtime 探测要拉起子进程——
用户会感到「点一下卡一下」。`SettingsNav.test.tsx` 守着这条。

**副标题里不许出现数字。** 设计稿写的是「1.4.2 → 1.5.0」「3 个项目」，
那是画稿时的示意数据。照抄的话，一个还没接项目功能的应用会告诉用户
他有 3 个项目——编造数据比空白更糟。
