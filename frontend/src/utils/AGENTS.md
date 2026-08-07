# AGENTS.md · frontend/src/utils

> **就近优先**。准入规则与后端 [`backend/internal/util`](../../../backend/internal/util/AGENTS.md) 对称。

## 负责什么

**纯函数工具**。按主题分文件：`format-duration.ts` `collection.ts` `path.ts` …

## 准入门槛：四条全满足

1. **纯函数** —— 无网络、无 DOM 副作用、无全局状态、无 `Date.now()` / `Math.random()`
2. **零业务语义** —— 不认识 Work / Unit / Contract
3. **已有 ≥ 2 个真实调用方，且跨 feature**
4. **有 Vitest 单测**

## 不负责什么

| ✗ 不该进来的 | 该去哪 |
|---|---|
| React hook | 通用的 → `src/ui/hooks/`；feature 专属 → 该 feature 目录 |
| 带业务语义（`formatUnitId`） | `src/models/` |
| 调 API | `src/api/` |
| 原生能力 | `src/platform/` |
| 常量、枚举 | `src/constants/` |
| 用户可见文案格式化 | 走 i18n，见 [`../../../docs/rules/i18n.md`](../../../docs/rules/i18n.md) |

**hook 不进这里** —— hook 有生命周期与副作用，不是纯函数。

## 禁止的文件名

`util.ts` `utils.ts` `helper.ts` `common.ts` `misc.ts` —— 由 `scripts/check-naming.sh` 拦下。

## 索引是强制的

**写新工具前先搜 [`INDEX.md`](INDEX.md)。** 新增/删除/改签名后同步索引：

```bash
make check-util-index
```

## 检查命令

```bash
pnpm -C ../../.. --filter frontend test --run src/utils
make -C ../../.. check-util-index
```

## 本域特有的坑

- **日期/数字/复数用 `Intl`，不要在这里手拼格式。** 它们与 locale 相关，属于 i18n。
  例外：等宽显示的时长与计数（`2:14`、`3/7`）是数据不是文案，可以在这里格式化。
- **别把 `lodash` 那一套整个抄进来。** 只抄真的用到的，且要有测试。
- **泛型别写过头。** 三个类型参数加一堆约束通常说明抽错了层。
