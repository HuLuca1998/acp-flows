# AGENTS.md · frontend/src/features/plan

> 本目录的规则。**就近优先**：与根 [`AGENTS.md`](../../../../AGENTS.md) 冲突时以本文件为准。

## 负责什么

计划面板：**子计划 DAG 与它下面的单元**。回答用户三个问题——
「拆成了什么」「每条谁做」「走到哪了」。

设计稿形态：`subplan-01 · ACP Runtime 抽象层 / accepted · 3/3`，
下挂 `unit-013 依赖 unit-012 · 契约未冻结`。

## 不负责什么

| 不在这里 | 去哪 | 为什么 |
|---|---|---|
| 契约全文与版本切换 | `features/contract`（`M7`） | 契约是单元的产物，不是计划的一部分 |
| 决策卡片（D1–D3） | `features/decision`（`M9`） | 决策由执行过程触发，不由计划触发 |
| 需求快照与冻结 | `features/work/RequirementBar` | 需求在计划**之前**，两者版本链各自独立 |
| 时间线事件过滤器 | `features/timeline` | 设计稿把它画在计划面板那一侧，但它管的是时间线 |

## 依赖方向

| | |
|---|---|
| 允许 import | `@/api/system` · `@/models/plan` · `react-i18next` |
| 禁止 import | 其它 `features/*`（横向依赖）· `@/api/gen` 直接用（走 `@/models`） |

横向依赖没有 lint 规则强制——但一旦出现，计划面板会跟着别的面板一起崩，
而用户看到的是一整块空白而不是「计划这块出问题了」。

## 检查命令

```bash
pnpm vitest run src/features/plan/     # 本域测试
pnpm tsc --noEmit                      # 类型检查
```

## 改这里之前必读

- [`design/PARITY.md`](../../../../design/PARITY.md) 的对话页一节 —— 计划面板欠了哪几块、归谁
- [`M6 施工图`](../../../../docs/plan/milestones/M6-plan-and-units.md)
- 术语表（根 `AGENTS.md` §8）—— **状态词不翻译**

## 本域特有的坑

- ★★ **进度与状态不许前端自己算。** 后端给什么显示什么——两处各算一遍
  必然漂移，而漂移的那一刻用户看到的是「3/3 但还在跑」。
- ★★ **认不出的角色显示原始 id，不编名字。** 编出来的名字与角色页那张表
  对不上，用户会以为有两个不同的角色。后端已经在 `role_display_name`
  里给了显示名，**认不出时它是空的**——那是有意的，别在前端补一个兜底名。
- ★ 状态词（`accepted` / `in_progress` / `pending`）**显示英文原值**。
  用户在文档、日志、界面上看到的必须是同一个词。
- ★ 还没规划时**整块不显示**，不弹「读取计划失败」——那是新工作的常态。
