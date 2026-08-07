# 工具库索引 · frontend/src/utils

> **写任何新工具函数前，先在本表里搜一遍。** 规则同后端，见
> [`../../../docs/coding-standards.md`](../../../docs/rules/coding-standards.md) §1.3–§1.4。
>
> 新增 / 删除 / 改签名后必须同步本表。`make check-util-index` 会逐项比对，不一致即红。

## 准入门槛（四条全满足才能进来）

1. 纯函数：无网络、无 DOM 副作用、无全局状态、无 `Date.now()` / `Math.random()`
2. 零业务语义：不认识 Work / Unit / Contract 这些概念
3. 已有 **≥ 2 个**真实调用方，且跨 feature
4. 有 Vitest 单测

> React hook **不进这里**——hook 有生命周期与副作用，不是纯函数。
> 通用 hook 放 `src/ui/hooks/`，feature 专属 hook 放该 feature 目录内。

## 索引

| 函数 | 文件 | 签名 | 一句话说明 |
|---|---|---|---|
| _（尚无条目）_ | | | |

<!--
登记示例，新增时照抄这个格式：

| `formatDuration` | `format-duration.ts` | `(ms: number) => string` | 毫秒转 `2:14` 形式的时长 |
| `groupBy`        | `collection.ts`      | `<T,K>(xs: T[], key: (x:T)=>K) => Map<K,T[]>` | 按键分组 |
-->
