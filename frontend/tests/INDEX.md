# 测试索引 · frontend

> **写任何新测试前，先在本表里按「行为」搜一遍。**
> 规则见 [`../../docs/testing-strategy.md`](../../docs/rules/testing-strategy.md) §8。
> `make check-test-index` 会逐项比对，不一致即红。

## 登记规则

- 每个 **`*.test.ts` / `*.test.tsx` 文件**一行（不是每个 `it()`）
- 「覆盖的行为」写用户可观察的行为，不写实现细节
- 断言 CSS 类名或 DOM 结构的测试**不许存在**，见 testing-strategy.md §6

## 本目录还放什么

`frontend/tests/` 除索引外，放**跨 feature 共享的测试基建**：

```
frontend/tests/
├── INDEX.md
├── msw/          由 api/openapi.yaml 生成的 mock handlers
├── fixtures/     录制的事件序列、示例契约/证据数据
└── setup.ts      Vitest 全局 setup
```

单个组件的测试与组件同目录（`EventStream.test.tsx` 挨着 `EventStream.tsx`），
**不集中放这里**。

## 索引

| 测试文件 | 位置 | 覆盖的行为 |
|---|---|---|
| `Button.test.tsx` | `src/ui/` | 设计规范 §05/§08 的按钮**行为与可访问性契约**：可访问名称、点击回调、disabled 不触发、**纯图标按钮必须同时有 title 与 data-tt**、带快捷键时 tooltip 拼上快捷键、带文字的按钮不加 tooltip、type 默认 button 不误提交表单。<br>纯视觉规则（主按钮永不实心等）由 stylelint + 人工走查保证，不在这里测 |
| `schema.contract.test.ts` | `src/api/` | ★ 生成物与 `api/openapi.yaml` 的**编译期**契约：六个端点齐备、枚举没退化成 `string`、`required` 没被丢、`Runtime.name` 不是封闭枚举（注册表可扩展）。<br>断言靠 `@ts-expect-error` —— 类型一放宽，tsc 就报「未使用的 directive」而**编译不过**。`vitest` 跑绿不代表通过，真正的门是 `make lint-frontend` |

<!--
登记示例：

| `EventStream.test.tsx`   | `src/features/conversation/` | 13 类事件各自的渲染形态；过滤器开关；折叠态 |
| `use-event-stream.test.ts`| `src/features/conversation/`| SSE 断线重连、Last-Event-ID 续传、乱序事件按 seq 归位 |
| `UpdateCard.test.tsx`    | `src/features/settings/`     | 更新状态机：available → preparing → blocked 时展示被卡住的工作 |
-->
